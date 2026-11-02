//go:build integration

package integration

import (
	"reflect"
	"strings"
	"testing"

	"conduit/backend/test/support"
)

func expectStatus(t *testing.T, response support.Response, want int) {
	t.Helper()
	if response.Status != want {
		t.Fatalf("status = %d, want %d, body = %s", response.Status, want, response.Raw)
	}
}

func expectEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func expectDeep(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func expectProblem(t *testing.T, response support.Response, status int, field, message string) {
	t.Helper()
	expectStatus(t, response, status)
	expectDeep(t, response.Errors(field), []string{message})
}

type account struct {
	client *support.Client
	token  string
	creds  support.Credentials
}

func (a account) auth() support.RequestOptions {
	return support.RequestOptions{Token: a.token}
}

func (a account) with(options support.RequestOptions) support.RequestOptions {
	options.Token = a.token
	return options
}

func newApp(t *testing.T, overrides ...map[string]string) *support.App {
	t.Helper()
	database := support.StartTestDatabase(t)
	merged := map[string]string{}
	for _, extra := range overrides {
		for key, value := range extra {
			merged[key] = value
		}
	}
	return support.StartApp(t, database.URL, merged)
}

func signIn(t *testing.T, app *support.App) account {
	t.Helper()
	client := app.NewClient()
	creds, token := support.MustRegisterUser(t, client)
	return account{client: client, token: token, creds: creds}
}

func publish(t *testing.T, author account, overrides map[string]any) support.Response {
	t.Helper()
	response := author.client.Post("/api/v1/articles", author.with(support.RequestOptions{JSON: support.ArticlePayload(overrides)}))
	expectStatus(t, response, 201)
	return response
}

func titles(response support.Response) []string {
	out := []string{}
	for _, entry := range response.Slice("articles") {
		article, _ := entry.(map[string]any)
		title, _ := article["title"].(string)
		out = append(out, title)
	}
	return out
}

func slugsOf(response support.Response) []string {
	out := []string{}
	for _, entry := range response.Slice("articles") {
		article, _ := entry.(map[string]any)
		slug, _ := article["slug"].(string)
		out = append(out, slug)
	}
	return out
}

func upper(value string) string { return strings.ToUpper(value) }

func noAuth() support.RequestOptions { return support.RequestOptions{} }

func tokenOf(token string) support.RequestOptions { return support.RequestOptions{Token: token} }
