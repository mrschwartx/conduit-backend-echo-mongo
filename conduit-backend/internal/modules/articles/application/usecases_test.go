package application

import (
	"context"
	"reflect"
	"testing"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type fixture struct {
	articles *memoryArticles
	authors  *fakeAuthors
	bus      *recordingBus
	create   *CreateArticle
	get      *GetArticle
	list     *ListArticles
	feed     *GetFeed
	update   *UpdateArticle
	remove   *DeleteArticle
	favorite *SetFavorite
	unfav    *SetFavorite
	tags     *ListPopularTags
}

func newFixture() fixture {
	articles := newMemoryArticles()
	authors := newFakeAuthors()
	bus := &recordingBus{}
	slugs := NewArticleSlugs(articles)
	return fixture{
		articles: articles, authors: authors, bus: bus,
		create:   NewCreateArticle(articles, slugs, authors),
		get:      NewGetArticle(articles, authors),
		list:     NewListArticles(articles, authors),
		feed:     NewGetFeed(articles, authors),
		update:   NewUpdateArticle(articles, slugs, authors),
		remove:   NewDeleteArticle(articles, bus),
		favorite: NewSetFavorite(articles, authors, true),
		unfav:    NewSetFavorite(articles, authors, false),
		tags:     NewListPopularTags(articles),
	}
}

func str(value string) sharedomain.Raw { return sharedomain.RawString(value) }

func input(title string, tagList ...any) ArticleInput {
	return ArticleInput{Title: str(title), Description: str("About " + title), Body: str("Body of " + title), TagList: sharedomain.RawOf(tagList)}
}

func (f fixture) publish(t *testing.T, authorID, title string, tagList ...any) ArticleDTO {
	t.Helper()
	dto, err := f.create.Execute(context.Background(), authorID, input(title, tagList...))
	if err != nil {
		t.Fatal(err)
	}
	return dto
}

func expectFields(t *testing.T, err error, kind sharedomain.ErrorKind, want map[string][]string) {
	t.Helper()
	if !sharedomain.IsKind(err, kind) || !reflect.DeepEqual(sharedomain.FieldsOf(err), want) {
		t.Fatalf("got %v %v, want %s %v", err, sharedomain.FieldsOf(err), kind, want)
	}
}

func TestCreateArticle(t *testing.T) {
	f := newFixture()
	dto := f.publish(t, "user-1", "Hello World", "go", "api")
	if dto.Slug != "hello-world" || dto.Title != "Hello World" || dto.Body != "Body of Hello World" ||
		!reflect.DeepEqual(dto.TagList, []string{"go", "api"}) || dto.Author.Username != "jane" ||
		dto.Favorited || dto.FavoritesCount != 0 || dto.CreatedAt == "" || dto.CreatedAt != dto.UpdatedAt {
		t.Fatalf("unexpected dto %+v", dto)
	}
}

func TestCreateArticleValidatesEveryField(t *testing.T) {
	f := newFixture()
	_, err := f.create.Execute(context.Background(), "user-1", ArticleInput{TagList: sharedomain.RawString("x")})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{
		"title": {"can't be blank"}, "description": {"can't be blank"}, "body": {"can't be blank"}, "tagList": {"is invalid"},
	})
}

func TestCreateArticleAllocatesUniqueSlugs(t *testing.T) {
	f := newFixture()
	if got := f.publish(t, "user-1", "Same Title").Slug; got != "same-title" {
		t.Fatal(got)
	}
	if got := f.publish(t, "user-2", "Same Title").Slug; got != "same-title-2" {
		t.Fatal(got)
	}
	if got := f.publish(t, "user-2", "Same title!").Slug; got != "same-title-3" {
		t.Fatal(got)
	}
	if got := f.publish(t, "user-1", "Feed").Slug; got != "feed-2" {
		t.Fatalf("reserved slugs are skipped: %s", got)
	}
}

func TestCreateArticleRetriesOnSlugRaces(t *testing.T) {
	f := newFixture()
	f.articles.slugClash = 2
	if dto := f.publish(t, "user-1", "Racy"); dto.Slug != "racy" {
		t.Fatalf("got %s", dto.Slug)
	}
	f.articles.slugClash = 10
	_, err := f.create.Execute(context.Background(), "user-1", input("Doomed"))
	if err != domain.ErrSlugTaken {
		t.Fatalf("gives up after %d attempts: %v", maxSlugAttempts, err)
	}
}

func TestGetArticle(t *testing.T) {
	f := newFixture()
	f.publish(t, "user-1", "Hello")
	f.articles.Favorite(context.Background(), "article-1", "user-2")
	f.authors.followed["user-2"] = []string{"user-1"}

	dto, err := f.get.Execute(context.Background(), "hello", "user-2")
	if err != nil || !dto.Favorited || dto.FavoritesCount != 1 || !dto.Author.Following {
		t.Fatalf("got %+v %v", dto, err)
	}
	dto, _ = f.get.Execute(context.Background(), "hello", "")
	if dto.Favorited || dto.Author.Following {
		t.Fatalf("anonymous viewers get neutral flags: %+v", dto)
	}
	_, err = f.get.Execute(context.Background(), "missing", "")
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"article": {"not found"}})
}

