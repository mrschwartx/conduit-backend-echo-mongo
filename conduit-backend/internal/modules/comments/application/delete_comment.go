package application

import (
	"context"
	"regexp"
	"strconv"

	"conduit/backend/internal/modules/comments/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

const maxCommentIDDigits = 18

var digits = regexp.MustCompile(`^\d+$`)

type DeleteComment struct {
	comments domain.Repository
	articles ArticleCatalog
}

func NewDeleteComment(comments domain.Repository, articles ArticleCatalog) *DeleteComment {
	return &DeleteComment{comments: comments, articles: articles}
}

func (uc *DeleteComment) Execute(ctx context.Context, requesterID, slug, rawID string) error {
	articleID, found, err := uc.articles.FindIDBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if !found {
		return sharedomain.NewNotFoundError(articleResource)
	}

	if !digits.MatchString(rawID) {
		return sharedomain.NewFieldError("id", "must be an integer")
	}
	if len(rawID) > maxCommentIDDigits {
		return sharedomain.NewNotFoundError(commentResource)
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return sharedomain.NewNotFoundError(commentResource)
	}

	comment, found, err := uc.comments.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !found || !comment.BelongsToArticle(articleID) {
		return sharedomain.NewNotFoundError(commentResource)
	}
	if !comment.IsAuthoredBy(requesterID) {
		return sharedomain.NewForbiddenError(commentResource)
	}
	deleted, err := uc.comments.DeleteByID(ctx, comment.ID())
	if err != nil {
		return err
	}
	if !deleted {
		return sharedomain.NewNotFoundError(commentResource)
	}
	return nil
}
