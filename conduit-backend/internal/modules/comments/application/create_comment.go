package application

import (
	"context"
	"time"

	"conduit/backend/internal/modules/comments/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

const (
	articleResource = "article"
	commentResource = "comment"
)

type CreateComment struct {
	comments domain.Repository
	articles ArticleCatalog
	authors  AuthorDirectory
	now      func() time.Time
}

func NewCreateComment(comments domain.Repository, articles ArticleCatalog, authors AuthorDirectory) *CreateComment {
	return &CreateComment{comments: comments, articles: articles, authors: authors, now: time.Now}
}

func (uc *CreateComment) Execute(ctx context.Context, authorID, slug string, raw sharedomain.Raw) (CommentDTO, error) {
	articleID, found, err := uc.articles.FindIDBySlug(ctx, slug)
	if err != nil {
		return CommentDTO{}, err
	}
	if !found {
		return CommentDTO{}, sharedomain.NewNotFoundError(articleResource)
	}

	body, err := domain.NewCommentBody(raw)
	if err != nil {
		return CommentDTO{}, err
	}
	comment, err := uc.comments.Add(ctx, domain.Create(domain.NewComment{
		ArticleID: articleID,
		AuthorID:  authorID,
		Body:      body,
		Now:       uc.now(),
	}))
	if err != nil {
		return CommentDTO{}, err
	}
	profiles, err := uc.authors.FindProfiles(ctx, []string{authorID}, authorID)
	if err != nil {
		return CommentDTO{}, err
	}
	return ToCommentDTO(comment, profiles[authorID]), nil
}
