package application

import (
	"context"

	"conduit/backend/internal/modules/articles/domain"
)

const PopularTagsLimit = 20

type ListPopularTags struct {
	articles domain.Repository
}

func NewListPopularTags(articles domain.Repository) *ListPopularTags {
	return &ListPopularTags{articles: articles}
}

func (uc *ListPopularTags) Execute(ctx context.Context) ([]string, error) {
	tags, err := uc.articles.PopularTags(ctx, PopularTagsLimit)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		return []string{}, nil
	}
	return tags, nil
}
