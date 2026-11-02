package application

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"conduit/backend/internal/modules/comments/domain"
	sharedapp "conduit/backend/internal/shared/application"
	sharedomain "conduit/backend/internal/shared/domain"
)

type memoryComments struct {
	items []domain.Comment
	next  int64
}

func (r *memoryComments) Add(_ context.Context, comment domain.Comment) (domain.Comment, error) {
	r.next++
	snapshot := comment.Snapshot()
	snapshot.ID = r.next
	stored := domain.Restore(snapshot)
	r.items = append(r.items, stored)
	return stored, nil
}

func (r *memoryComments) FindByID(_ context.Context, id int64) (domain.Comment, bool, error) {
	for _, item := range r.items {
		if item.ID() == id {
			return item, true, nil
		}
	}
	return domain.Comment{}, false, nil
}

func (r *memoryComments) ListByArticle(_ context.Context, articleID string) ([]domain.Comment, error) {
	out := []domain.Comment{}
	for i := len(r.items) - 1; i >= 0; i-- {
		if r.items[i].ArticleID() == articleID {
			out = append(out, r.items[i])
		}
	}
	return out, nil
}

func (r *memoryComments) DeleteByID(_ context.Context, id int64) (bool, error) {
	i := slices.IndexFunc(r.items, func(c domain.Comment) bool { return c.ID() == id })
	if i < 0 {
		return false, nil
	}
	r.items = slices.Delete(r.items, i, i+1)
	return true, nil
}

func (r *memoryComments) DeleteByArticle(_ context.Context, articleID string) (int64, error) {
	before := len(r.items)
	r.items = slices.DeleteFunc(r.items, func(c domain.Comment) bool { return c.ArticleID() == articleID })
	return int64(before - len(r.items)), nil
}

type fakeCatalog map[string]string

func (c fakeCatalog) FindIDBySlug(_ context.Context, slug string) (string, bool, error) {
	id, ok := c[slug]
	return id, ok, nil
}

type fakeAuthors struct{}

func (fakeAuthors) FindProfiles(_ context.Context, ids []string, _ string) (map[string]sharedapp.AuthorProfile, error) {
	profiles := map[string]sharedapp.AuthorProfile{}
	for _, id := range ids {
		profiles[id] = sharedapp.AuthorProfile{Username: "name-of-" + id}
	}
	return profiles, nil
}

func (fakeAuthors) FindIDByUsername(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (fakeAuthors) ListFollowedIDs(context.Context, string) ([]string, error) { return nil, nil }

type fixture struct {
	comments *memoryComments
	create   *CreateComment
	list     *ListComments
	remove   *DeleteComment
	purge    *DeleteCommentsByArticle
}

func newFixture() fixture {
	comments := &memoryComments{}
	catalog := fakeCatalog{"first": "article-1", "second": "article-2"}
	return fixture{
		comments: comments,
		create:   NewCreateComment(comments, catalog, fakeAuthors{}),
		list:     NewListComments(comments, catalog, fakeAuthors{}),
		remove:   NewDeleteComment(comments, catalog),
		purge:    NewDeleteCommentsByArticle(comments),
	}
}

func expectFields(t *testing.T, err error, kind sharedomain.ErrorKind, want map[string][]string) {
	t.Helper()
	if !sharedomain.IsKind(err, kind) || !reflect.DeepEqual(sharedomain.FieldsOf(err), want) {
		t.Fatalf("got %v %v, want %s %v", err, sharedomain.FieldsOf(err), kind, want)
	}
}

func TestCommentBody(t *testing.T) {
	body, err := domain.NewCommentBody(sharedomain.RawString("  hi  "))
	if err != nil || body.Value() != "hi" {
		t.Fatalf("got %q %v", body.Value(), err)
	}
	_, err = domain.NewCommentBody(sharedomain.Raw{})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"body": {"can't be blank"}})
	_, err = domain.NewCommentBody(sharedomain.RawString(fmt.Sprintf("%01001d", 0)))
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"body": {"is too long (maximum is 1000 characters)"}})
}

func TestCreateAndListComments(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	first, err := f.create.Execute(ctx, "user-1", "first", sharedomain.RawString("Hello"))
	if err != nil || first.ID != 1 || first.Body != "Hello" || first.Author.Username != "name-of-user-1" || first.CreatedAt != first.UpdatedAt {
		t.Fatalf("got %+v %v", first, err)
	}
	f.create.Execute(ctx, "user-2", "first", sharedomain.RawString("Second"))
	f.create.Execute(ctx, "user-2", "second", sharedomain.RawString("Elsewhere"))

	comments, err := f.list.Execute(ctx, "first", "")
	if err != nil || len(comments) != 2 || comments[0].Body != "Second" || comments[1].Body != "Hello" {
		t.Fatalf("newest first: %+v %v", comments, err)
	}

	f.purge.Execute(ctx, "article-2")
	if comments, _ := f.list.Execute(ctx, "second", ""); comments == nil || len(comments) != 0 {
		t.Fatalf("an article without comments lists an empty slice: %v", comments)
	}
}

func TestCreateCommentFailures(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	_, err := f.create.Execute(ctx, "user-1", "missing", sharedomain.RawString(""))
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"article": {"not found"}})
	_, err = f.create.Execute(ctx, "user-1", "first", sharedomain.RawString("  "))
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"body": {"can't be blank"}})
	_, err = f.list.Execute(ctx, "missing", "")
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"article": {"not found"}})
}

func TestDeleteComment(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	f.create.Execute(ctx, "user-1", "first", sharedomain.RawString("Mine"))

	err := f.remove.Execute(ctx, "user-1", "missing", "1")
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"article": {"not found"}})
	err = f.remove.Execute(ctx, "user-1", "first", "abc")
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"id": {"must be an integer"}})
	err = f.remove.Execute(ctx, "user-1", "first", "-1")
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"id": {"must be an integer"}})
	err = f.remove.Execute(ctx, "user-1", "first", "9999999999999999999")
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"comment": {"not found"}})
	err = f.remove.Execute(ctx, "user-1", "first", "42")
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"comment": {"not found"}})
	err = f.remove.Execute(ctx, "user-1", "second", "1")
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"comment": {"not found"}})
	err = f.remove.Execute(ctx, "user-2", "first", "1")
	expectFields(t, err, sharedomain.KindForbidden, map[string][]string{"comment": {"forbidden"}})

	if err := f.remove.Execute(ctx, "user-1", "first", "1"); err != nil {
		t.Fatal(err)
	}
	if len(f.comments.items) != 0 {
		t.Fatal("the comment must be gone")
	}
}

func TestDeleteCommentsByArticle(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	f.create.Execute(ctx, "user-1", "first", sharedomain.RawString("a"))
	f.create.Execute(ctx, "user-1", "first", sharedomain.RawString("b"))
	f.create.Execute(ctx, "user-1", "second", sharedomain.RawString("c"))

	if count, err := f.purge.Execute(ctx, "article-1"); err != nil || count != 2 {
		t.Fatalf("got %d %v", count, err)
	}
	if len(f.comments.items) != 1 {
		t.Fatal("other articles keep their comments")
	}
}
