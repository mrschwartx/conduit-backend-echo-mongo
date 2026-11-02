package application

import (
	"context"

	"conduit/backend/internal/modules/articles/domain"
)

type GetArticle struct {
	articles domain.Repository
	authors  AuthorDirectory
}

func NewGetArticle(articles domain.Repository, authors AuthorDirectory) *GetArticle {
	return &GetArticle{articles: articles, authors: authors}
}

func (uc *GetArticle) Execute(ctx context.Context, slug, viewerID string) (ArticleDTO, error) {
	return presentArticle(ctx, uc.articles, uc.authors, slug, viewerID)
}
