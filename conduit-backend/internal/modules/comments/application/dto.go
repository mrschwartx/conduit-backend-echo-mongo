package application

import (
	"conduit/backend/internal/modules/comments/domain"
	sharedapp "conduit/backend/internal/shared/application"
	sharedomain "conduit/backend/internal/shared/domain"
)

type CommentDTO struct {
	ID        int64                   `json:"id"`
	CreatedAt string                  `json:"createdAt"`
	UpdatedAt string                  `json:"updatedAt"`
	Body      string                  `json:"body"`
	Author    sharedapp.AuthorProfile `json:"author"`
}

func ToCommentDTO(comment domain.Comment, author sharedapp.AuthorProfile) CommentDTO {
	return CommentDTO{
		ID:        comment.ID(),
		CreatedAt: sharedomain.FormatTimestamp(comment.CreatedAt()),
		UpdatedAt: sharedomain.FormatTimestamp(comment.UpdatedAt()),
		Body:      comment.Body().Value(),
		Author:    author,
	}
}
