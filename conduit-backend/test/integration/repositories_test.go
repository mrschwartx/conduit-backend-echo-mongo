//go:build integration

package integration

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	articlesdomain "conduit/backend/internal/modules/articles/domain"
	articlesinfra "conduit/backend/internal/modules/articles/infrastructure"
	commentsdomain "conduit/backend/internal/modules/comments/domain"
	commentsinfra "conduit/backend/internal/modules/comments/infrastructure"
	usersdomain "conduit/backend/internal/modules/users/domain"
	usersinfra "conduit/backend/internal/modules/users/infrastructure"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/postgres"
	"conduit/backend/test/support"
)

func connect(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

func newPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx := connect(t)
	database := support.StartTestDatabase(t)
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{URL: database.URL, PoolMax: 4, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil); err != nil {
		t.Fatal(err)
	}
	return ctx, pool
}

func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := connect(t)
	database := support.StartTestDatabase(t)
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{URL: database.URL, PoolMax: 4, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	first, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, len(first), 4)
	expectEqual(t, len(second), 0)
}

func TestMigrationsSurviveConcurrentStarts(t *testing.T) {
	ctx := connect(t)
	database := support.StartTestDatabase(t)
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{URL: database.URL, PoolMax: 8, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	results := make(chan error, 4)
	for range 4 {
		go func() {
			_, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil)
			results <- err
		}()
	}
	for range 4 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

func buildUser(t *testing.T, repo *usersinfra.PostgresRepository, username, email string, now time.Time) usersdomain.User {
	t.Helper()
	name, err := usersdomain.NewUsername(sharedomain.RawString(username))
	if err != nil {
		t.Fatal(err)
	}
	address, err := usersdomain.NewEmail(sharedomain.RawString(email))
	if err != nil {
		t.Fatal(err)
	}
	return usersdomain.Register(usersdomain.Registration{ID: repo.NextID(), Username: name, Email: address, PasswordHash: "hash", Now: now})
}

func TestUserRepository(t *testing.T) {
	ctx, pool := newPool(t)
	users := usersinfra.NewPostgresRepository(pool)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	user := buildUser(t, users, "JohnDoe", "John@Mail.com", base)
	if err := users.Add(ctx, user); err != nil {
		t.Fatal(err)
	}

	t.Run("round trips a user", func(t *testing.T) {
		found, ok, err := users.FindByID(ctx, user.ID())
		if err != nil || !ok {
			t.Fatalf("find failed: %v %v", ok, err)
		}
		expectDeep(t, found.Snapshot(), user.Snapshot())
	})

	t.Run("finds by email and username regardless of case", func(t *testing.T) {
		if _, ok, err := users.FindByEmail(ctx, "JOHN@mail.COM"); err != nil || !ok {
			t.Fatalf("email lookup failed: %v %v", ok, err)
		}
		if _, ok, err := users.FindByUsername(ctx, "johndoe"); err != nil || !ok {
			t.Fatalf("username lookup failed: %v %v", ok, err)
		}
		if _, ok, _ := users.FindByUsername(ctx, "nobody"); ok {
			t.Fatal("unknown usernames are not found")
		}
	})

	t.Run("reports conflicts through the domain error", func(t *testing.T) {
		duplicate := buildUser(t, users, "johndoe", "other@mail.com", base)
		err := users.Add(ctx, duplicate)
		if !sharedomain.IsKind(err, sharedomain.KindConflict) {
			t.Fatalf("expected a conflict, got %v", err)
		}
		expectDeep(t, sharedomain.FieldsOf(err), map[string][]string{"username": {"has already been taken"}})

		err = users.Add(ctx, buildUser(t, users, "someone", "JOHN@mail.com", base))
		expectDeep(t, sharedomain.FieldsOf(err), map[string][]string{"email": {"has already been taken"}})
	})

	t.Run("finds taken fields and honours the exclusion", func(t *testing.T) {
		taken, err := users.FindTakenFields(ctx, usersdomain.TakenQuery{Email: "john@mail.com", Username: "JOHNDOE"})
		if err != nil {
			t.Fatal(err)
		}
		expectDeep(t, taken, []string{"email", "username"})

		taken, _ = users.FindTakenFields(ctx, usersdomain.TakenQuery{Email: "john@mail.com", Username: "johndoe", ExcludeID: user.ID()})
		expectEqual(t, len(taken), 0)

		taken, _ = users.FindTakenFields(ctx, usersdomain.TakenQuery{Email: "free@mail.com"})
		expectEqual(t, len(taken), 0)

		taken, _ = users.FindTakenFields(ctx, usersdomain.TakenQuery{Username: "johndoe"})
		expectDeep(t, taken, []string{"username"})
	})

	t.Run("updates profile fields", func(t *testing.T) {
		bio, _ := usersdomain.NewBio(sharedomain.RawString("Writer"))
		image, _ := usersdomain.NewProfileImage(sharedomain.RawString("https://example.com/a.png"))
		current, _, _ := users.FindByID(ctx, user.ID())
		current.UpdateProfile(usersdomain.ProfileChanges{SetBio: true, Bio: bio, SetImage: true, Image: image}, base.Add(time.Hour))
		updated, err := users.Update(ctx, current)
		if err != nil || !updated {
			t.Fatalf("update failed: %v %v", updated, err)
		}
		found, _, _ := users.FindByID(ctx, user.ID())
		expectDeep(t, found.Snapshot(), current.Snapshot())

		current.UpdateProfile(usersdomain.ProfileChanges{SetBio: true, SetImage: true}, base.Add(2*time.Hour))
		users.Update(ctx, current)
		found, _, _ = users.FindByID(ctx, user.ID())
		if found.Bio() != nil || found.Image() != nil {
			t.Fatal("clearing must store NULL")
		}
	})

	t.Run("update reports missing rows and conflicts", func(t *testing.T) {
		ghost := buildUser(t, users, "ghost", "ghost@mail.com", base)
		updated, err := users.Update(ctx, ghost)
		if err != nil || updated {
			t.Fatalf("got %v %v", updated, err)
		}

		second := buildUser(t, users, "second", "second@mail.com", base)
		users.Add(ctx, second)
		name, _ := usersdomain.NewUsername(sharedomain.RawString("JohnDoe"))
		second.UpdateProfile(usersdomain.ProfileChanges{Username: &name}, base)
		_, err = users.Update(ctx, second)
		if !sharedomain.IsKind(err, sharedomain.KindConflict) {
			t.Fatalf("expected a conflict, got %v", err)
		}
	})

	t.Run("looks users up in bulk and checks existence", func(t *testing.T) {
		found, err := users.FindByIDs(ctx, []string{user.ID(), "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"})
		if err != nil || len(found) != 1 {
			t.Fatalf("got %d %v", len(found), err)
		}
		if exists, _ := users.ExistsByID(ctx, user.ID()); !exists {
			t.Fatal("the user exists")
		}
		if exists, _ := users.ExistsByID(ctx, "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"); exists {
			t.Fatal("unknown ids do not exist")
		}
	})
}

func TestFollowRepository(t *testing.T) {
	ctx, pool := newPool(t)
	users := usersinfra.NewPostgresRepository(pool)
	follows := usersinfra.NewPostgresFollowRepository(pool)
	now := time.Now()
	a := buildUser(t, users, "alpha", "a@mail.com", now)
	b := buildUser(t, users, "bravo", "b@mail.com", now)
	c := buildUser(t, users, "charlie", "c@mail.com", now)
	for _, u := range []usersdomain.User{a, b, c} {
		if err := users.Add(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	for range 2 {
		if err := follows.Follow(ctx, a.ID(), b.ID()); err != nil {
			t.Fatalf("follow must be idempotent: %v", err)
		}
	}
	follows.Follow(ctx, a.ID(), c.ID())

	if ok, _ := follows.IsFollowing(ctx, a.ID(), b.ID()); !ok {
		t.Fatal("alpha follows bravo")
	}
	if ok, _ := follows.IsFollowing(ctx, b.ID(), a.ID()); ok {
		t.Fatal("follows are directional")
	}

	among, err := follows.FindFollowedAmong(ctx, a.ID(), []string{b.ID(), a.ID()})
	if err != nil {
		t.Fatal(err)
	}
	expectDeep(t, among, map[string]bool{b.ID(): true})

	ids, _ := follows.ListFollowedIDs(ctx, a.ID())
	expectEqual(t, len(ids), 2)

	for range 2 {
		if err := follows.Unfollow(ctx, a.ID(), b.ID()); err != nil {
			t.Fatalf("unfollow must be idempotent: %v", err)
		}
	}
	ids, _ = follows.ListFollowedIDs(ctx, a.ID())
	expectDeep(t, ids, []string{c.ID()})

	if err := follows.Follow(ctx, a.ID(), a.ID()); err == nil {
		t.Fatal("the database must refuse self follows")
	}
}

func rawTags(values ...string) sharedomain.Raw {
	entries := make([]any, 0, len(values))
	for _, value := range values {
		entries = append(entries, value)
	}
	return sharedomain.RawOf(entries)
}

func buildArticle(t *testing.T, repo *articlesinfra.PostgresRepository, authorID, slug, title string, tags []string, now time.Time) articlesdomain.Article {
	t.Helper()
	articleTitle, _ := articlesdomain.NewArticleTitle(sharedomain.RawString(title))
	description, _ := articlesdomain.NewArticleDescription(sharedomain.RawString("About " + title))
	body, _ := articlesdomain.NewArticleBody(sharedomain.RawString("Body of " + title))
	tagList, err := articlesdomain.NewTagList(rawTags(tags...))
	if err != nil {
		t.Fatal(err)
	}
	return articlesdomain.Create(articlesdomain.NewArticle{
		ID: repo.NextID(), AuthorID: authorID, Slug: slug, Title: articleTitle,
		Description: description, Body: body, TagList: tagList, Now: now,
	})
}

func TestArticleRepository(t *testing.T) {
	ctx, pool := newPool(t)
	users := usersinfra.NewPostgresRepository(pool)
	articles := articlesinfra.NewPostgresRepository(pool)
	now := time.Now()
	author := buildUser(t, users, "author", "author@mail.com", now)
	reader := buildUser(t, users, "reader", "reader@mail.com", now)
	users.Add(ctx, author)
	users.Add(ctx, reader)

	first := buildArticle(t, articles, author.ID(), "first", "First", []string{"go", "api", "web"}, now)
	second := buildArticle(t, articles, reader.ID(), "second", "Second", []string{"go"}, now.Add(time.Second))
	third := buildArticle(t, articles, author.ID(), "third", "Third", nil, now.Add(2*time.Second))
	for _, article := range []articlesdomain.Article{first, second, third} {
		if err := articles.Add(ctx, article); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("round trips an article keeping tag order", func(t *testing.T) {
		found, ok, err := articles.FindBySlug(ctx, "first")
		if err != nil || !ok {
			t.Fatalf("find failed: %v %v", ok, err)
		}
		expectDeep(t, found.Snapshot(), first.Snapshot())
		expectDeep(t, found.TagList().Values(), []string{"go", "api", "web"})

		bare, _, _ := articles.FindBySlug(ctx, "third")
		if bare.TagList().Values() == nil || len(bare.TagList().Values()) != 0 {
			t.Fatal("articles without tags have an empty list")
		}
		if _, ok, _ := articles.FindBySlug(ctx, "missing"); ok {
			t.Fatal("unknown slugs are not found")
		}
	})

	t.Run("rejects duplicate slugs with a sentinel", func(t *testing.T) {
		clash := buildArticle(t, articles, author.ID(), "first", "Other", nil, now)
		if err := articles.Add(ctx, clash); !errors.Is(err, articlesdomain.ErrSlugTaken) {
			t.Fatalf("expected ErrSlugTaken, got %v", err)
		}
		if taken, _ := articles.SlugTaken(ctx, "first", ""); !taken {
			t.Fatal("the slug is taken")
		}
		if taken, _ := articles.SlugTaken(ctx, "first", first.ID()); taken {
			t.Fatal("an article never collides with itself")
		}
		if taken, _ := articles.SlugTaken(ctx, "free", ""); taken {
			t.Fatal("free slugs are free")
		}
	})

	t.Run("a failed add leaves no orphaned tags", func(t *testing.T) {
		clash := buildArticle(t, articles, author.ID(), "first", "Other", []string{"orphan"}, now)
		articles.Add(ctx, clash)
		tags, _ := articles.PopularTags(ctx, 50)
		for _, tag := range tags {
			if tag == "orphan" {
				t.Fatal("the transaction must roll back")
			}
		}
	})

	t.Run("builds views with favorites", func(t *testing.T) {
		if err := articles.Favorite(ctx, first.ID(), reader.ID()); err != nil {
			t.Fatal(err)
		}
		if err := articles.Favorite(ctx, first.ID(), reader.ID()); err != nil {
			t.Fatalf("favoriting twice is idempotent: %v", err)
		}
		articles.Favorite(ctx, first.ID(), author.ID())

		view, ok, err := articles.FindViewBySlug(ctx, "first", reader.ID())
		if err != nil || !ok {
			t.Fatalf("view failed: %v %v", ok, err)
		}
		expectEqual(t, view.FavoritesCount, 2)
		expectEqual(t, view.Favorited, true)
		expectEqual(t, view.Body, "Body of First")

		anonymous, _, _ := articles.FindViewBySlug(ctx, "first", "")
		expectEqual(t, anonymous.Favorited, false)
		expectEqual(t, anonymous.FavoritesCount, 2)

		articles.Unfavorite(ctx, first.ID(), reader.ID())
		articles.Unfavorite(ctx, first.ID(), reader.ID())
		view, _, _ = articles.FindViewBySlug(ctx, "first", reader.ID())
		expectEqual(t, view.FavoritesCount, 1)
		expectEqual(t, view.Favorited, false)
		articles.Favorite(ctx, first.ID(), reader.ID())
	})

	slugs := func(page articlesdomain.Page) []string {
		out := []string{}
		for _, item := range page.Items {
			out = append(out, item.Slug)
		}
		return out
	}

	t.Run("pages newest first and filters", func(t *testing.T) {
		page, err := articles.FindPage(ctx, articlesdomain.PageQuery{Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		expectDeep(t, slugs(page), []string{"third", "second", "first"})
		expectEqual(t, page.Total, 3)

		page, _ = articles.FindPage(ctx, articlesdomain.PageQuery{Limit: 1, Offset: 1})
		expectDeep(t, slugs(page), []string{"second"})
		expectEqual(t, page.Total, 3)

		tag := "go"
		page, _ = articles.FindPage(ctx, articlesdomain.PageQuery{Tag: &tag, Limit: 20})
		expectDeep(t, slugs(page), []string{"second", "first"})

		empty := ""
		page, _ = articles.FindPage(ctx, articlesdomain.PageQuery{Tag: &empty, Limit: 20})
		expectEqual(t, page.Total, 0)
		if page.Items == nil {
			t.Fatal("items must never be nil")
		}

		page, _ = articles.FindPage(ctx, articlesdomain.PageQuery{AuthorID: author.ID(), Limit: 20})
		expectDeep(t, slugs(page), []string{"third", "first"})

		page, _ = articles.FindPage(ctx, articlesdomain.PageQuery{AuthorIDs: []string{reader.ID()}, Limit: 20})
		expectDeep(t, slugs(page), []string{"second"})

		page, _ = articles.FindPage(ctx, articlesdomain.PageQuery{FavoritedByID: reader.ID(), Limit: 20})
		expectDeep(t, slugs(page), []string{"first"})

		page, _ = articles.FindPage(ctx, articlesdomain.PageQuery{Tag: &tag, AuthorID: author.ID(), ViewerID: reader.ID(), Limit: 20})
		expectDeep(t, slugs(page), []string{"first"})
		expectEqual(t, page.Items[0].Favorited, true)
	})

	t.Run("updates an article and replaces its tags", func(t *testing.T) {
		current, _, _ := articles.FindBySlug(ctx, "second")
		title, _ := articlesdomain.NewArticleTitle(sharedomain.RawString("Second Renamed"))
		tags, _ := articlesdomain.NewTagList(rawTags("fresh", "tags"))
		current.Revise(articlesdomain.Revision{Slug: "second-renamed", Title: &title, TagList: &tags}, now.Add(time.Hour))
		updated, err := articles.Update(ctx, current)
		if err != nil || !updated {
			t.Fatalf("update failed: %v %v", updated, err)
		}
		found, _, _ := articles.FindBySlug(ctx, "second-renamed")
		expectDeep(t, found.Snapshot(), current.Snapshot())

		clash, _, _ := articles.FindBySlug(ctx, "second-renamed")
		clash.Revise(articlesdomain.Revision{Slug: "first"}, now)
		if _, err := articles.Update(ctx, clash); !errors.Is(err, articlesdomain.ErrSlugTaken) {
			t.Fatalf("expected ErrSlugTaken, got %v", err)
		}

		ghost := buildArticle(t, articles, author.ID(), "ghost", "Ghost", []string{"x"}, now)
		if updated, err := articles.Update(ctx, ghost); err != nil || updated {
			t.Fatalf("got %v %v", updated, err)
		}
	})

	t.Run("lists popular tags by usage then name", func(t *testing.T) {
		fourth := buildArticle(t, articles, author.ID(), "fourth", "Fourth", []string{"go", "tags"}, now.Add(3*time.Second))
		if err := articles.Add(ctx, fourth); err != nil {
			t.Fatal(err)
		}
		tags, err := articles.PopularTags(ctx, 3)
		if err != nil {
			t.Fatal(err)
		}
		expectDeep(t, tags, []string{"go", "tags", "api"})
		limited, _ := articles.PopularTags(ctx, 1)
		expectDeep(t, limited, []string{"go"})
	})

	t.Run("deletes an article with its tags and favorites", func(t *testing.T) {
		deleted, err := articles.DeleteByID(ctx, first.ID())
		if err != nil || !deleted {
			t.Fatalf("delete failed: %v %v", deleted, err)
		}
		if _, ok, _ := articles.FindBySlug(ctx, "first"); ok {
			t.Fatal("the article is gone")
		}
		var leftovers int
		pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM article_tags WHERE article_id = $1) + (SELECT count(*) FROM favorites WHERE article_id = $1)", first.ID()).Scan(&leftovers)
		expectEqual(t, leftovers, 0)
		if deleted, _ := articles.DeleteByID(ctx, first.ID()); deleted {
			t.Fatal("deleting twice reports false")
		}
	})
}

func TestCommentRepository(t *testing.T) {
	ctx, pool := newPool(t)
	users := usersinfra.NewPostgresRepository(pool)
	articles := articlesinfra.NewPostgresRepository(pool)
	comments := commentsinfra.NewPostgresRepository(pool)
	now := time.Now()

	author := buildUser(t, users, "author", "author@mail.com", now)
	users.Add(ctx, author)
	article := buildArticle(t, articles, author.ID(), "post", "Post", nil, now)
	other := buildArticle(t, articles, author.ID(), "other", "Other", nil, now)
	articles.Add(ctx, article)
	articles.Add(ctx, other)

	add := func(articleID, text string, at time.Time) commentsdomain.Comment {
		body, _ := commentsdomain.NewCommentBody(sharedomain.RawString(text))
		stored, err := comments.Add(ctx, commentsdomain.Create(commentsdomain.NewComment{ArticleID: articleID, AuthorID: author.ID(), Body: body, Now: at}))
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}

	first := add(article.ID(), "one", now)
	second := add(article.ID(), "two", now.Add(time.Second))
	add(other.ID(), "elsewhere", now)

	if first.ID() == 0 || second.ID() <= first.ID() {
		t.Fatalf("ids must be generated and increasing: %d %d", first.ID(), second.ID())
	}

	found, ok, err := comments.FindByID(ctx, first.ID())
	if err != nil || !ok {
		t.Fatalf("find failed: %v %v", ok, err)
	}
	expectDeep(t, found.Snapshot(), first.Snapshot())
	if _, ok, _ := comments.FindByID(ctx, 99999); ok {
		t.Fatal("unknown ids are not found")
	}

	listed, err := comments.ListByArticle(ctx, article.ID())
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, comment := range listed {
		got = append(got, comment.Body().Value())
	}
	if !reflect.DeepEqual(got, []string{"two", "one"}) {
		t.Fatalf("newest first expected, got %v", got)
	}

	if deleted, _ := comments.DeleteByID(ctx, first.ID()); !deleted {
		t.Fatal("delete must report true")
	}
	if deleted, _ := comments.DeleteByID(ctx, first.ID()); deleted {
		t.Fatal("deleting twice reports false")
	}

	count, err := comments.DeleteByArticle(ctx, article.ID())
	if err != nil || count != 1 {
		t.Fatalf("got %d %v", count, err)
	}

	add(other.ID(), "cascade me", now)
	if _, err := articles.DeleteByID(ctx, other.ID()); err != nil {
		t.Fatal(err)
	}
	remaining, _ := comments.ListByArticle(ctx, other.ID())
	expectEqual(t, len(remaining), 0)
}
