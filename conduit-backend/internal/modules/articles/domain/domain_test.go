package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"

	sharedomain "conduit/backend/internal/shared/domain"
)

func expectProblem(t *testing.T, err error, field, want string) {
	t.Helper()
	got := sharedomain.FieldsOf(err)[field]
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s: got %v, want %q", field, got, want)
	}
}

func tags(values ...any) sharedomain.Raw { return sharedomain.RawOf(values) }

func TestArticleTexts(t *testing.T) {
	title, err := NewArticleTitle(sharedomain.RawString("  Hello  "))
	if err != nil || title.Value() != "Hello" {
		t.Fatalf("got %q %v", title.Value(), err)
	}
	_, err = NewArticleTitle(sharedomain.RawString(strings.Repeat("a", 101)))
	expectProblem(t, err, "title", "is too long (maximum is 100 characters)")
	_, err = NewArticleTitle(sharedomain.Raw{})
	expectProblem(t, err, "title", "can't be blank")

	_, err = NewArticleDescription(sharedomain.RawString(strings.Repeat("a", 201)))
	expectProblem(t, err, "description", "is too long (maximum is 200 characters)")
	_, err = NewArticleDescription(sharedomain.RawString(" "))
	expectProblem(t, err, "description", "can't be blank")

	_, err = NewArticleBody(sharedomain.RawString(strings.Repeat("a", 20001)))
	expectProblem(t, err, "body", "is too long (maximum is 20000 characters)")
	if _, err := NewArticleBody(sharedomain.RawString(strings.Repeat("a", 20000))); err != nil {
		t.Fatal(err)
	}
	_, err = NewArticleBody(sharedomain.RawOf(3))
	expectProblem(t, err, "body", "can't be blank")
}

func TestTagList(t *testing.T) {
	for _, raw := range []sharedomain.Raw{{}, sharedomain.RawOf(nil), tags()} {
		list, err := NewTagList(raw)
		if err != nil || list.Values() == nil || len(list.Values()) != 0 {
			t.Fatalf("absent, null and empty lists are empty: %v %v", list.Values(), err)
		}
	}
	list, err := NewTagList(tags(" go ", "api", "go", "api", "web"))
	if err != nil || !reflect.DeepEqual(list.Values(), []string{"go", "api", "web"}) {
		t.Fatalf("trim and dedupe keeping order: %v %v", list.Values(), err)
	}

	cases := []struct {
		raw  sharedomain.Raw
		want string
	}{
		{sharedomain.RawString("go"), "is invalid"},
		{sharedomain.RawOf(map[string]any{}), "is invalid"},
		{tags("ok", 5), "is invalid"},
		{tags("ok", "  "), "can't contain blank tags"},
		{tags("two words"), "cannot contain whitespace or commas inside a tag"},
		{tags("a,b"), "cannot contain whitespace or commas inside a tag"},
		{tags("a\u00a0b"), "cannot contain whitespace or commas inside a tag"},
		{tags(strings.Repeat("a", 31)), "cannot contain a tag longer than 30 characters"},
	}
	for _, tc := range cases {
		_, err := NewTagList(tc.raw)
		expectProblem(t, err, "tagList", tc.want)
	}

	many := make([]any, 11)
	for i := range many {
		many[i] = strings.Repeat("t", i+1)
	}
	_, err = NewTagList(tags(many...))
	expectProblem(t, err, "tagList", "is too long (maximum is 10 tags)")
	if _, err := NewTagList(tags(many[:10]...)); err != nil {
		t.Fatal(err)
	}

	values := list.Values()
	values[0] = "mutated"
	if list.Values()[0] != "go" {
		t.Error("tag lists must be immutable")
	}
	if !list.Equals(RestoreTagList([]string{"go", "api", "web"})) || list.Equals(RestoreTagList(nil)) {
		t.Error("equality mismatch")
	}
	if RestoreTagList(nil).Values() == nil {
		t.Error("restored nil lists are empty, not nil")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World":             "hello-world",
		"  --Hello,   World!!  ":  "hello-world",
		"Crème Brûlée à la façon": "creme-brulee-a-la-facon",
		"ＦＵＬＬ width":              "full-width",
		"日本語":                     "article",
		"!!!":                     "article",
		"":                        "article",
		"Go 1.22: What's New?":    "go-1-22-what-s-new",
		strings.Repeat("ab ", 60): strings.TrimRight(strings.Repeat("ab-", 60)[:80], "-"),
	}
	for title, want := range cases {
		if got := Slugify(title); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", title, got, want)
		}
	}
	if got := Slugify(strings.Repeat("a", 200)); len(got) != MaxSlugLength {
		t.Errorf("slug length = %d", len(got))
	}
}

func TestSlugCandidate(t *testing.T) {
	if SlugCandidate("hello", 1) != "hello" || SlugCandidate("hello", 2) != "hello-2" || SlugCandidate("hello", 15) != "hello-15" {
		t.Error("unexpected candidates")
	}
	long := strings.Repeat("a", 80)
	candidate := SlugCandidate(long, 10)
	if len(candidate) != MaxSlugLength || !strings.HasSuffix(candidate, "-10") {
		t.Errorf("candidates must respect the limit: %q", candidate)
	}
	if got := SlugCandidate(strings.Repeat("a", 77)+"-b", 2); strings.Contains(got, "--") {
		t.Errorf("dashes must not double up: %q", got)
	}
	if !IsReservedSlug("feed") || IsReservedSlug("hello") {
		t.Error("only feed is reserved")
	}
}

func TestArticleLifecycle(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 123456789, time.UTC)
	title, _ := NewArticleTitle(sharedomain.RawString("Title"))
	description, _ := NewArticleDescription(sharedomain.RawString("Desc"))
	body, _ := NewArticleBody(sharedomain.RawString("Body"))
	list, _ := NewTagList(tags("a"))
	article := Create(NewArticle{ID: "id", AuthorID: "author", Slug: "title", Title: title, Description: description, Body: body, TagList: list, Now: now})

	if !article.IsAuthoredBy("author") || article.IsAuthoredBy("other") {
		t.Error("authorship mismatch")
	}
	if article.CreatedAt().Nanosecond() != 123456000 || !article.CreatedAt().Equal(article.UpdatedAt()) {
		t.Fatalf("unexpected timestamps %v %v", article.CreatedAt(), article.UpdatedAt())
	}

	newTitle, _ := NewArticleTitle(sharedomain.RawString("New"))
	newTags, _ := NewTagList(tags())
	later := now.Add(time.Hour)
	article.Revise(Revision{Slug: "new", Title: &newTitle, TagList: &newTags}, later)
	if article.Slug() != "new" || article.Title().Value() != "New" || article.Description().Value() != "Desc" ||
		article.Body().Value() != "Body" || len(article.TagList().Values()) != 0 {
		t.Fatalf("unexpected article %+v", article.Snapshot())
	}
	if !article.UpdatedAt().Equal(later.Truncate(time.Microsecond)) {
		t.Fatal("updatedAt must advance")
	}

	article.Revise(Revision{}, later.Add(time.Hour))
	if article.Slug() != "new" {
		t.Error("an empty slug keeps the current one")
	}

	restored := Restore(article.Snapshot())
	if !reflect.DeepEqual(restored.Snapshot(), article.Snapshot()) {
		t.Fatal("restore must round trip")
	}
}
