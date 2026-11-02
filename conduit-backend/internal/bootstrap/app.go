package bootstrap

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"conduit/backend/internal/shared/infrastructure/httpx"
)

func NewApp(container *Container) (*echo.Echo, error) {
	cfg := container.Config
	logger := container.Logger

	e := echo.New()
	e.Logger = logger
	e.HTTPErrorHandler = httpx.NewErrorHandler(logger)

	cors, err := middleware.CORSConfig{
		AllowOrigins: []string{strings.TrimRight(cfg.ClientURL, "/")},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderContentType, echo.HeaderAuthorization},
	}.ToMiddleware()
	if err != nil {
		return nil, err
	}
	e.Use(cors)

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		HandleError: true,
		LogMethod:   true,
		LogURIPath:  true,
		LogStatus:   true,
		LogLatency:  true,
		LogValuesFunc: func(_ *echo.Context, values middleware.RequestLoggerValues) error {
			logger.Info("HTTP request",
				"method", values.Method,
				"path", values.URIPath,
				"status", values.Status,
				"durationMs", float64(values.Latency.Microseconds())/1000,
			)
			return nil
		},
	}))
	e.Use(httpx.JSONBody())

	e.GET("/health", func(c *echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	container.Users.RegisterRoutes(e)
	container.Articles.RegisterRoutes(e)
	container.Comments.RegisterRoutes(e)
	return e, nil
}