func TestListArticlesFilters(t *testing.T) {
	f := newFixture()
	f.publish(t, "user-1", "First", "go", "api")
	f.publish(t, "user-2", "Second", "go")
	f.publish(t, "user-1", "Third", "web")
	f.articles.Favorite(context.Background(), "article-2", "user-1")
	ctx := context.Background()

	titles := func(page ArticlePageDTO) []string {
		out := []string{}
		for _, a := range page.Articles {
			out = append(out, a.Title)
		}
		return out
	}

	page, err := f.list.Execute(ctx, ListArticlesInput{})
	if err != nil || page.ArticlesCount != 3 || !reflect.DeepEqual(titles(page), []string{"Third", "Second", "First"}) {
		t.Fatalf("newest first: %v %v", titles(page), err)
	}
	page, _ = f.list.Execute(ctx, ListArticlesInput{Tag: []string{"go"}})
	if !reflect.DeepEqual(titles(page), []string{"Second", "First"}) {
		t.Fatalf("tag: %v", titles(page))
	}
	page, _ = f.list.Execute(ctx, ListArticlesInput{Tag: []string{""}})
	if page.ArticlesCount != 0 || page.Articles == nil {
		t.Fatalf("an empty tag matches nothing: %+v", page)
	}
	page, _ = f.list.Execute(ctx, ListArticlesInput{Author: []string{"jane"}})
	if !reflect.DeepEqual(titles(page), []string{"Third", "First"}) {
		t.Fatalf("author: %v", titles(page))
	}
	page, _ = f.list.Execute(ctx, ListArticlesInput{Favorited: []string{"jane"}})
	if !reflect.DeepEqual(titles(page), []string{"Second"}) {
		t.Fatalf("favorited: %v", titles(page))
	}
	page, _ = f.list.Execute(ctx, ListArticlesInput{Author: []string{"ghost"}})
	if page.ArticlesCount != 0 || page.Articles == nil || len(page.Articles) != 0 {
		t.Fatalf("unknown authors yield an empty page: %+v", page)
	}
	page, _ = f.list.Execute(ctx, ListArticlesInput{Favorited: []string{"ghost"}})
	if page.ArticlesCount != 0 {
		t.Fatalf("unknown favoriters yield an empty page: %+v", page)
	}
	page, _ = f.list.Execute(ctx, ListArticlesInput{Offset: []string{"1"}, Limit: []string{"1"}})
	if page.ArticlesCount != 3 || !reflect.DeepEqual(titles(page), []string{"Second"}) {
		t.Fatalf("paging: %v %d", titles(page), page.ArticlesCount)
	}
}

func TestListArticlesValidation(t *testing.T) {
	f := newFixture()
	_, err := f.list.Execute(context.Background(), ListArticlesInput{
		Offset: []string{"x"}, Limit: []string{"0"}, Tag: []string{"a", "b"}, Author: []string{"a", "b"}, Favorited: []string{"a", "b"},
	})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{
		"offset": {"must be an integer"}, "limit": {"must be at least 1"},
		"tag": {"is invalid"}, "author": {"is invalid"}, "favorited": {"is invalid"},
	})
}

func TestGetFeed(t *testing.T) {
	f := newFixture()
	f.publish(t, "user-1", "From Jane")
	f.publish(t, "user-2", "From John")
	ctx := context.Background()

	page, err := f.feed.Execute(ctx, "user-3", nil, nil)
	if err != nil || page.ArticlesCount != 0 || page.Articles == nil {
		t.Fatalf("no follows means an empty feed: %+v %v", page, err)
	}
	f.authors.followed["user-3"] = []string{"user-2"}
	page, _ = f.feed.Execute(ctx, "user-3", nil, nil)
	if page.ArticlesCount != 1 || page.Articles[0].Title != "From John" || !page.Articles[0].Author.Following {
		t.Fatalf("unexpected feed %+v", page)
	}
	_, err = f.feed.Execute(ctx, "user-3", []string{"-3"}, nil)
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"offset": {"must be an integer"}})
}

func TestUpdateArticle(t *testing.T) {
	f := newFixture()
	f.publish(t, "user-1", "Original", "a")
	f.publish(t, "user-2", "Taken")
	ctx := context.Background()

	dto, err := f.update.Execute(ctx, "user-1", "original", ArticleInput{Body: str("New body")})
	if err != nil || dto.Slug != "original" || dto.Body != "New body" || dto.Title != "Original" || !reflect.DeepEqual(dto.TagList, []string{"a"}) {
		t.Fatalf("partial updates keep the rest: %+v %v", dto, err)
	}

	dto, err = f.update.Execute(ctx, "user-1", "original", ArticleInput{Title: str("Brand New"), TagList: sharedomain.RawOf([]any{})})
	if err != nil || dto.Slug != "brand-new" || len(dto.TagList) != 0 {
		t.Fatalf("a new title re-slugs: %+v %v", dto, err)
	}

	dto, _ = f.update.Execute(ctx, "user-1", "brand-new", ArticleInput{Title: str("Taken")})
	if dto.Slug != "taken-2" {
		t.Fatalf("re-slugging avoids collisions: %s", dto.Slug)
	}

	dto, _ = f.update.Execute(ctx, "user-1", "taken-2", ArticleInput{Title: str("Taken")})
	if dto.Slug != "taken-2" {
		t.Fatalf("an unchanged title keeps the slug: %s", dto.Slug)
	}

	dto, err = f.update.Execute(ctx, "user-1", "taken-2", ArticleInput{})
	if err != nil || dto.Slug != "taken-2" {
		t.Fatalf("an empty update is a no-op: %+v %v", dto, err)
	}
}

