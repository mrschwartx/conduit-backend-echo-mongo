package rest

import "github.com/labstack/echo/v5"

func RegisterRoutes(e *echo.Echo, handler *Handler, authenticate, optionalAuthenticate echo.MiddlewareFunc) {
	api := e.Group("/api/v1")
	api.GET("/articles/:slug/comments", handler.List, optionalAuthenticate)
	api.POST("/articles/:slug/comments", handler.Create, authenticate)
	api.DELETE("/articles/:slug/comments/:id", handler.Delete, authenticate)
}
