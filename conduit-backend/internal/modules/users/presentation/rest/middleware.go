package rest

import (
	"regexp"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/modules/users/application"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/httpx"
)

var tokenHeader = regexp.MustCompile(`^Token (\S+)$`)

func Authenticate(verify *application.VerifyAccessToken, optional bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get(echo.HeaderAuthorization)
			if header == "" {
				if !optional {
					return sharedomain.NewMissingTokenError()
				}
				httpx.SetAuthUser(c, "")
				return next(c)
			}

			match := tokenHeader.FindStringSubmatch(header)
			if match == nil {
				return sharedomain.NewInvalidTokenError()
			}
			userID, err := verify.Execute(c.Request().Context(), match[1])
			if err != nil {
				return err
			}
			httpx.SetAuthUser(c, userID)
			return next(c)
		}
	}
}
