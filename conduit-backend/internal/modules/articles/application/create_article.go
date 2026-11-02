package application

import (
	"context"
	"time"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type CreateArticle struct {
	articles domain.Repository
	slugs    *ArticleSlugs
	authors  AuthorDirectory
	now      func() time.Time
}

func NewCreateArticle(articles domain.Repository, slugs *ArticleSlugs, authors AuthorDirectory) *CreateArticle {
	return &CreateArticle{articles: articles, slugs: slugs, authors: authors, now: time.Now}
}

func (uc *CreateArticle) Execute(ctx context.Context, authorID string, input ArticleInput) (ArticleDTO, error) {
	fields, err := parseArticleFields(input, true)
	if err != nil {
		return ArticleDTO{}, err
	}
	id := uc.articles.NextID()

	var created domain.Article
	err = persistWithUniqueSlug(
		func() (string, error) { return uc.slugs.Allocate(ctx, fields.title.Value(), "") },
		func(slug string) error {
			created = domain.Create(domain.NewArticle{
				ID:          id,
				AuthorID:    authorID,
				Slug:        slug,
				Title:       *fields.title,
				Description: *fields.description,
				Body:        *fields.body,
				TagList:     *fields.tagList,
				Now:         uc.now(),
			})
			return uc.articles.Add(ctx, created)
		},
	)
	if err != nil {
		return ArticleDTO{}, err
	}
	return presentArticle(ctx, uc.articles, uc.authors, created.Slug(), authorID)
}

func presentArticle(ctx context.Context, articles domain.Repository, authors AuthorDirectory, slug, viewerID string) (ArticleDTO, error) {
	view, found, err := articles.FindViewBySlug(ctx, slug, viewerID)
	if err != nil {
		return ArticleDTO{}, err
	}
	if !found {
		return ArticleDTO{}, sharedomain.NewNotFoundError(articleResource)
	}
	profiles, err := authors.FindProfiles(ctx, []string{view.AuthorID}, viewerID)
	if err != nil {
		return ArticleDTO{}, err
	}
	return ToArticleDTO(view, profiles[view.AuthorID]), nil
}

const articleResource = "article"
