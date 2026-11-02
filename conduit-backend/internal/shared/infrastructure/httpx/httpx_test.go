package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	sharedomain "conduit/backend/internal/shared/domain"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newEcho() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = NewErrorHandler(quietLogger())
	e.Use(JSONBody())
	return e
}

func do(e *echo.Echo, method, path, contentType, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set(echo.HeaderContentType, contentType)
	}
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	return recorder
}

func decodeErrors(t *testing.T, recorder *httptest.ResponseRecorder) map[string][]string {
	t.Helper()
	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json %q: %v", recorder.Body.String(), err)
	}
	return payload.Errors
}

func TestErrorHandlerMapsEveryKind(t *testing.T) {
	cases := []struct {
		err    error
		status int
		key    string
		text   string
	}{
		{sharedomain.NewFieldError("title", "can't be blank"), 422, "title", "can't be blank"},
		{sharedomain.NewNotFoundError("article"), 404, "article", "not found"},
		{sharedomain.NewConflictError("email"), 409, "email", "has already been taken"},
		{sharedomain.NewMissingTokenError(), 401, "token", "is missing"},
		{sharedomain.NewInvalidCredentialsError(), 401, "credentials", "are invalid"},
		{sharedomain.NewForbiddenError("comment"), 403, "comment", "forbidden"},
		{sharedomain.NewUnsupportedMediaTypeError(), 415, "content-type", "must be application/json"},
		{sharedomain.NewMalformedBodyError(), 400, "body", "is not valid JSON"},
		{sharedomain.NewPayloadTooLargeError(), 413, "body", "is too large"},
		{fmt.Errorf("wrapped: %w", sharedomain.NewNotFoundError("profile")), 404, "profile", "not found"},
		{echo.ErrNotFound, 404, "path", "not found"},
		{echo.ErrMethodNotAllowed, 404, "path", "not found"},
		{errors.New("database exploded"), 500, "server", "internal server error"},
	}
	for _, tc := range cases {
		e := newEcho()
		err := tc.err
		e.GET("/x", func(*echo.Context) error { return err })
		recorder := do(e, http.MethodGet, "/x", "", "")
		got := decodeErrors(t, recorder)
		if recorder.Code != tc.status || len(got[tc.key]) != 1 || got[tc.key][0] != tc.text {
			t.Errorf("%v: got %d %v", tc.err, recorder.Code, got)
		}
		if !strings.HasPrefix(recorder.Header().Get(echo.HeaderContentType), echo.MIMEApplicationJSON) {
			t.Errorf("%v: content type %q", tc.err, recorder.Header().Get(echo.HeaderContentType))
		}
	}
}

func TestUnhandledErrorsNeverLeakDetails(t *testing.T) {
	e := newEcho()
	e.GET("/x", func(*echo.Context) error { return errors.New("password=hunter2") })
	recorder := do(e, http.MethodGet, "/x", "", "")
	if strings.Contains(recorder.Body.String(), "hunter2") {
		t.Fatal("internal messages must not reach the client")
	}
}

func TestUnknownRoutesReturnTheErrorEnvelope(t *testing.T) {
	e := newEcho()
	recorder := do(e, http.MethodGet, "/api/v1/nothing", "", "")
	if recorder.Code != 404 || decodeErrors(t, recorder)["path"][0] != "not found" {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestJSONBodyRejectsBadRequests(t *testing.T) {
	e := newEcho()
	e.POST("/api/v1/echo", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	cases := []struct {
		name        string
		contentType string
		body        string
		status      int
		key         string
	}{
		{"wrong content type", "text/plain", `{"a":1}`, 415, "content-type"},
		{"missing content type", "", `{"a":1}`, 415, "content-type"},
		{"broken json", "application/json", `{"a":`, 400, "body"},
		{"primitive json", "application/json", `"text"`, 400, "body"},
		{"number json", "application/json", `42`, 400, "body"},
		{"null json", "application/json", `null`, 400, "body"},
		{"too large", "application/json", `{"a":"` + strings.Repeat("x", MaxJSONBytes) + `"}`, 413, "body"},
	}
	for _, tc := range cases {
		recorder := do(e, http.MethodPost, "/api/v1/echo", tc.contentType, tc.body)
		if recorder.Code != tc.status || len(decodeErrors(t, recorder)[tc.key]) != 1 {
			t.Errorf("%s: got %d %s", tc.name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestJSONBodyAllowsValidRequests(t *testing.T) {
	e := newEcho()
	e.POST("/api/v1/echo", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
	e.POST("/outside", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	for _, tc := range []struct{ contentType, body string }{
		{"application/json", `{"a":1}`},
		{"application/json; charset=utf-8", `[1,2]`},
		{"Application/JSON", ` {"a":1} `},
		{"", ``},
		{"text/plain", ``},
	} {
		if recorder := do(e, http.MethodPost, "/api/v1/echo", tc.contentType, tc.body); recorder.Code != http.StatusNoContent {
			t.Errorf("%q %q: got %d", tc.contentType, tc.body, recorder.Code)
		}
	}
	if recorder := do(e, http.MethodPost, "/outside", "text/plain", "anything"); recorder.Code != http.StatusNoContent {
		t.Errorf("only the API prefix is checked, got %d", recorder.Code)
	}
}

func TestEnvelope(t *testing.T) {
	e := newEcho()
	var captured sharedomain.Fields
	e.POST("/api/v1/users", func(c *echo.Context) error {
		captured = Envelope(c, "user")
		return c.NoContent(http.StatusNoContent)
	})

	do(e, http.MethodPost, "/api/v1/users", "application/json", `{"user":{"username":"jane","bio":null,"age":3},"other":{"x":1}}`)
	if text, _ := captured.Get("username").Text(); text != "jane" || !captured.Get("bio").IsSet() || captured.Get("bio").Value() != nil || !captured.Get("age").IsSet() {
		t.Fatalf("unexpected fields %+v", captured)
	}
	if captured.Get("Username").IsSet() {
		t.Fatal("keys are case sensitive")
	}

	for _, body := range []string{``, `{}`, `[]`, `{"user":null}`, `{"user":"x"}`, `{"user":[1]}`, `{"users":{"username":"a"}}`} {
		captured = nil
		do(e, http.MethodPost, "/api/v1/users", "application/json", body)
		if captured.Get("username").IsSet() {
			t.Errorf("%q must yield no fields", body)
		}
	}
}

func TestAuthUserContext(t *testing.T) {
	e := newEcho()
	var before, after string
	e.GET("/x", func(c *echo.Context) error {
		before = AuthUserID(c)
		SetAuthUser(c, "user-1")
		after = AuthUserID(c)
		return c.NoContent(http.StatusNoContent)
	})
	do(e, http.MethodGet, "/x", "", "")
	if before != "" || after != "user-1" {
		t.Fatalf("got %q %q", before, after)
	}
}

func TestHeadRequestsGetNoBody(t *testing.T) {
	e := newEcho()
	e.GET("/x", func(*echo.Context) error { return sharedomain.NewNotFoundError("x") })
	request := httptest.NewRequest(http.MethodHead, "/x", bytes.NewReader(nil))
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound || recorder.Body.Len() != 0 {
		t.Fatalf("got %d %q", recorder.Code, recorder.Body.String())
	}
}
