package domain

import (
	"context"
	"time"
)

type View struct {
	ID             string
	AuthorID       string
	Slug           string
	Title          string
	Description    string
	Body           string
	TagList        []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FavoritesCount int
	Favorited      bool
}

type PageQuery struct {
	Tag           *string
	AuthorID      string
	AuthorIDs     []string
	FavoritedByID string
	ViewerID      string
	Offset        int
	Limit         int
}

type Page struct {
	Items []View
	Total int
}

type Repository interface {
	NextID() string
	Add(ctx context.Context, article Article) error
	Update(ctx context.Context, article Article) (bool, error)
	FindBySlug(ctx context.Context, slug string) (Article, bool, error)
	FindViewBySlug(ctx context.Context, slug, viewerID string) (View, bool, error)
	FindPage(ctx context.Context, query PageQuery) (Page, error)
	SlugTaken(ctx context.Context, slug, excludeID string) (bool, error)
	DeleteByID(ctx context.Context, id string) (bool, error)
	Favorite(ctx context.Context, articleID, userID string) error
	Unfavorite(ctx context.Context, articleID, userID string) error
	PopularTags(ctx context.Context, limit int) ([]string, error)
}
