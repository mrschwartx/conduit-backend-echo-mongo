package rest

import "github.com/labstack/echo/v5"

func RegisterRoutes(e *echo.Echo, users *UsersHandler, profiles *ProfilesHandler, authenticate, optionalAuthenticate echo.MiddlewareFunc) {
	api := e.Group("/api/v1")
	api.POST("/users/login", users.Login)
	api.POST("/users", users.Register)
	api.GET("/user", users.Current, authenticate)
	api.PUT("/user", users.Update, authenticate)
	api.GET("/profiles/:username", profiles.Get, optionalAuthenticate)
	api.POST("/profiles/:username/follow", profiles.Follow, authenticate)
	api.DELETE("/profiles/:username/follow", profiles.Unfollow, authenticate)
}
