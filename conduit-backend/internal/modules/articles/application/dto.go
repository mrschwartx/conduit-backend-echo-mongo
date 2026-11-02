package application

import (
	"context"

	"conduit/backend/internal/modules/articles/domain"
	sharedapp "conduit/backend/internal/shared/application"
	sharedomain "conduit/backend/internal/shared/domain"
)

type ArticleSummaryDTO struct {
	Slug           string                  `json:"slug"`
	Title          string                  `json:"title"`
	Description    string                  `json:"description"`
	TagList        []string                `json:"tagList"`
	CreatedAt      string                  `json:"createdAt"`
	UpdatedAt      string                  `json:"updatedAt"`
	Favorited      bool                    `json:"favorited"`
	FavoritesCount int                     `json:"favoritesCount"`
	Author         sharedapp.AuthorProfile `json:"author"`
}

type ArticleDTO struct {
	Slug           string                  `json:"slug"`
	Title          string                  `json:"title"`
	Description    string                  `json:"description"`
	Body           string                  `json:"body"`
	TagList        []string                `json:"tagList"`
	CreatedAt      string                  `json:"createdAt"`
	UpdatedAt      string                  `json:"updatedAt"`
	Favorited      bool                    `json:"favorited"`
	FavoritesCount int                     `json:"favoritesCount"`
	Author         sharedapp.AuthorProfile `json:"author"`
}

type ArticlePageDTO struct {
	Articles      []ArticleSummaryDTO `json:"articles"`
	ArticlesCount int                 `json:"articlesCount"`
}

func tagsOrEmpty(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func ToArticleDTO(view domain.View, author sharedapp.AuthorProfile) ArticleDTO {
	return ArticleDTO{
		Slug:           view.Slug,
		Title:          view.Title,
		Description:    view.Description,
		Body:           view.Body,
		TagList:        tagsOrEmpty(view.TagList),
		CreatedAt:      sharedomain.FormatTimestamp(view.CreatedAt),
		UpdatedAt:      sharedomain.FormatTimestamp(view.UpdatedAt),
		Favorited:      view.Favorited,
		FavoritesCount: view.FavoritesCount,
		Author:         author,
	}
}

func ToArticleSummaryDTO(view domain.View, author sharedapp.AuthorProfile) ArticleSummaryDTO {
	return ArticleSummaryDTO{
		Slug:           view.Slug,
		Title:          view.Title,
		Description:    view.Description,
		TagList:        tagsOrEmpty(view.TagList),
		CreatedAt:      sharedomain.FormatTimestamp(view.CreatedAt),
		UpdatedAt:      sharedomain.FormatTimestamp(view.UpdatedAt),
		Favorited:      view.Favorited,
		FavoritesCount: view.FavoritesCount,
		Author:         author,
	}
}

func EmptyPageDTO() ArticlePageDTO {
	return ArticlePageDTO{Articles: []ArticleSummaryDTO{}}
}

func buildPageDTO(ctx context.Context, page domain.Page, authors AuthorDirectory, viewerID string) (ArticlePageDTO, error) {
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.AuthorID)
	}
	profiles, err := authors.FindProfiles(ctx, ids, viewerID)
	if err != nil {
		return ArticlePageDTO{}, err
	}
	result := ArticlePageDTO{Articles: make([]ArticleSummaryDTO, 0, len(page.Items)), ArticlesCount: page.Total}
	for _, item := range page.Items {
		result.Articles = append(result.Articles, ToArticleSummaryDTO(item, profiles[item.AuthorID]))
	}
	return result, nil
}
