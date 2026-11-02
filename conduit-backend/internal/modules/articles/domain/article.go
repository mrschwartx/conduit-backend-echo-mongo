package domain

import (
	"time"

	sharedomain "conduit/backend/internal/shared/domain"
)

type Article struct {
	id          string
	authorID    string
	slug        string
	title       ArticleTitle
	description ArticleDescription
	body        ArticleBody
	tagList     TagList
	createdAt   time.Time
	updatedAt   time.Time
}

type Snapshot struct {
	ID          string
	AuthorID    string
	Slug        string
	Title       string
	Description string
	Body        string
	TagList     []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type NewArticle struct {
	ID          string
	AuthorID    string
	Slug        string
	Title       ArticleTitle
	Description ArticleDescription
	Body        ArticleBody
	TagList     TagList
	Now         time.Time
}

type Revision struct {
	Slug        string
	Title       *ArticleTitle
	Description *ArticleDescription
	Body        *ArticleBody
	TagList     *TagList
}

func Create(params NewArticle) Article {
	timestamp := sharedomain.TruncateToMicrosecond(params.Now)
	return Article{
		id:          params.ID,
		authorID:    params.AuthorID,
		slug:        params.Slug,
		title:       params.Title,
		description: params.Description,
		body:        params.Body,
		tagList:     params.TagList,
		createdAt:   timestamp,
		updatedAt:   timestamp,
	}
}

func Restore(snapshot Snapshot) Article {
	return Article{
		id:          snapshot.ID,
		authorID:    snapshot.AuthorID,
		slug:        snapshot.Slug,
		title:       RestoreArticleTitle(snapshot.Title),
		description: RestoreArticleDescription(snapshot.Description),
		body:        RestoreArticleBody(snapshot.Body),
		tagList:     RestoreTagList(snapshot.TagList),
		createdAt:   snapshot.CreatedAt.UTC(),
		updatedAt:   snapshot.UpdatedAt.UTC(),
	}
}

func (a Article) ID() string                      { return a.id }
func (a Article) AuthorID() string                { return a.authorID }
func (a Article) Slug() string                    { return a.slug }
func (a Article) Title() ArticleTitle             { return a.title }
func (a Article) Description() ArticleDescription { return a.description }
func (a Article) Body() ArticleBody               { return a.body }
func (a Article) TagList() TagList                { return a.tagList }
func (a Article) CreatedAt() time.Time            { return a.createdAt }
func (a Article) UpdatedAt() time.Time            { return a.updatedAt }

func (a Article) IsAuthoredBy(userID string) bool {
	return a.authorID == userID
}

func (a *Article) Revise(revision Revision, now time.Time) {
	if revision.Slug != "" {
		a.slug = revision.Slug
	}
	if revision.Title != nil {
		a.title = *revision.Title
	}
	if revision.Description != nil {
		a.description = *revision.Description
	}
	if revision.Body != nil {
		a.body = *revision.Body
	}
	if revision.TagList != nil {
		a.tagList = *revision.TagList
	}
	a.updatedAt = sharedomain.TruncateToMicrosecond(now)
}

func (a Article) Snapshot() Snapshot {
	return Snapshot{
		ID:          a.id,
		AuthorID:    a.authorID,
		Slug:        a.slug,
		Title:       a.title.Value(),
		Description: a.description.Value(),
		Body:        a.body.Value(),
		TagList:     a.tagList.Values(),
		CreatedAt:   a.createdAt,
		UpdatedAt:   a.updatedAt,
	}
}