func TestUpdateArticleChecksInOrder(t *testing.T) {
	f := newFixture()
	f.publish(t, "user-1", "Mine")
	ctx := context.Background()

	_, err := f.update.Execute(ctx, "user-1", "missing", ArticleInput{Title: str("")})
	expectFields(t, err, sharedomain.KindNotFound, map[string][]string{"article": {"not found"}})
	_, err = f.update.Execute(ctx, "user-2", "mine", ArticleInput{Title: str("")})
	expectFields(t, err, sharedomain.KindForbidden, map[string][]string{"article": {"forbidden"}})
	_, err = f.update.Execute(ctx, "user-1", "mine", ArticleInput{Title: str(""), TagList: sharedomain.RawString("x")})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"title": {"can't be blank"}, "tagList": {"is invalid"}})
}

func TestDeleteArticle(t *testing.T) {
	f := newFixture()
	f.publish(t, "user-1", "Doomed")
	ctx := context.Background()

	if err := f.remove.Execute(ctx, "user-2", "doomed"); !sharedomain.IsKind(err, sharedomain.KindForbidden) {
		t.Fatalf("got %v", err)
	}
	if err := f.remove.Execute(ctx, "user-1", "missing"); !sharedomain.IsKind(err, sharedomain.KindNotFound) {
		t.Fatalf("got %v", err)
	}
	if len(f.bus.events) != 0 {
		t.Fatal("failed deletes publish nothing")
	}
	if err := f.remove.Execute(ctx, "user-1", "doomed"); err != nil {
		t.Fatal(err)
	}
	if len(f.articles.articles) != 0 || len(f.bus.events) != 1 {
		t.Fatalf("expected one event, got %+v", f.bus.events)
	}
	event := f.bus.events[0]
	if event.Name != sharedomain.ArticleDeletedEvent || event.Payload.(sharedomain.ArticleDeleted).ArticleID != "article-1" {
		t.Fatalf("unexpected event %+v", event)
	}
}

func TestFavorites(t *testing.T) {
	f := newFixture()
	f.publish(t, "user-1", "Liked")
	ctx := context.Background()

	dto, err := f.favorite.Execute(ctx, "user-2", "liked")
	if err != nil || !dto.Favorited || dto.FavoritesCount != 1 {
		t.Fatalf("got %+v %v", dto, err)
	}
	dto, _ = f.favorite.Execute(ctx, "user-2", "liked")
	if dto.FavoritesCount != 1 {
		t.Fatalf("favoriting twice is idempotent: %+v", dto)
	}
	dto, _ = f.favorite.Execute(ctx, "user-1", "liked")
	if dto.FavoritesCount != 2 {
		t.Fatalf("got %+v", dto)
	}
	dto, err = f.unfav.Execute(ctx, "user-2", "liked")
	if err != nil || dto.Favorited || dto.FavoritesCount != 1 {
		t.Fatalf("got %+v %v", dto, err)
	}
	dto, _ = f.unfav.Execute(ctx, "user-2", "liked")
	if dto.FavoritesCount != 1 {
		t.Fatalf("unfavoriting twice is idempotent: %+v", dto)
	}
	if _, err := f.favorite.Execute(ctx, "user-2", "missing"); !sharedomain.IsKind(err, sharedomain.KindNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, err := f.unfav.Execute(ctx, "user-2", "missing"); !sharedomain.IsKind(err, sharedomain.KindNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestPopularTagsAndCatalog(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	if tags, err := f.tags.Execute(ctx); err != nil || tags == nil || len(tags) != 0 {
		t.Fatalf("no tags yields an empty list: %v %v", tags, err)
	}
	f.publish(t, "user-1", "One", "go", "web")
	f.publish(t, "user-1", "Two", "go", "api")
	if tags, _ := f.tags.Execute(ctx); !reflect.DeepEqual(tags, []string{"go", "api", "web"}) {
		t.Fatalf("got %v", tags)
	}

	catalog := NewArticleCatalog(f.articles)
	if id, ok, _ := catalog.FindIDBySlug(ctx, "one"); !ok || id != "article-1" {
		t.Fatalf("got %q %v", id, ok)
	}
	if _, ok, _ := catalog.FindIDBySlug(ctx, "missing"); ok {
		t.Fatal("unknown slugs are not found")
	}
}

var _ domain.Repository = (*memoryArticles)(nil)
