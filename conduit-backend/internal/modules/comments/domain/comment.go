package domain

import (
	"context"
	"time"

	sharedomain "conduit/backend/internal/shared/domain"
)

var bodyRule = sharedomain.TextRule{Min: 1, Max: 1000}

type CommentBody struct{ value string }

func NewCommentBody(raw sharedomain.Raw) (CommentBody, error) {
	value, err := sharedomain.ValidateText("body", raw, bodyRule)
	return CommentBody{value: value}, err
}

func RestoreCommentBody(value string) CommentBody { return CommentBody{value: value} }

func (b CommentBody) Value() string { return b.value }

type Comment struct {
	id        int64
	articleID string
	authorID  string
	body      CommentBody
	createdAt time.Time
	updatedAt time.Time
}

type Snapshot struct {
	ID        int64
	ArticleID string
	AuthorID  string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NewComment struct {
	ArticleID string
	AuthorID  string
	Body      CommentBody
	Now       time.Time
}

func Create(params NewComment) Comment {
	timestamp := sharedomain.TruncateToMicrosecond(params.Now)
	return Comment{
		articleID: params.ArticleID,
		authorID:  params.AuthorID,
		body:      params.Body,
		createdAt: timestamp,
		updatedAt: timestamp,
	}
}

func Restore(snapshot Snapshot) Comment {
	return Comment{
		id:        snapshot.ID,
		articleID: snapshot.ArticleID,
		authorID:  snapshot.AuthorID,
		body:      RestoreCommentBody(snapshot.Body),
		createdAt: snapshot.CreatedAt.UTC(),
		updatedAt: snapshot.UpdatedAt.UTC(),
	}
}

func (c Comment) ID() int64            { return c.id }
func (c Comment) ArticleID() string    { return c.articleID }
func (c Comment) AuthorID() string     { return c.authorID }
func (c Comment) Body() CommentBody    { return c.body }
func (c Comment) CreatedAt() time.Time { return c.createdAt }
func (c Comment) UpdatedAt() time.Time { return c.updatedAt }

func (c Comment) BelongsToArticle(articleID string) bool {
	return c.articleID == articleID
}

func (c Comment) IsAuthoredBy(userID string) bool {
	return c.authorID == userID
}

func (c Comment) Snapshot() Snapshot {
	return Snapshot{
		ID:        c.id,
		ArticleID: c.articleID,
		AuthorID:  c.authorID,
		Body:      c.body.Value(),
		CreatedAt: c.createdAt,
		UpdatedAt: c.updatedAt,
	}
}

type Repository interface {
	Add(ctx context.Context, comment Comment) (Comment, error)
	FindByID(ctx context.Context, id int64) (Comment, bool, error)
	ListByArticle(ctx context.Context, articleID string) ([]Comment, error)
	DeleteByID(ctx context.Context, id int64) (bool, error)
	DeleteByArticle(ctx context.Context, articleID string) (int64, error)
}
