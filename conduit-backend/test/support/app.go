package support

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"testing"

	"conduit/backend/internal/bootstrap"
)

type App struct {
	*bootstrap.Server
	BaseURL string
	tb      testing.TB
}

func StartApp(tb testing.TB, databaseURL string, overrides map[string]string) *App {
	tb.Helper()

	values := map[string]string{"DATABASE_URL": databaseURL}
	for key, value := range overrides {
		values[key] = value
	}
	cfg := TestConfig(tb, values)

	server, err := bootstrap.Start(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		tb.Fatalf("cannot start the app: %v", err)
	}
	app := &App{
		Server:  server,
		BaseURL: "http://127.0.0.1:" + strconv.Itoa(server.Port()),
		tb:      tb,
	}
	tb.Cleanup(func() {
		if err := server.Stop(); err != nil {
			tb.Errorf("stop failed: %v", err)
		}
	})
	return app
}

func (a *App) NewClient() *Client {
	return NewClient(a.tb, a.BaseURL)
}

type Credentials struct {
	Username string
	Email    string
	Password string
}

func RandomCredentials() Credentials {
	return Credentials{
		Username: "user_" + randomToken(8),
		Email:    randomToken(8) + "@mail.com",
		Password: "Password123!",
	}
}

func RegisterUser(tb testing.TB, client *Client, mutate ...func(*Credentials)) (Credentials, Response) {
	tb.Helper()
	credentials := RandomCredentials()
	for _, change := range mutate {
		change(&credentials)
	}
	response := client.Post("/api/v1/users", RequestOptions{JSON: map[string]any{"user": map[string]any{
		"username": credentials.Username,
		"email":    credentials.Email,
		"password": credentials.Password,
	}}})
	return credentials, response
}

func MustRegisterUser(tb testing.TB, client *Client) (Credentials, string) {
	tb.Helper()
	credentials, response := RegisterUser(tb, client)
	if response.Status != 201 {
		tb.Fatalf("registration failed: %d %s", response.Status, response.Raw)
	}
	return credentials, response.String("user", "token")
}

func ArticlePayload(overrides map[string]any) map[string]any {
	article := map[string]any{
		"title":       "How to train your dragon",
		"description": "Ever wonder how?",
		"body":        "You have to believe",
		"tagList":     []string{"reactjs", "angularjs", "dragons"},
	}
	for key, value := range overrides {
		if value == nil {
			delete(article, key)
			continue
		}
		article[key] = value
	}
	return map[string]any{"article": article}
}

func randomToken(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	token := make([]byte, length)
	for index := range token {
		token[index] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(token)
}
