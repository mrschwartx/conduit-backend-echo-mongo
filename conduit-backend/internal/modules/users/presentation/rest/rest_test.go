package rest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/modules/users/application"
	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/httpx"
)

type tokens struct{}

func (tokens) Issue(string) (string, error) { return "", nil }

func (tokens) Verify(token string) (string, error) {
	if token == "good" {
		return "user-1", nil
	}
	return "", sharedomain.NewInvalidTokenError()
}

type repository struct {
	domain.Repository
	known map[string]bool
}

func (r repository) ExistsByID(_ context.Context, id string) (bool, error) {
	return r.known[id], nil
}

func newApp(t *testing.T, known map[string]bool, optional bool) *echo.Echo {
	t.Helper()
	verify := application.NewVerifyAccessToken(tokens{}, repository{known: known})
	e := echo.New()
	e.HTTPErrorHandler = httpx.NewErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.GET("/me", func(c *echo.Context) error {
		return c.String(http.StatusOK, "user="+httpx.AuthUserID(c))
	}, Authenticate(verify, optional))
	return e
}

func call(e *echo.Echo, header string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	if header != "" {
		request.Header.Set(echo.HeaderAuthorization, header)
	}
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	return recorder
}

func tokenProblem(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var payload httpx.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil || len(payload.Errors["token"]) != 1 {
		t.Fatalf("unexpected body %q", recorder.Body.String())
	}
	return payload.Errors["token"][0]
}

func TestRequiredAuthentication(t *testing.T) {
	e := newApp(t, map[string]bool{"user-1": true}, false)

	if recorder := call(e, "Token good"); recorder.Code != 200 || recorder.Body.String() != "user=user-1" {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := call(e, ""); recorder.Code != 401 || tokenProblem(t, recorder) != "is missing" {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
	for _, header := range []string{"Bearer good", "token good", "Token", "Token  good", "Token good extra", "good"} {
		if recorder := call(e, header); recorder.Code != 401 || tokenProblem(t, recorder) != "is invalid" {
			t.Errorf("%q: got %d %s", header, recorder.Code, recorder.Body.String())
		}
	}
	if recorder := call(e, "Token bad"); recorder.Code != 401 {
		t.Errorf("unverifiable tokens are rejected, got %d", recorder.Code)
	}
}

func TestTokensOfDeletedUsersAreRejected(t *testing.T) {
	e := newApp(t, map[string]bool{}, false)
	if recorder := call(e, "Token good"); recorder.Code != 401 || tokenProblem(t, recorder) != "is invalid" {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestOptionalAuthentication(t *testing.T) {
	e := newApp(t, map[string]bool{"user-1": true}, true)

	if recorder := call(e, ""); recorder.Code != 200 || recorder.Body.String() != "user=" {
		t.Fatalf("anonymous requests pass: %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := call(e, "Token good"); recorder.Body.String() != "user=user-1" {
		t.Fatalf("got %s", recorder.Body.String())
	}
	if recorder := call(e, "Bearer good"); recorder.Code != 401 {
		t.Fatalf("a malformed header is still an error, got %d", recorder.Code)
	}
	if recorder := call(e, "Token bad"); recorder.Code != 401 {
		t.Fatalf("an invalid token is still an error, got %d", recorder.Code)
	}
}
