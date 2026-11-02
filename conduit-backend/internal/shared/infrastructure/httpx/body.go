package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	sharedomain "conduit/backend/internal/shared/domain"
)

const (
	MaxJSONBytes = 256 * 1024
	apiPrefix    = "/api/v1"
	bodyKey      = "request.body"
)

func hasBody(request *http.Request) bool {
	if request.ContentLength > 0 {
		return true
	}
	return request.ContentLength < 0 && len(request.TransferEncoding) > 0
}

func isJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == echo.MIMEApplicationJSON
}

func JSONBody() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			request := c.Request()
			if !strings.HasPrefix(request.URL.Path, apiPrefix+"/") && request.URL.Path != apiPrefix {
				return next(c)
			}
			if !hasBody(request) {
				return next(c)
			}
			if !isJSONContentType(request.Header.Get(echo.HeaderContentType)) {
				return sharedomain.NewUnsupportedMediaTypeError()
			}
			body, err := io.ReadAll(http.MaxBytesReader(c.Response(), request.Body, MaxJSONBytes))
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					return sharedomain.NewPayloadTooLargeError()
				}
				return sharedomain.NewMalformedBodyError()
			}
			trimmed := bytes.TrimSpace(body)
			if len(trimmed) > 0 {
				if (trimmed[0] != '{' && trimmed[0] != '[') || !json.Valid(trimmed) {
					return sharedomain.NewMalformedBodyError()
				}
			}
			c.Set(bodyKey, trimmed)
			return next(c)
		}
	}
}

func Envelope(c *echo.Context, key string) sharedomain.Fields {
	body, _ := c.Get(bodyKey).([]byte)
	if len(body) == 0 || body[0] != '{' {
		return nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	inner, ok := envelope[key]
	if !ok || len(inner) == 0 || inner[0] != '{' {
		return nil
	}
	var fields map[string]sharedomain.Raw
	if err := json.Unmarshal(inner, &fields); err != nil {
		return nil
	}
	return sharedomain.Fields(fields)
}
