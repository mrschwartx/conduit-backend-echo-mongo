package application

import (
	"context"

	"conduit/backend/internal/modules/articles/domain"
)

type ArticleCatalog struct {
	articles domain.Repository
}

func NewArticleCatalog(articles domain.Repository) *ArticleCatalog {
	return &ArticleCatalog{articles: articles}
}

func (c *ArticleCatalog) FindIDBySlug(ctx context.Context, slug string) (string, bool, error) {
	article, found, err := c.articles.FindBySlug(ctx, slug)
	if err != nil || !found {
		return "", false, err
	}
	return article.ID(), true, nil
}
