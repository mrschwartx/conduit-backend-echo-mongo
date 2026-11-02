//go:build integration

package integration

import (
	"strconv"
	"strings"
	"testing"

	"conduit/backend/test/support"
)

func commentBody(text any) support.RequestOptions {
	return support.RequestOptions{JSON: map[string]any{"comment": map[string]any{"body": text}}}
}

func TestComments(t *testing.T) {
	app := newApp(t)
	author := signIn(t, app)
	jane := signIn(t, app)
	john := signIn(t, app)
	slug := publish(t, author, nil).String("article", "slug")
	other := publish(t, author, map[string]any{"title": "Another"}).String("article", "slug")
	path := "/api/v1/articles/" + slug + "/comments"

	t.Run("creating requires authentication", func(t *testing.T) {
		expectProblem(t, jane.client.Post(path, commentBody("hi")), 401, "token", "is missing")
	})

	t.Run("creates comments", func(t *testing.T) {
		response := jane.client.Post(path, jane.with(commentBody(" Great read ")))
		expectStatus(t, response, 201)
		expectEqual(t, response.String("comment", "body"), "Great read")
		expectEqual(t, response.String("comment", "author", "username"), jane.creds.Username)
		expectEqual(t, response.String("comment", "createdAt"), response.String("comment", "updatedAt"))
		if response.Number("comment", "id") < 1 {
			t.Fatalf("an integer id is expected: %s", response.Raw)
		}
	})

	t.Run("validates the body", func(t *testing.T) {
		expectProblem(t, jane.client.Post(path, jane.with(commentBody("   "))), 422, "body", "can't be blank")
		expectProblem(t, jane.client.Post(path, jane.with(commentBody(5))), 422, "body", "can't be blank")
		expectProblem(t, jane.client.Post(path, jane.with(commentBody(strings.Repeat("a", 1001)))), 422, "body", "is too long (maximum is 1000 characters)")
		expectProblem(t, jane.client.Post(path, jane.with(support.RequestOptions{RawBody: `{}`, ContentType: "application/json"})), 422, "body", "can't be blank")
	})

	t.Run("commenting on a missing article is not found", func(t *testing.T) {
		expectProblem(t, jane.client.Post("/api/v1/articles/missing/comments", jane.with(commentBody("hi"))), 404, "article", "not found")
	})

	t.Run("lists newest first with author flags", func(t *testing.T) {
		john.client.Post(path, john.with(commentBody("Second!")))
		john.client.Post("/api/v1/profiles/"+jane.creds.Username+"/follow", john.auth())

		response := john.client.Get(path, john.auth())
		expectStatus(t, response, 200)
		comments := response.Slice("comments")
		expectEqual(t, len(comments), 2)
		first := comments[0].(map[string]any)
		last := comments[1].(map[string]any)
		expectEqual(t, first["body"], "Second!")
		expectEqual(t, last["body"], "Great read")
		expectEqual(t, last["author"].(map[string]any)["following"], true)
		expectEqual(t, first["author"].(map[string]any)["following"], false)

		anonymous := john.client.Get(path)
		expectEqual(t, anonymous.Slice("comments")[1].(map[string]any)["author"].(map[string]any)["following"], false)
	})

	t.Run("an article without comments lists an empty array", func(t *testing.T) {
		response := jane.client.Get("/api/v1/articles/" + other + "/comments")
		expectStatus(t, response, 200)
		expectDeep(t, response.Slice("comments"), []any{})
		expectProblem(t, jane.client.Get("/api/v1/articles/missing/comments"), 404, "article", "not found")
	})

	t.Run("deleting follows the check order", func(t *testing.T) {
		list := jane.client.Get(path)
		janeComment := strconv.Itoa(int(list.Slice("comments")[1].(map[string]any)["id"].(float64)))
		target := path + "/" + janeComment

		expectProblem(t, jane.client.Delete(target), 401, "token", "is missing")
		expectProblem(t, jane.client.Delete("/api/v1/articles/missing/comments/"+janeComment, jane.auth()), 404, "article", "not found")
		expectProblem(t, jane.client.Delete(path+"/abc", jane.auth()), 422, "id", "must be an integer")
		expectProblem(t, jane.client.Delete(path+"/-1", jane.auth()), 422, "id", "must be an integer")
		expectProblem(t, jane.client.Delete(path+"/99999999999999999999", jane.auth()), 404, "comment", "not found")
		expectProblem(t, jane.client.Delete(path+"/987654", jane.auth()), 404, "comment", "not found")
		expectProblem(t, jane.client.Delete("/api/v1/articles/"+other+"/comments/"+janeComment, jane.auth()), 404, "comment", "not found")
		expectProblem(t, john.client.Delete(target, john.auth()), 403, "comment", "forbidden")

		response := jane.client.Delete(target, jane.auth())
		expectStatus(t, response, 204)
		expectEqual(t, len(response.Raw), 0)
		expectProblem(t, jane.client.Delete(target, jane.auth()), 404, "comment", "not found")
		expectEqual(t, len(jane.client.Get(path).Slice("comments")), 1)
	})
}
