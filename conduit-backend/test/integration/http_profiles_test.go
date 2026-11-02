//go:build integration

package integration

import "testing"

func TestProfiles(t *testing.T) {
	app := newApp(t)
	jane := signIn(t, app)
	john := signIn(t, app)
	profilePath := func(a account) string { return "/api/v1/profiles/" + a.creds.Username }

	t.Run("anyone may read a profile", func(t *testing.T) {
		response := john.client.Get(profilePath(jane))
		expectStatus(t, response, 200)
		expectEqual(t, response.String("profile", "username"), jane.creds.Username)
		expectEqual(t, response.Bool("profile", "following"), false)
		if response.Value("profile", "bio") != nil || response.Value("profile", "image") != nil {
			t.Fatalf("bio and image default to null: %s", response.Raw)
		}
	})

	t.Run("usernames are matched case insensitively", func(t *testing.T) {
		expectStatus(t, john.client.Get("/api/v1/profiles/"+upper(jane.creds.Username)), 200)
	})

	t.Run("unknown profiles are not found", func(t *testing.T) {
		expectProblem(t, john.client.Get("/api/v1/profiles/nobody-here"), 404, "profile", "not found")
	})

	t.Run("following requires authentication", func(t *testing.T) {
		expectProblem(t, john.client.Post(profilePath(jane)+"/follow"), 401, "token", "is missing")
		expectProblem(t, john.client.Do("DELETE", profilePath(jane)+"/follow", noAuth()), 401, "token", "is missing")
	})

	t.Run("follows and unfollows idempotently", func(t *testing.T) {
		for range 2 {
			response := john.client.Post(profilePath(jane)+"/follow", john.auth())
			expectStatus(t, response, 200)
			expectEqual(t, response.Bool("profile", "following"), true)
		}
		seen := john.client.Get(profilePath(jane), john.auth())
		expectEqual(t, seen.Bool("profile", "following"), true)
		stranger := jane.client.Get(profilePath(jane), jane.auth())
		expectEqual(t, stranger.Bool("profile", "following"), false)
		anonymous := john.client.Get(profilePath(jane))
		expectEqual(t, anonymous.Bool("profile", "following"), false)

		for range 2 {
			response := john.client.Delete(profilePath(jane)+"/follow", john.auth())
			expectStatus(t, response, 200)
			expectEqual(t, response.Bool("profile", "following"), false)
		}
	})

	t.Run("refuses to follow yourself or unknown users", func(t *testing.T) {
		expectProblem(t, jane.client.Post(profilePath(jane)+"/follow", jane.auth()), 422, "username", "you cannot follow yourself")
		expectProblem(t, jane.client.Post("/api/v1/profiles/nobody-here/follow", jane.auth()), 404, "profile", "not found")
		expectProblem(t, jane.client.Delete("/api/v1/profiles/nobody-here/follow", jane.auth()), 404, "profile", "not found")
	})

	t.Run("an invalid token fails even on public reads", func(t *testing.T) {
		expectProblem(t, john.client.Get(profilePath(jane), tokenOf("garbage")), 401, "token", "is invalid")
	})
}
