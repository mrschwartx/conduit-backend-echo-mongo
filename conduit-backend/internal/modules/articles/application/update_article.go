package application

import (
	"context"
	"time"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type UpdateArticle struct {
	articles domain.Repository
	slugs    *ArticleSlugs
	authors  AuthorDirectory
	now      func() time.Time
}

func NewUpdateArticle(articles domain.Repository, slugs *ArticleSlugs, authors AuthorDirectory) *UpdateArticle {
	return &UpdateArticle{articles: articles, slugs: slugs, authors: authors, now: time.Now}
}

func (uc *UpdateArticle) Execute(ctx context.Context, requesterID, slug string, input ArticleInput) (ArticleDTO, error) {
	article, found, err := uc.articles.FindBySlug(ctx, slug)
	if err != nil {
		return ArticleDTO{}, err
	}
	if !found {
		return ArticleDTO{}, sharedomain.NewNotFoundError(articleResource)
	}
	if !article.IsAuthoredBy(requesterID) {
		return ArticleDTO{}, sharedomain.NewForbiddenError(articleResource)
	}

	if input.hasAny() {
		fields, err := parseArticleFields(input, false)
		if err != nil {
			return ArticleDTO{}, err
		}
		titleChanged := fields.title != nil && fields.title.Value() != article.Title().Value()
		err = persistWithUniqueSlug(
			func() (string, error) {
				if titleChanged {
					return uc.slugs.Allocate(ctx, fields.title.Value(), article.ID())
				}
				return article.Slug(), nil
			},
			func(nextSlug string) error {
				article.Revise(domain.Revision{
					Slug:        nextSlug,
					Title:       fields.title,
					Description: fields.description,
					Body:        fields.body,
					TagList:     fields.tagList,
				}, uc.now())
				updated, err := uc.articles.Update(ctx, article)
				if err != nil {
					return err
				}
				if !updated {
					return sharedomain.NewNotFoundError(articleResource)
				}
				return nil
			},
		)
		if err != nil {
			return ArticleDTO{}, err
		}
	}
	return presentArticle(ctx, uc.articles, uc.authors, article.Slug(), requesterID)
}
