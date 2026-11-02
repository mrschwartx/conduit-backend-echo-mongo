package application

import (
	"context"
	"time"

	"conduit/backend/internal/modules/articles/domain"
	sharedapp "conduit/backend/internal/shared/application"
	sharedomain "conduit/backend/internal/shared/domain"
)

type DeleteArticle struct {
	articles domain.Repository
	events   sharedapp.EventBus
	now      func() time.Time
}

func NewDeleteArticle(articles domain.Repository, events sharedapp.EventBus) *DeleteArticle {
	return &DeleteArticle{articles: articles, events: events, now: time.Now}
}

func (uc *DeleteArticle) Execute(ctx context.Context, requesterID, slug string) error {
	article, found, err := uc.articles.FindBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if !found {
		return sharedomain.NewNotFoundError(articleResource)
	}
	if !article.IsAuthoredBy(requesterID) {
		return sharedomain.NewForbiddenError(articleResource)
	}
	deleted, err := uc.articles.DeleteByID(ctx, article.ID())
	if err != nil {
		return err
	}
	if !deleted {
		return sharedomain.NewNotFoundError(articleResource)
	}
	uc.events.Publish(context.WithoutCancel(ctx), sharedomain.NewEvent(
		sharedomain.ArticleDeletedEvent,
		sharedomain.ArticleDeleted{ArticleID: article.ID()},
		uc.now(),
	))
	return nil
}
