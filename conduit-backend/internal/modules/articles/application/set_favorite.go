package application

import (
	"context"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type SetFavorite struct {
	articles domain.Repository
	authors  AuthorDirectory
	favorite bool
}

func NewSetFavorite(articles domain.Repository, authors AuthorDirectory, favorite bool) *SetFavorite {
	return &SetFavorite{articles: articles, authors: authors, favorite: favorite}
}

func (uc *SetFavorite) Execute(ctx context.Context, userID, slug string) (ArticleDTO, error) {
	article, found, err := uc.articles.FindBySlug(ctx, slug)
	if err != nil {
		return ArticleDTO{}, err
	}
	if !found {
		return ArticleDTO{}, sharedomain.NewNotFoundError(articleResource)
	}
	if uc.favorite {
		err = uc.articles.Favorite(ctx, article.ID(), userID)
	} else {
		err = uc.articles.Unfavorite(ctx, article.ID(), userID)
	}
	if err != nil {
		return ArticleDTO{}, err
	}
	return presentArticle(ctx, uc.articles, uc.authors, slug, userID)
}
