package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewEntityIDProducesLowercaseV4UUIDs(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		id := NewEntityID()
		if !IsEntityID(id) || id != strings.ToLower(id) || id[14] != '4' {
			t.Fatalf("invalid id %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
	for _, value := range []string{"", "not-an-id", "aaaaaaaaaaaa4aaa8aaaaaaaaaaaaaaa", "gggggggg-gggg-4ggg-8ggg-gggggggggggg"} {
		if IsEntityID(value) {
			t.Errorf("%q must not be an entity id", value)
		}
	}
}

func TestErrorConstructors(t *testing.T) {
	cases := []struct {
		err    error
		kind   ErrorKind
		fields map[string][]string
	}{
		{NewFieldError("title", "a", "b"), KindValidation, map[string][]string{"title": {"a", "b"}}},
		{NewConflictError("email", "username"), KindConflict, map[string][]string{"email": {"has already been taken"}, "username": {"has already been taken"}}},
		{NewNotFoundError("article"), KindNotFound, map[string][]string{"article": {"not found"}}},
		{NewForbiddenError("comment"), KindForbidden, map[string][]string{"comment": {"forbidden"}}},
		{NewMissingTokenError(), KindAuthentication, map[string][]string{"token": {"is missing"}}},
		{NewInvalidTokenError(), KindAuthentication, map[string][]string{"token": {"is invalid"}}},
		{NewExpiredTokenError(), KindAuthentication, map[string][]string{"token": {"has expired"}}},
		{NewInvalidCredentialsError(), KindAuthentication, map[string][]string{"credentials": {"are invalid"}}},
		{NewUnsupportedMediaTypeError(), KindUnsupportedMedia, map[string][]string{"content-type": {"must be application/json"}}},
		{NewMalformedBodyError(), KindMalformedBody, map[string][]string{"body": {"is not valid JSON"}}},
		{NewPayloadTooLargeError(), KindPayloadTooLarge, map[string][]string{"body": {"is too large"}}},
	}
	for _, tc := range cases {
		if !IsKind(tc.err, tc.kind) || !reflect.DeepEqual(FieldsOf(tc.err), tc.fields) {
			t.Errorf("%v: kind or fields mismatch, got %v", tc.kind, FieldsOf(tc.err))
		}
		wrapped := fmt.Errorf("wrapped: %w", tc.err)
		if got, ok := KindOf(wrapped); !ok || got != tc.kind {
			t.Errorf("wrapped kind %s mismatch", tc.kind)
		}
	}
	if _, ok := KindOf(errors.New("plain")); ok {
		t.Error("plain errors have no kind")
	}
	if FieldsOf(errors.New("plain")) != nil {
		t.Error("plain errors have no fields")
	}
}

func TestErrorCollector(t *testing.T) {
	collector := NewErrorCollector()
	if collector.Err() != nil || !collector.IsEmpty() {
		t.Fatal("a new collector is empty")
	}
	if err := collector.Capture(nil); err != nil {
		t.Fatal(err)
	}
	if err := collector.Capture(NewFieldError("a", "x")); err != nil {
		t.Fatal(err)
	}
	collector.Add("a", "y")
	collector.Add("b", "z")
	plain := errors.New("boom")
	if err := collector.Capture(plain); err != plain {
		t.Fatalf("non validation errors pass through, got %v", err)
	}
	want := map[string][]string{"a": {"x", "y"}, "b": {"z"}}
	if err := collector.Err(); !IsKind(err, KindValidation) || !reflect.DeepEqual(FieldsOf(err), want) {
		t.Fatalf("unexpected %v", FieldsOf(err))
	}
}

func TestRawDistinguishesAbsentNullAndValues(t *testing.T) {
	var payload struct {
		Present Raw `json:"present"`
		Null    Raw `json:"null"`
		Number  Raw `json:"number"`
		Missing Raw `json:"missing"`
	}
	if err := json.Unmarshal([]byte(`{"present":"hi","null":null,"number":5}`), &payload); err != nil {
		t.Fatal(err)
	}
	if text, ok := payload.Present.Text(); !ok || text != "hi" {
		t.Errorf("present = %q %v", text, ok)
	}
	if !payload.Null.IsSet() || payload.Null.Value() != nil {
		t.Error("null must count as set with a nil value")
	}
	if _, ok := payload.Number.Text(); ok || !payload.Number.IsSet() {
		t.Error("number is set but not text")
	}
	if payload.Missing.IsSet() {
		t.Error("missing must not count as set")
	}
}

func TestFieldsGetReturnsUnsetForMissingKeys(t *testing.T) {
	var fields Fields
	if fields.Get("x").IsSet() {
		t.Fatal("nil fields have no keys")
	}
	if err := json.Unmarshal([]byte(`{"a":"b","c":null}`), &fields); err != nil {
		t.Fatal(err)
	}
	if text, _ := fields.Get("a").Text(); text != "b" || !fields.Get("c").IsSet() || fields.Get("A").IsSet() {
		t.Fatalf("unexpected fields %+v", fields)
	}
}

func TestValidateText(t *testing.T) {
	rule := TextRule{Min: 1, Max: 5}
	if value, err := ValidateText("name", RawString("  ab "), rule); err != nil || value != "ab" {
		t.Fatalf("trim failed: %q %v", value, err)
	}
	long := TextRule{Min: 3, Max: 5}
	cases := []struct {
		raw  Raw
		rule TextRule
		want string
	}{
		{RawOf(12), rule, "can't be blank"},
		{RawOf(nil), rule, "can't be blank"},
		{Raw{}, rule, "can't be blank"},
		{RawString("   "), rule, "can't be blank"},
		{RawString("ab"), long, "is too short (minimum is 3 characters)"},
		{RawString("abcdef"), rule, "is too long (maximum is 5 characters)"},
		{RawString("😀😀😀😀😀😀"), rule, "is too long (maximum is 5 characters)"},
	}
	for _, tc := range cases {
		_, err := ValidateText("name", tc.raw, tc.rule)
		if got := FieldsOf(err)["name"]; len(got) != 1 || got[0] != tc.want {
			t.Errorf("got %v, want %q", got, tc.want)
		}
	}
	if _, err := ValidateText("name", RawString("😀😀😀😀😀"), rule); err != nil {
		t.Errorf("code points are counted, not UTF-16 units: %v", err)
	}
}

func TestTrimTextAndIsSpace(t *testing.T) {
	if got := TrimText("\u00a0\ufeff  hello \n\t"); got != "hello" {
		t.Errorf("got %q", got)
	}
	if IsSpace(0x85) || !IsSpace(0xFEFF) || !IsSpace(' ') || IsSpace('a') {
		t.Error("IsSpace must follow the JavaScript definition")
	}
}

func TestParsePagination(t *testing.T) {
	ok := []struct {
		offset, limit []string
		want          Pagination
	}{
		{nil, nil, Pagination{Offset: 0, Limit: 20}},
		{[]string{"5"}, []string{"10"}, Pagination{Offset: 5, Limit: 10}},
		{[]string{" 7 "}, []string{"500"}, Pagination{Offset: 7, Limit: 50}},
	}
	for _, tc := range ok {
		got, err := ParsePagination(tc.offset, tc.limit)
		if err != nil || got != tc.want {
			t.Errorf("got %+v %v, want %+v", got, err, tc.want)
		}
	}
	bad := []struct {
		offset, limit []string
		want          map[string][]string
	}{
		{[]string{"x"}, nil, map[string][]string{"offset": {"must be an integer"}}},
		{[]string{"-1"}, nil, map[string][]string{"offset": {"must be an integer"}}},
		{nil, []string{"0"}, map[string][]string{"limit": {"must be at least 1"}}},
		{nil, []string{"1.5"}, map[string][]string{"limit": {"must be an integer"}}},
		{[]string{"1", "2"}, []string{""}, map[string][]string{"offset": {"must be an integer"}, "limit": {"must be an integer"}}},
		{[]string{"99999999999999999999"}, nil, map[string][]string{"offset": {"must be at least 0"}}},
	}
	for _, tc := range bad {
		_, err := ParsePagination(tc.offset, tc.limit)
		if !IsKind(err, KindValidation) || !reflect.DeepEqual(FieldsOf(err), tc.want) {
			t.Errorf("offset=%v limit=%v: got %v", tc.offset, tc.limit, FieldsOf(err))
		}
	}
}

func TestTimestampHelpers(t *testing.T) {
	value := time.Date(2026, 3, 4, 5, 6, 7, 123456789, time.FixedZone("x", 3600))
	if got := FormatTimestamp(value); got != "2026-03-04T04:06:07.123Z" {
		t.Errorf("format = %s", got)
	}
	if got := TruncateToMicrosecond(value).Nanosecond(); got != 123456000 {
		t.Errorf("µs truncation = %d", got)
	}
}

func TestNewEvent(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))
	event := NewEvent(ArticleDeletedEvent, ArticleDeleted{ArticleID: "a"}, at)
	if event.Name != "article.deleted" || event.OccurredAt.Location() != time.UTC || !event.OccurredAt.Equal(at) {
		t.Fatalf("unexpected event %+v", event)
	}
}
