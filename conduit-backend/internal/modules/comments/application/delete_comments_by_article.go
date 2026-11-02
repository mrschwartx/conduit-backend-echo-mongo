package application

import (
	"context"

	"conduit/backend/internal/modules/comments/domain"
)

type DeleteCommentsByArticle struct {
	comments domain.Repository
}

func NewDeleteCommentsByArticle(comments domain.Repository) *DeleteCommentsByArticle {
	return &DeleteCommentsByArticle{comments: comments}
}

func (uc *DeleteCommentsByArticle) Execute(ctx context.Context, articleID string) (int64, error) {
	return uc.comments.DeleteByArticle(ctx, articleID)
}
