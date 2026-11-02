package application

import (
	"context"

	sharedapp "conduit/backend/internal/shared/application"
)

type AuthorDirectory = sharedapp.AuthorDirectory

type ArticleCatalog interface {
	FindIDBySlug(ctx context.Context, slug string) (string, bool, error)
}
