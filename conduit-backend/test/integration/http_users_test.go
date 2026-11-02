//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"conduit/backend/test/support"
)

func userEnvelope(fields map[string]any) support.RequestOptions {
	return support.RequestOptions{JSON: map[string]any{"user": fields}}
}

func TestRegisterUser(t *testing.T) {
	app := newApp(t)
	client := app.NewClient()

	t.Run("creates the user and returns a usable token", func(t *testing.T) {
		creds := support.RandomCredentials()
		creds.Email = strings.ToUpper(creds.Email)
		response := client.Post("/api/v1/users", userEnvelope(map[string]any{
			"username": " " + creds.Username + " ", "email": creds.Email, "password": creds.Password,
		}))
		expectStatus(t, response, 201)
		expectEqual(t, response.String("user", "username"), creds.Username)
		expectEqual(t, response.String("user", "email"), strings.ToLower(creds.Email))
		if response.Value("user", "bio") != nil || response.Value("user", "image") != nil {
			t.Fatalf("bio and image must be null: %s", response.Raw)
		}
		if _, present := response.Body["user"].(map[string]any)["bio"]; !present {
			t.Fatal("bio must be present as null")
		}
		if strings.Contains(string(response.Raw), "password") {
			t.Fatalf("the password must never be returned: %s", response.Raw)
		}

		current := client.Get("/api/v1/user", support.RequestOptions{Token: response.String("user", "token")})
		expectStatus(t, current, 200)
		expectEqual(t, current.String("user", "username"), creds.Username)
	})

	t.Run("reports every invalid field", func(t *testing.T) {
		response := client.Post("/api/v1/users", userEnvelope(map[string]any{"username": "x", "email": "bad", "password": "short"}))
		expectStatus(t, response, 422)
		expectDeep(t, response.Errors("username"), []string{"is too short (minimum is 3 characters)"})
		expectDeep(t, response.Errors("email"), []string{"is invalid"})
		expectDeep(t, response.Errors("password"), []string{"is too short (minimum is 8 characters)"})
	})

	t.Run("treats a missing envelope as empty input", func(t *testing.T) {
		for _, body := range []string{`{}`, `{"user":null}`, `{"user":"x"}`, `{"username":"a"}`} {
			response := client.Post("/api/v1/users", support.RequestOptions{RawBody: body, ContentType: "application/json"})
			expectStatus(t, response, 422)
			expectDeep(t, response.Errors("username"), []string{"can't be blank"})
			expectDeep(t, response.Errors("email"), []string{"can't be blank"})
			expectDeep(t, response.Errors("password"), []string{"can't be blank"})
		}
	})

	t.Run("rejects duplicates case insensitively", func(t *testing.T) {
		creds, _ := support.MustRegisterUser(t, client)
		response := client.Post("/api/v1/users", userEnvelope(map[string]any{
			"username": strings.ToUpper(creds.Username), "email": strings.ToUpper(creds.Email), "password": "Password123!",
		}))
		expectStatus(t, response, 409)
		expectDeep(t, response.Errors("username"), []string{"has already been taken"})
		expectDeep(t, response.Errors("email"), []string{"has already been taken"})

		other := support.RandomCredentials()
		response = client.Post("/api/v1/users", userEnvelope(map[string]any{
			"username": other.Username, "email": creds.Email, "password": other.Password,
		}))
		expectStatus(t, response, 409)
		expectDeep(t, response.Errors("email"), []string{"has already been taken"})
		expectEqual(t, len(response.Errors("username")), 0)
	})
}

func TestLogin(t *testing.T) {
	app := newApp(t)
	client := app.NewClient()
	creds, _ := support.MustRegisterUser(t, client)

	t.Run("accepts the credentials", func(t *testing.T) {
		response := client.Post("/api/v1/users/login", userEnvelope(map[string]any{"email": " " + strings.ToUpper(creds.Email) + " ", "password": creds.Password}))
		expectStatus(t, response, 200)
		expectEqual(t, response.String("user", "username"), creds.Username)
		if response.String("user", "token") == "" {
			t.Fatal("a token is expected")
		}
	})

	t.Run("hides whether the email or the password was wrong", func(t *testing.T) {
		wrong := client.Post("/api/v1/users/login", userEnvelope(map[string]any{"email": creds.Email, "password": "WrongPassword1"}))
		unknown := client.Post("/api/v1/users/login", userEnvelope(map[string]any{"email": "nobody@mail.com", "password": creds.Password}))
		expectProblem(t, wrong, 401, "credentials", "are invalid")
		expectProblem(t, unknown, 401, "credentials", "are invalid")
		expectEqual(t, string(wrong.Raw), string(unknown.Raw))
	})

	t.Run("validates the input", func(t *testing.T) {
		response := client.Post("/api/v1/users/login", userEnvelope(map[string]any{"email": "  ", "password": 5}))
		expectStatus(t, response, 422)
		expectDeep(t, response.Errors("email"), []string{"can't be blank"})
		expectDeep(t, response.Errors("password"), []string{"can't be blank"})
	})
}

