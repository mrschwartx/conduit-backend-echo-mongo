package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	sharedomain "conduit/backend/internal/shared/domain"
)

type ErrorResponse struct {
	Errors map[string][]string `json:"errors"`
}

var statusByKind = map[sharedomain.ErrorKind]int{
	sharedomain.KindValidation:       http.StatusUnprocessableEntity,
	sharedomain.KindNotFound:         http.StatusNotFound,
	sharedomain.KindConflict:         http.StatusConflict,
	sharedomain.KindAuthentication:   http.StatusUnauthorized,
	sharedomain.KindForbidden:        http.StatusForbidden,
	sharedomain.KindUnsupportedMedia: http.StatusUnsupportedMediaType,
	sharedomain.KindMalformedBody:    http.StatusBadRequest,
	sharedomain.KindPayloadTooLarge:  http.StatusRequestEntityTooLarge,
}

func NewErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if response, _ := echo.UnwrapResponse(c.Response()); response != nil && response.Committed {
			return
		}

		status, fields, client := classify(err)
		if client {
			logger.Warn("Request rejected", "status", status, "message", err.Error())
		} else {
			logger.Error("Unhandled error", "message", err.Error())
		}

		if c.Request().Method == http.MethodHead {
			_ = c.NoContent(status)
			return
		}
		if sendErr := c.JSON(status, ErrorResponse{Errors: fields}); sendErr != nil {
			logger.Error("Failed to send error response", "message", sendErr.Error())
		}
	}
}

func classify(err error) (int, map[string][]string, bool) {
	var domainErr *sharedomain.Error
	if errors.As(err, &domainErr) {
		if status, known := statusByKind[domainErr.Kind]; known {
			return status, domainErr.Fields, true
		}
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge, sharedomain.FieldsOf(sharedomain.NewPayloadTooLargeError()), true
	}

	switch echo.StatusCode(err) {
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return http.StatusNotFound, map[string][]string{"path": {"not found"}}, true
	case http.StatusRequestEntityTooLarge:
		return http.StatusRequestEntityTooLarge, sharedomain.FieldsOf(sharedomain.NewPayloadTooLargeError()), true
	}
	return http.StatusInternalServerError, map[string][]string{"server": {"internal server error"}}, false
}
