package application

import (
	"context"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type ListArticlesInput struct {
	ViewerID  string
	Tag       []string
	Author    []string
	Favorited []string
	Offset    []string
	Limit     []string
}

type ListArticles struct {
	articles domain.Repository
	authors  AuthorDirectory
}

func NewListArticles(articles domain.Repository, authors AuthorDirectory) *ListArticles {
	return &ListArticles{articles: articles, authors: authors}
}

func singleFilter(errs *sharedomain.ErrorCollector, name string, values []string) *string {
	switch len(values) {
	case 0:
		return nil
	case 1:
		return &values[0]
	default:
		errs.Add(name, sharedomain.Invalid)
		return nil
	}
}

func (uc *ListArticles) Execute(ctx context.Context, input ListArticlesInput) (ArticlePageDTO, error) {
	errs := sharedomain.NewErrorCollector()
	paging, err := sharedomain.ParsePagination(input.Offset, input.Limit)
	if captured := errs.Capture(err); captured != nil {
		return ArticlePageDTO{}, captured
	}
	tag := singleFilter(errs, "tag", input.Tag)
	author := singleFilter(errs, "author", input.Author)
	favorited := singleFilter(errs, "favorited", input.Favorited)
	if err := errs.Err(); err != nil {
		return ArticlePageDTO{}, err
	}

	query := domain.PageQuery{Tag: tag, ViewerID: input.ViewerID, Offset: paging.Offset, Limit: paging.Limit}
	if author != nil {
		id, found, err := uc.authors.FindIDByUsername(ctx, *author)
		if err != nil {
			return ArticlePageDTO{}, err
		}
		if !found {
			return EmptyPageDTO(), nil
		}
		query.AuthorID = id
	}
	if favorited != nil {
		id, found, err := uc.authors.FindIDByUsername(ctx, *favorited)
		if err != nil {
			return ArticlePageDTO{}, err
		}
		if !found {
			return EmptyPageDTO(), nil
		}
		query.FavoritedByID = id
	}

	page, err := uc.articles.FindPage(ctx, query)
	if err != nil {
		return ArticlePageDTO{}, err
	}
	return buildPageDTO(ctx, page, uc.authors, input.ViewerID)
}
