//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	"conduit/backend/test/support"
)

func TestHealthEndpoint(t *testing.T) {
	app := newApp(t)
	response := app.NewClient().Get("/health")
	expectStatus(t, response, 200)
	expectEqual(t, string(response.Raw), "OK")
}

func TestUnknownRoutesUseTheErrorEnvelope(t *testing.T) {
	app := newApp(t)
	client := app.NewClient()
	for _, path := range []string{"/", "/api/v1/nothing", "/api/v1/articles/a/b/c"} {
		expectProblem(t, client.Get(path), 404, "path", "not found")
	}
	expectProblem(t, client.Do(http.MethodPatch, "/api/v1/user", support.RequestOptions{}), 404, "path", "not found")
}

func TestRemovedLegacyRoutesStayGone(t *testing.T) {
	app := newApp(t)
	client := app.NewClient()
	for _, path := range []string{"/api/v1/auth/signup", "/api/v1/tasks", "/api/v1/users/me"} {
		expectStatus(t, client.Get(path), 404)
	}
}

func TestRequestBodyHandling(t *testing.T) {
	app := newApp(t)
	client := app.NewClient()

	t.Run("rejects non JSON content types", func(t *testing.T) {
		response := client.Post("/api/v1/users", support.RequestOptions{RawBody: `{"user":{}}`, ContentType: "text/plain"})
		expectProblem(t, response, 415, "content-type", "must be application/json")
		response = client.Post("/api/v1/users", support.RequestOptions{RawBody: `{"user":{}}`})
		expectProblem(t, response, 415, "content-type", "must be application/json")
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		for _, body := range []string{`{"user":`, `"text"`, `42`, `null`} {
			response := client.Post("/api/v1/users", support.RequestOptions{RawBody: body, ContentType: "application/json"})
			expectProblem(t, response, 400, "body", "is not valid JSON")
		}
	})

	t.Run("rejects oversized bodies", func(t *testing.T) {
		body := `{"user":{"bio":"` + strings.Repeat("x", 300*1024) + `"}}`
		expectProblem(t, client.Post("/api/v1/users", support.RequestOptions{RawBody: body, ContentType: "application/json"}), 413, "body", "is too large")
	})

	t.Run("checks the body before authentication", func(t *testing.T) {
		response := client.Post("/api/v1/articles", support.RequestOptions{RawBody: `{"a":1}`, ContentType: "text/plain"})
		expectStatus(t, response, 415)
	})

	t.Run("accepts a charset parameter", func(t *testing.T) {
		response := client.Post("/api/v1/users/login", support.RequestOptions{RawBody: `{"user":{}}`, ContentType: "application/json; charset=utf-8"})
		expectStatus(t, response, 422)
	})
}

func TestCORS(t *testing.T) {
	app := newApp(t, map[string]string{"CLIENT_URL": "http://localhost:3000/"})
	client := app.NewClient()

	preflight := client.Do(http.MethodOptions, "/api/v1/articles", support.RequestOptions{Headers: map[string]string{
		"Origin":                         "http://localhost:3000",
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "authorization,content-type",
	}})
	expectEqual(t, preflight.Status < 300, true)
	expectEqual(t, preflight.Header.Get("Access-Control-Allow-Origin"), "http://localhost:3000")
	expectEqual(t, preflight.Header.Get("Access-Control-Allow-Credentials"), "")
	allowed := strings.ToLower(preflight.Header.Get("Access-Control-Allow-Headers"))
	expectEqual(t, strings.Contains(allowed, "authorization") && strings.Contains(allowed, "content-type"), true)
	methods := preflight.Header.Get("Access-Control-Allow-Methods")
	expectEqual(t, strings.Contains(methods, "PUT") && strings.Contains(methods, "DELETE") && !strings.Contains(methods, "PATCH"), true)

	foreign := client.Get("/health", support.RequestOptions{Headers: map[string]string{"Origin": "http://evil.example"}})
	expectEqual(t, foreign.Header.Get("Access-Control-Allow-Origin"), "")
}
