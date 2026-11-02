package application

import (
	"context"

	"conduit/backend/internal/modules/comments/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type ListComments struct {
	comments domain.Repository
	articles ArticleCatalog
	authors  AuthorDirectory
}

func NewListComments(comments domain.Repository, articles ArticleCatalog, authors AuthorDirectory) *ListComments {
	return &ListComments{comments: comments, articles: articles, authors: authors}
}

func (uc *ListComments) Execute(ctx context.Context, slug, viewerID string) ([]CommentDTO, error) {
	articleID, found, err := uc.articles.FindIDBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, sharedomain.NewNotFoundError(articleResource)
	}

	comments, err := uc.comments.ListByArticle(ctx, articleID)
	if err != nil {
		return nil, err
	}
	authorIDs := make([]string, 0, len(comments))
	for _, comment := range comments {
		authorIDs = append(authorIDs, comment.AuthorID())
	}
	profiles, err := uc.authors.FindProfiles(ctx, authorIDs, viewerID)
	if err != nil {
		return nil, err
	}
	result := make([]CommentDTO, 0, len(comments))
	for _, comment := range comments {
		result = append(result, ToCommentDTO(comment, profiles[comment.AuthorID()]))
	}
	return result, nil
}
