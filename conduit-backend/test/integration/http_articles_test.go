//go:build integration

package integration

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"conduit/backend/test/support"
)

func TestCreateArticle(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)

	t.Run("requires authentication", func(t *testing.T) {
		expectProblem(t, author.client.Post("/api/v1/articles", support.RequestOptions{JSON: support.ArticlePayload(nil)}), 401, "token", "is missing")
	})

	t.Run("creates the article", func(t *testing.T) {
		response := publish(t, author, nil)
		expectEqual(t, response.String("article", "slug"), "how-to-train-your-dragon")
		expectEqual(t, response.String("article", "title"), "How to train your dragon")
		expectEqual(t, response.String("article", "description"), "Ever wonder how?")
		expectEqual(t, response.String("article", "body"), "You have to believe")
		expectDeep(t, response.Slice("article", "tagList"), []any{"reactjs", "angularjs", "dragons"})
		expectEqual(t, response.Bool("article", "favorited"), false)
		expectEqual(t, response.Number("article", "favoritesCount"), 0)
		expectEqual(t, response.String("article", "author", "username"), author.creds.Username)
		expectEqual(t, response.Bool("article", "author", "following"), false)
		if len(response.String("article", "createdAt")) != len("2026-01-01T00:00:00.000Z") {
			t.Fatalf("unexpected timestamp %q", response.String("article", "createdAt"))
		}
		expectEqual(t, response.String("article", "createdAt"), response.String("article", "updatedAt"))
	})

	t.Run("gives duplicate titles unique slugs", func(t *testing.T) {
		expectEqual(t, publish(t, author, map[string]any{"title": "Twin"}).String("article", "slug"), "twin")
		expectEqual(t, publish(t, author, map[string]any{"title": "Twin"}).String("article", "slug"), "twin-2")
		expectEqual(t, publish(t, author, map[string]any{"title": "TWIN!"}).String("article", "slug"), "twin-3")
	})

	t.Run("never uses the reserved feed slug", func(t *testing.T) {
		expectEqual(t, publish(t, author, map[string]any{"title": "Feed"}).String("article", "slug"), "feed-2")
	})

	t.Run("normalises tags and allows none", func(t *testing.T) {
		response := publish(t, author, map[string]any{"title": "Tags", "tagList": []string{" go ", "api", "go"}})
		expectDeep(t, response.Slice("article", "tagList"), []any{"go", "api"})
		response = publish(t, author, map[string]any{"title": "No Tags", "tagList": nil})
		expectDeep(t, response.Slice("article", "tagList"), []any{})
	})

	t.Run("validates every field", func(t *testing.T) {
		response := author.client.Post("/api/v1/articles", author.with(support.RequestOptions{JSON: map[string]any{"article": map[string]any{
			"title": " ", "description": strings.Repeat("a", 201), "body": 3, "tagList": "go",
		}}}))
		expectStatus(t, response, 422)
		expectDeep(t, response.Errors("title"), []string{"can't be blank"})
		expectDeep(t, response.Errors("description"), []string{"is too long (maximum is 200 characters)"})
		expectDeep(t, response.Errors("body"), []string{"can't be blank"})
		expectDeep(t, response.Errors("tagList"), []string{"is invalid"})

		response = author.client.Post("/api/v1/articles", author.with(support.RequestOptions{JSON: support.ArticlePayload(map[string]any{"tagList": []string{"two words"}})}))
		expectProblem(t, response, 422, "tagList", "cannot contain whitespace or commas inside a tag")
		response = author.client.Post("/api/v1/articles", author.with(support.RequestOptions{JSON: support.ArticlePayload(map[string]any{"tagList": []any{"ok", 5}})}))
		expectProblem(t, response, 422, "tagList", "is invalid")
	})
}

