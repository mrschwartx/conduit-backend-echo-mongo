package application

import (
	"context"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type GetFeed struct {
	articles domain.Repository
	authors  AuthorDirectory
}

func NewGetFeed(articles domain.Repository, authors AuthorDirectory) *GetFeed {
	return &GetFeed{articles: articles, authors: authors}
}

func (uc *GetFeed) Execute(ctx context.Context, viewerID string, offset, limit []string) (ArticlePageDTO, error) {
	paging, err := sharedomain.ParsePagination(offset, limit)
	if err != nil {
		return ArticlePageDTO{}, err
	}
	authorIDs, err := uc.authors.ListFollowedIDs(ctx, viewerID)
	if err != nil {
		return ArticlePageDTO{}, err
	}
	if len(authorIDs) == 0 {
		return EmptyPageDTO(), nil
	}
	page, err := uc.articles.FindPage(ctx, domain.PageQuery{
		AuthorIDs: authorIDs,
		ViewerID:  viewerID,
		Offset:    paging.Offset,
		Limit:     paging.Limit,
	})
	if err != nil {
		return ArticlePageDTO{}, err
	}
	return buildPageDTO(ctx, page, uc.authors, viewerID)
}
