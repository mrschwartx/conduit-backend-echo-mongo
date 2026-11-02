package rest

import "github.com/labstack/echo/v5"

func RegisterRoutes(e *echo.Echo, handler *Handler, authenticate, optionalAuthenticate echo.MiddlewareFunc) {
	api := e.Group("/api/v1")
	api.GET("/articles", handler.List, optionalAuthenticate)
	api.POST("/articles", handler.Create, authenticate)
	api.GET("/articles/feed", handler.Feed, authenticate)
	api.GET("/articles/:slug", handler.Get, optionalAuthenticate)
	api.PUT("/articles/:slug", handler.Update, authenticate)
	api.DELETE("/articles/:slug", handler.Delete, authenticate)
	api.POST("/articles/:slug/favorite", handler.Favorite, authenticate)
	api.DELETE("/articles/:slug/favorite", handler.Unfavorite, authenticate)
	api.GET("/tags", handler.Tags)
}
