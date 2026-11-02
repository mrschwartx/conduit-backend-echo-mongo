package application

import (
	"context"
	"errors"

	"conduit/backend/internal/modules/articles/domain"
)

const maxSlugAttempts = 5

type ArticleSlugs struct {
	articles domain.Repository
}

func NewArticleSlugs(articles domain.Repository) *ArticleSlugs {
	return &ArticleSlugs{articles: articles}
}

func (s *ArticleSlugs) Allocate(ctx context.Context, title, excludeID string) (string, error) {
	base := domain.Slugify(title)
	for attempt := 1; ; attempt++ {
		candidate := domain.SlugCandidate(base, attempt)
		if domain.IsReservedSlug(candidate) {
			continue
		}
		taken, err := s.articles.SlugTaken(ctx, candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
}

func persistWithUniqueSlug(allocate func() (string, error), persist func(slug string) error) error {
	for attempt := 1; ; attempt++ {
		slug, err := allocate()
		if err != nil {
			return err
		}
		err = persist(slug)
		if err == nil {
			return nil
		}
		if !errors.Is(err, domain.ErrSlugTaken) || attempt >= maxSlugAttempts {
			return err
		}
	}
}