func TestReadArticle(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)
	reader := signIn(t, app)
	slug := publish(t, author, nil).String("article", "slug")
	path := "/api/v1/articles/" + slug

	t.Run("is public", func(t *testing.T) {
		response := reader.client.Get(path)
		expectStatus(t, response, 200)
		expectEqual(t, response.String("article", "body"), "You have to believe")
	})

	t.Run("personalises flags for signed in readers", func(t *testing.T) {
		reader.client.Post("/api/v1/profiles/"+author.creds.Username+"/follow", reader.auth())
		reader.client.Post(path+"/favorite", reader.auth())
		response := reader.client.Get(path, reader.auth())
		expectEqual(t, response.Bool("article", "favorited"), true)
		expectEqual(t, response.Number("article", "favoritesCount"), 1)
		expectEqual(t, response.Bool("article", "author", "following"), true)

		anonymous := reader.client.Get(path)
		expectEqual(t, anonymous.Bool("article", "favorited"), false)
		expectEqual(t, anonymous.Number("article", "favoritesCount"), 1)
	})

	t.Run("unknown slugs are not found", func(t *testing.T) {
		expectProblem(t, reader.client.Get("/api/v1/articles/nothing-here"), 404, "article", "not found")
	})

	t.Run("an invalid token fails even on public reads", func(t *testing.T) {
		expectProblem(t, reader.client.Get(path, tokenOf("garbage")), 401, "token", "is invalid")
	})
}

func TestListArticles(t *testing.T) {
	app := newApp(t)
	jane := signIn(t, app)
	john := signIn(t, app)
	publish(t, jane, map[string]any{"title": "First", "tagList": []string{"go", "api"}})
	publish(t, john, map[string]any{"title": "Second", "tagList": []string{"go"}})
	publish(t, jane, map[string]any{"title": "Third", "tagList": []string{"web"}})
	john.client.Post("/api/v1/articles/second/favorite", john.auth())
	jane.client.Post("/api/v1/articles/second/favorite", jane.auth())

	t.Run("lists newest first with a total count", func(t *testing.T) {
		response := jane.client.Get("/api/v1/articles")
		expectStatus(t, response, 200)
		expectDeep(t, titles(response), []string{"Third", "Second", "First"})
		expectEqual(t, response.Number("articlesCount"), 3)
		first := response.Slice("articles")[0].(map[string]any)
		if _, hasBody := first["body"]; hasBody {
			t.Fatal("summaries must not include the body")
		}
	})

	t.Run("filters by tag, author and favorited", func(t *testing.T) {
		expectDeep(t, titles(jane.client.Get("/api/v1/articles?tag=go")), []string{"Second", "First"})
		expectDeep(t, titles(jane.client.Get("/api/v1/articles?author="+upper(jane.creds.Username))), []string{"Third", "First"})
		expectDeep(t, titles(jane.client.Get("/api/v1/articles?favorited="+john.creds.Username)), []string{"Second"})
		expectDeep(t, titles(jane.client.Get("/api/v1/articles?tag=go&author="+jane.creds.Username)), []string{"First"})
	})

	t.Run("unknown filters yield an empty page", func(t *testing.T) {
		for _, query := range []string{"?tag=nothing", "?tag=", "?author=nobody", "?favorited=nobody"} {
			response := jane.client.Get("/api/v1/articles" + query)
			expectStatus(t, response, 200)
			expectDeep(t, response.Slice("articles"), []any{})
			expectEqual(t, response.Number("articlesCount"), 0)
		}
	})

	t.Run("pages with limit and offset", func(t *testing.T) {
		response := jane.client.Get("/api/v1/articles?limit=1&offset=1")
		expectDeep(t, titles(response), []string{"Second"})
		expectEqual(t, response.Number("articlesCount"), 3)
		expectDeep(t, titles(jane.client.Get("/api/v1/articles?offset=10")), []string{})
		expectEqual(t, len(titles(jane.client.Get("/api/v1/articles?limit=500"))), 3)
	})

	t.Run("validates paging and filter parameters", func(t *testing.T) {
		expectProblem(t, jane.client.Get("/api/v1/articles?limit=0"), 422, "limit", "must be at least 1")
		expectProblem(t, jane.client.Get("/api/v1/articles?limit=abc"), 422, "limit", "must be an integer")
		expectProblem(t, jane.client.Get("/api/v1/articles?offset=-1"), 422, "offset", "must be an integer")
		expectProblem(t, jane.client.Get("/api/v1/articles?offset=1&offset=2"), 422, "offset", "must be an integer")
		expectProblem(t, jane.client.Get("/api/v1/articles?tag=a&tag=b"), 422, "tag", "is invalid")
	})

	t.Run("personalises flags", func(t *testing.T) {
		response := jane.client.Get("/api/v1/articles?tag=go", jane.auth())
		second := response.Slice("articles")[0].(map[string]any)
		expectEqual(t, second["favorited"], true)
		expectDeep(t, second["favoritesCount"], float64(2))
	})
}

