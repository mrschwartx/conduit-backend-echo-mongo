package rest

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/modules/users/application"
	"conduit/backend/internal/shared/infrastructure/httpx"
)

type UsersHandler struct {
	register *application.RegisterUser
	login    *application.AuthenticateUser
	current  *application.GetCurrentUser
	update   *application.UpdateCurrentUser
}

func NewUsersHandler(
	register *application.RegisterUser,
	login *application.AuthenticateUser,
	current *application.GetCurrentUser,
	update *application.UpdateCurrentUser,
) *UsersHandler {
	return &UsersHandler{register: register, login: login, current: current, update: update}
}

type userResponse struct {
	User application.UserDTO `json:"user"`
}

func (h *UsersHandler) Register(c *echo.Context) error {
	fields := httpx.Envelope(c, "user")
	user, err := h.register.Execute(c.Request().Context(), application.RegisterUserInput{
		Username: fields.Get("username"),
		Email:    fields.Get("email"),
		Password: fields.Get("password"),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, userResponse{User: user})
}

func (h *UsersHandler) Login(c *echo.Context) error {
	fields := httpx.Envelope(c, "user")
	user, err := h.login.Execute(c.Request().Context(), application.AuthenticateUserInput{
		Email:    fields.Get("email"),
		Password: fields.Get("password"),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, userResponse{User: user})
}

func (h *UsersHandler) Current(c *echo.Context) error {
	user, err := h.current.Execute(c.Request().Context(), httpx.AuthUserID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, userResponse{User: user})
}

func (h *UsersHandler) Update(c *echo.Context) error {
	fields := httpx.Envelope(c, "user")
	user, err := h.update.Execute(c.Request().Context(), application.UpdateCurrentUserInput{
		UserID: httpx.AuthUserID(c),
		Changes: application.UserChanges{
			Username: fields.Get("username"),
			Email:    fields.Get("email"),
			Password: fields.Get("password"),
			Bio:      fields.Get("bio"),
			Image:    fields.Get("image"),
		},
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, userResponse{User: user})
}

type ProfilesHandler struct {
	get      *application.GetProfile
	follow   *application.FollowUser
	unfollow *application.UnfollowUser
}

func NewProfilesHandler(get *application.GetProfile, follow *application.FollowUser, unfollow *application.UnfollowUser) *ProfilesHandler {
	return &ProfilesHandler{get: get, follow: follow, unfollow: unfollow}
}

type profileResponse struct {
	Profile application.ProfileDTO `json:"profile"`
}

func (h *ProfilesHandler) Get(c *echo.Context) error {
	profile, err := h.get.Execute(c.Request().Context(), c.Param("username"), httpx.AuthUserID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, profileResponse{Profile: profile})
}

func (h *ProfilesHandler) Follow(c *echo.Context) error {
	profile, err := h.follow.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("username"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, profileResponse{Profile: profile})
}

func (h *ProfilesHandler) Unfollow(c *echo.Context) error {
	profile, err := h.unfollow.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("username"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, profileResponse{Profile: profile})
}
