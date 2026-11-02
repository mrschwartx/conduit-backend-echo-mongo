package rest

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/modules/articles/application"
	"conduit/backend/internal/shared/infrastructure/httpx"
)

type Handler struct {
	create     *application.CreateArticle
	get        *application.GetArticle
	list       *application.ListArticles
	feed       *application.GetFeed
	update     *application.UpdateArticle
	remove     *application.DeleteArticle
	favorite   *application.SetFavorite
	unfavorite *application.SetFavorite
	tags       *application.ListPopularTags
}

func NewHandler(
	create *application.CreateArticle,
	get *application.GetArticle,
	list *application.ListArticles,
	feed *application.GetFeed,
	update *application.UpdateArticle,
	remove *application.DeleteArticle,
	favorite *application.SetFavorite,
	unfavorite *application.SetFavorite,
	tags *application.ListPopularTags,
) *Handler {
	return &Handler{
		create: create, get: get, list: list, feed: feed, update: update,
		remove: remove, favorite: favorite, unfavorite: unfavorite, tags: tags,
	}
}

type articleResponse struct {
	Article application.ArticleDTO `json:"article"`
}

type tagsResponse struct {
	Tags []string `json:"tags"`
}

func articleInput(c *echo.Context) application.ArticleInput {
	fields := httpx.Envelope(c, "article")
	return application.ArticleInput{
		Title:       fields.Get("title"),
		Description: fields.Get("description"),
		Body:        fields.Get("body"),
		TagList:     fields.Get("tagList"),
	}
}

func (h *Handler) List(c *echo.Context) error {
	query := c.QueryParams()
	page, err := h.list.Execute(c.Request().Context(), application.ListArticlesInput{
		ViewerID:  httpx.AuthUserID(c),
		Tag:       query["tag"],
		Author:    query["author"],
		Favorited: query["favorited"],
		Offset:    query["offset"],
		Limit:     query["limit"],
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, page)
}

func (h *Handler) Feed(c *echo.Context) error {
	query := c.QueryParams()
	page, err := h.feed.Execute(c.Request().Context(), httpx.AuthUserID(c), query["offset"], query["limit"])
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, page)
}

func (h *Handler) Get(c *echo.Context) error {
	article, err := h.get.Execute(c.Request().Context(), c.Param("slug"), httpx.AuthUserID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, articleResponse{Article: article})
}

func (h *Handler) Create(c *echo.Context) error {
	article, err := h.create.Execute(c.Request().Context(), httpx.AuthUserID(c), articleInput(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, articleResponse{Article: article})
}

func (h *Handler) Update(c *echo.Context) error {
	article, err := h.update.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("slug"), articleInput(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, articleResponse{Article: article})
}

func (h *Handler) Delete(c *echo.Context) error {
	if err := h.remove.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("slug")); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) Favorite(c *echo.Context) error {
	article, err := h.favorite.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("slug"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, articleResponse{Article: article})
}

func (h *Handler) Unfavorite(c *echo.Context) error {
	article, err := h.unfavorite.Execute(c.Request().Context(), httpx.AuthUserID(c), c.Param("slug"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, articleResponse{Article: article})
}

func (h *Handler) Tags(c *echo.Context) error {
	tags, err := h.tags.Execute(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tagsResponse{Tags: tags})
}