func TestFeed(t *testing.T) {
	app := newApp(t)
	jane := signIn(t, app)
	john := signIn(t, app)
	reader := signIn(t, app)
	publish(t, jane, map[string]any{"title": "By Jane"})
	publish(t, john, map[string]any{"title": "By John"})

	t.Run("requires authentication", func(t *testing.T) {
		expectProblem(t, reader.client.Get("/api/v1/articles/feed"), 401, "token", "is missing")
	})

	t.Run("is empty without follows", func(t *testing.T) {
		response := reader.client.Get("/api/v1/articles/feed", reader.auth())
		expectStatus(t, response, 200)
		expectDeep(t, response.Slice("articles"), []any{})
		expectEqual(t, response.Number("articlesCount"), 0)
	})

	t.Run("contains only followed authors", func(t *testing.T) {
		reader.client.Post("/api/v1/profiles/"+john.creds.Username+"/follow", reader.auth())
		response := reader.client.Get("/api/v1/articles/feed", reader.auth())
		expectDeep(t, titles(response), []string{"By John"})
		expectEqual(t, response.Slice("articles")[0].(map[string]any)["author"].(map[string]any)["following"], true)

		reader.client.Post("/api/v1/profiles/"+jane.creds.Username+"/follow", reader.auth())
		expectDeep(t, titles(reader.client.Get("/api/v1/articles/feed", reader.auth())), []string{"By John", "By Jane"})
		expectDeep(t, titles(reader.client.Get("/api/v1/articles/feed?limit=1&offset=1", reader.auth())), []string{"By Jane"})
	})

	t.Run("validates paging", func(t *testing.T) {
		expectProblem(t, reader.client.Get("/api/v1/articles/feed?limit=0", reader.auth()), 422, "limit", "must be at least 1")
	})
}

func TestUpdateArticle(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)
	other := signIn(t, app)
	publish(t, author, map[string]any{"title": "Original"})
	publish(t, other, map[string]any{"title": "Taken"})
	put := func(who account, slug string, fields map[string]any) support.Response {
		return who.client.Put("/api/v1/articles/"+slug, who.with(support.RequestOptions{JSON: map[string]any{"article": fields}}))
	}

	t.Run("requires authentication", func(t *testing.T) {
		response := author.client.Put("/api/v1/articles/original", support.RequestOptions{JSON: map[string]any{"article": map[string]any{"body": "x"}}})
		expectProblem(t, response, 401, "token", "is missing")
	})

	t.Run("checks existence, then ownership, then validation", func(t *testing.T) {
		expectProblem(t, put(author, "missing", map[string]any{"title": ""}), 404, "article", "not found")
		expectProblem(t, put(other, "original", map[string]any{"title": ""}), 403, "article", "forbidden")
		expectProblem(t, put(author, "original", map[string]any{"title": ""}), 422, "title", "can't be blank")
	})

	t.Run("updates only the provided fields", func(t *testing.T) {
		response := put(author, "original", map[string]any{"body": "New body"})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("article", "body"), "New body")
		expectEqual(t, response.String("article", "title"), "Original")
		expectEqual(t, response.String("article", "slug"), "original")
		expectDeep(t, response.Slice("article", "tagList"), []any{"reactjs", "angularjs", "dragons"})
		if response.String("article", "updatedAt") < response.String("article", "createdAt") {
			t.Fatal("updatedAt must not precede createdAt")
		}
	})

	t.Run("re-slugs on a new title and keeps the old slug dead", func(t *testing.T) {
		response := put(author, "original", map[string]any{"title": "Renamed Article", "tagList": []string{"fresh"}})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("article", "slug"), "renamed-article")
		expectDeep(t, response.Slice("article", "tagList"), []any{"fresh"})
		expectProblem(t, author.client.Get("/api/v1/articles/original"), 404, "article", "not found")
		expectStatus(t, author.client.Get("/api/v1/articles/renamed-article"), 200)
	})

	t.Run("avoids colliding with another article", func(t *testing.T) {
		response := put(author, "renamed-article", map[string]any{"title": "Taken"})
		expectEqual(t, response.String("article", "slug"), "taken-2")
		same := put(author, "taken-2", map[string]any{"title": "Taken"})
		expectEqual(t, same.String("article", "slug"), "taken-2")
	})

	t.Run("an empty update changes nothing", func(t *testing.T) {
		response := put(author, "taken-2", map[string]any{})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("article", "slug"), "taken-2")
	})

	t.Run("clears tags with null", func(t *testing.T) {
		response := put(author, "taken-2", map[string]any{"tagList": nil})
		expectDeep(t, response.Slice("article", "tagList"), []any{})
	})
}