func TestCurrentUserAuthentication(t *testing.T) {
	app := newApp(t)
	client := app.NewClient()
	_, token := support.MustRegisterUser(t, client)

	cases := map[string]struct {
		header  string
		message string
	}{
		"no header":      {"", "is missing"},
		"bearer scheme":  {"Bearer " + token, "is invalid"},
		"lowercase":      {"token " + token, "is invalid"},
		"bare token":     {token, "is invalid"},
		"extra segments": {"Token " + token + " extra", "is invalid"},
		"garbage":        {"Token not.a.token", "is invalid"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			options := support.RequestOptions{}
			if tc.header != "" {
				options.Headers = map[string]string{"Authorization": tc.header}
			}
			expectProblem(t, client.Get("/api/v1/user", options), 401, "token", tc.message)
		})
	}

	t.Run("rejects tokens signed with another secret", func(t *testing.T) {
		other := newApp(t, map[string]string{"AUTH_SECRET": "another-secret-value-with-more-than-32-chars"})
		_, foreign := support.MustRegisterUser(t, other.NewClient())
		expectProblem(t, client.Get("/api/v1/user", support.RequestOptions{Token: foreign}), 401, "token", "is invalid")
	})
}

func TestExpiredTokens(t *testing.T) {
	app := newApp(t, map[string]string{"AUTH_TOKEN_TTL": "1"})
	client := app.NewClient()
	_, token := support.MustRegisterUser(t, client)

	expectStatus(t, client.Get("/api/v1/user", support.RequestOptions{Token: token}), 200)
	time.Sleep(2200 * time.Millisecond)
	expectProblem(t, client.Get("/api/v1/user", support.RequestOptions{Token: token}), 401, "token", "has expired")
}

func TestUpdateCurrentUser(t *testing.T) {
	app := newApp(t)
	user := signIn(t, app)
	other := signIn(t, app)
	put := func(account account, fields map[string]any) support.Response {
		return account.client.Put("/api/v1/user", account.with(userEnvelope(fields)))
	}

	t.Run("requires authentication", func(t *testing.T) {
		expectProblem(t, user.client.Put("/api/v1/user", userEnvelope(map[string]any{"bio": "x"})), 401, "token", "is missing")
	})

	t.Run("rejects an empty update", func(t *testing.T) {
		expectProblem(t, put(user, map[string]any{}), 422, "user", "can't be empty")
		expectProblem(t, user.client.Put("/api/v1/user", user.with(support.RequestOptions{RawBody: `{}`, ContentType: "application/json"})), 422, "user", "can't be empty")
	})

	t.Run("updates profile fields and returns a fresh token", func(t *testing.T) {
		response := put(user, map[string]any{"bio": " I like pizza ", "image": "https://example.com/me.png"})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("user", "bio"), "I like pizza")
		expectEqual(t, response.String("user", "image"), "https://example.com/me.png")
		expectEqual(t, response.String("user", "username"), user.creds.Username)
		if response.String("user", "token") == "" {
			t.Fatal("a token is expected")
		}

		current := user.client.Get("/api/v1/user", user.auth())
		expectEqual(t, current.String("user", "bio"), "I like pizza")
	})

	t.Run("clears optional fields with null or blank", func(t *testing.T) {
		response := put(user, map[string]any{"bio": nil, "image": "  "})
		expectStatus(t, response, 200)
		if response.Value("user", "bio") != nil || response.Value("user", "image") != nil {
			t.Fatalf("expected nulls: %s", response.Raw)
		}
	})

	t.Run("changes the email and username", func(t *testing.T) {
		creds := support.RandomCredentials()
		response := put(user, map[string]any{"username": creds.Username, "email": creds.Email})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("user", "username"), creds.Username)
		expectEqual(t, response.String("user", "email"), creds.Email)
		user.creds = creds

		login := user.client.Post("/api/v1/users/login", userEnvelope(map[string]any{"email": creds.Email, "password": creds.Password}))
		expectStatus(t, login, 200)
	})

	t.Run("changes the password", func(t *testing.T) {
		expectStatus(t, put(user, map[string]any{"password": "BrandNewPass1"}), 200)
		old := user.client.Post("/api/v1/users/login", userEnvelope(map[string]any{"email": user.creds.Email, "password": user.creds.Password}))
		expectStatus(t, old, 401)
		fresh := user.client.Post("/api/v1/users/login", userEnvelope(map[string]any{"email": user.creds.Email, "password": "BrandNewPass1"}))
		expectStatus(t, fresh, 200)
	})

	t.Run("validates every provided field", func(t *testing.T) {
		response := put(user, map[string]any{"username": "x", "email": "bad", "password": "short", "image": "http://insecure.com/a.png", "bio": 5})
		expectStatus(t, response, 422)
		expectDeep(t, response.Errors("username"), []string{"is too short (minimum is 3 characters)"})
		expectDeep(t, response.Errors("email"), []string{"is invalid"})
		expectDeep(t, response.Errors("password"), []string{"is too short (minimum is 8 characters)"})
		expectDeep(t, response.Errors("image"), []string{"must be a valid https URL"})
		expectDeep(t, response.Errors("bio"), []string{"is invalid"})
	})

	t.Run("rejects taken values but allows re-using your own", func(t *testing.T) {
		response := put(user, map[string]any{"username": other.creds.Username, "email": other.creds.Email})
		expectStatus(t, response, 409)
		expectDeep(t, response.Errors("username"), []string{"has already been taken"})
		expectDeep(t, response.Errors("email"), []string{"has already been taken"})

		expectStatus(t, put(user, map[string]any{"username": strings.ToUpper(user.creds.Username), "email": user.creds.Email}), 200)
	})
}
