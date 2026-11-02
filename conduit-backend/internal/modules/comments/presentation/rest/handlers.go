package rest

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/modules/comments/application"
	"conduit/backend/internal/shared/infrastructure/httpx"
)

type Handler struct {
	create *application.CreateComment
	list   *application.ListComments
	remove *application.DeleteComment
}

func NewHandler(create *application.CreateComment, list *application.ListComments, remove *application.DeleteComment) *Handler {
	return &Handler{create: create, list: list, remove: remove}
}

type commentResponse struct {
	Comment application.CommentDTO `json:"comment"`
}

type commentsResponse struct {
	Comments []application.CommentDTO `json:"comments"`
}

func (h *Handler) List(c *echo.Context) error {
	comments, err := h.list.Execute(c.Request().Context(), c.Param("slug"), httpx.AuthUserID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, commentsResponse{Comments: comments})
}

func (h *Handler) Create(c *echo.Context) error {
	fields := httpx.Envelope(c, "comment")
	comment, err := h.create.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("slug"), fields.Get("body"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, commentResponse{Comment: comment})
}

func (h *Handler) Delete(c *echo.Context) error {
	if err := h.remove.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("slug"), c.Param("id")); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