func TestDeleteArticle(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)
	other := signIn(t, app)
	slug := publish(t, author, nil).String("article", "slug")
	path := "/api/v1/articles/" + slug

	other.client.Post(path+"/favorite", other.auth())
	other.client.Post(path+"/comments", other.with(support.RequestOptions{JSON: map[string]any{"comment": map[string]any{"body": "Nice"}}}))

	expectProblem(t, author.client.Delete(path), 401, "token", "is missing")
	expectProblem(t, author.client.Delete("/api/v1/articles/missing", author.auth()), 404, "article", "not found")
	expectProblem(t, other.client.Delete(path, other.auth()), 403, "article", "forbidden")

	response := author.client.Delete(path, author.auth())
	expectStatus(t, response, 204)
	expectEqual(t, len(response.Raw), 0)

	expectProblem(t, author.client.Get(path), 404, "article", "not found")
	expectProblem(t, author.client.Delete(path, author.auth()), 404, "article", "not found")

	var leftovers int
	database := app.Container.Pool
	database.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM comments) + (SELECT count(*) FROM favorites) + (SELECT count(*) FROM article_tags)").Scan(&leftovers)
	expectEqual(t, leftovers, 0)
}

func TestFavorites(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)
	fan := signIn(t, app)
	slug := publish(t, author, nil).String("article", "slug")
	path := "/api/v1/articles/" + slug + "/favorite"

	expectProblem(t, fan.client.Post(path), 401, "token", "is missing")
	expectProblem(t, fan.client.Post("/api/v1/articles/missing/favorite", fan.auth()), 404, "article", "not found")
	expectProblem(t, fan.client.Delete("/api/v1/articles/missing/favorite", fan.auth()), 404, "article", "not found")

	for range 2 {
		response := fan.client.Post(path, fan.auth())
		expectStatus(t, response, 200)
		expectEqual(t, response.Bool("article", "favorited"), true)
		expectEqual(t, response.Number("article", "favoritesCount"), 1)
	}
	response := author.client.Post(path, author.auth())
	expectEqual(t, response.Number("article", "favoritesCount"), 2)

	for range 2 {
		response := fan.client.Delete(path, fan.auth())
		expectStatus(t, response, 200)
		expectEqual(t, response.Bool("article", "favorited"), false)
		expectEqual(t, response.Number("article", "favoritesCount"), 1)
	}
}

func TestTags(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)

	empty := author.client.Get("/api/v1/tags")
	expectStatus(t, empty, 200)
	expectDeep(t, empty.Slice("tags"), []any{})

	publish(t, author, map[string]any{"title": "One", "tagList": []string{"go", "web"}})
	publish(t, author, map[string]any{"title": "Two", "tagList": []string{"go", "api"}})
	publish(t, author, map[string]any{"title": "Three", "tagList": []string{"go", "api", "zeta"}})

	expectDeep(t, author.client.Get("/api/v1/tags").Slice("tags"), []any{"go", "api", "web", "zeta"})
}

func TestSlugsSurviveSpecialTitles(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)
	cases := map[string]string{
		"Crème Brûlée":      "creme-brulee",
		"日本語":               "article",
		"  spaced   out  ":  "spaced-out",
		"What's new in Go?": "what-s-new-in-go",
	}
	for title, want := range cases {
		response := publish(t, author, map[string]any{"title": title})
		expectEqual(t, response.String("article", "slug"), want)
		expectStatus(t, author.client.Get("/api/v1/articles/"+url.PathEscape(want)), 200)
	}
}
