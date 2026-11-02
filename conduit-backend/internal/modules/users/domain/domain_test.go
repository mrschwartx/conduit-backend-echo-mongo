package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"

	sharedomain "conduit/backend/internal/shared/domain"
)

func messages(err error, field string) []string {
	return sharedomain.FieldsOf(err)[field]
}

func expectProblem(t *testing.T, err error, field, want string) {
	t.Helper()
	got := messages(err, field)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s: got %v, want %q", field, got, want)
	}
}

func TestUsername(t *testing.T) {
	if value, err := NewUsername(sharedomain.RawString("  john_doe-1  ")); err != nil || value.Value() != "john_doe-1" {
		t.Fatalf("got %q %v", value.Value(), err)
	}
	cases := []struct {
		raw  sharedomain.Raw
		want string
	}{
		{sharedomain.RawOf(1), "can't be blank"},
		{sharedomain.Raw{}, "can't be blank"},
		{sharedomain.RawString("   "), "can't be blank"},
		{sharedomain.RawString("ab"), "is too short (minimum is 3 characters)"},
		{sharedomain.RawString(strings.Repeat("a", 31)), "is too long (maximum is 30 characters)"},
		{sharedomain.RawString("john doe"), "is invalid"},
		{sharedomain.RawString("jöhn"), "is invalid"},
	}
	for _, tc := range cases {
		_, err := NewUsername(tc.raw)
		expectProblem(t, err, "username", tc.want)
	}
	if !RestoreUsername("a").Equals(RestoreUsername("a")) || RestoreUsername("a").Equals(RestoreUsername("b")) {
		t.Error("equality mismatch")
	}
}

func TestEmail(t *testing.T) {
	value, err := NewEmail(sharedomain.RawString("  John@Mail.COM "))
	if err != nil || value.Value() != "john@mail.com" {
		t.Fatalf("got %q %v", value.Value(), err)
	}
	for _, valid := range []string{"a@b.co", "first.last+tag@sub.example.org"} {
		if _, err := NewEmail(sharedomain.RawString(valid)); err != nil {
			t.Errorf("%q should be valid: %v", valid, err)
		}
	}
	for _, invalid := range []string{"plain", "a@b", "a@@b.co", "@b.co", "a b@c.co"} {
		_, err := NewEmail(sharedomain.RawString(invalid))
		expectProblem(t, err, "email", "is invalid")
	}
	_, err = NewEmail(sharedomain.RawString(strings.Repeat("a", 250) + "@b.co"))
	expectProblem(t, err, "email", "is too long (maximum is 254 characters)")
	_, err = NewEmail(sharedomain.RawOf(nil))
	expectProblem(t, err, "email", "can't be blank")
	if !RestoreEmail("a@b.co").Equals(RestoreEmail("a@b.co")) {
		t.Error("equality mismatch")
	}
}

func TestPlainPassword(t *testing.T) {
	for _, valid := range []string{"exactly8", strings.Repeat("a", 72), "        "} {
		if _, err := NewPlainPassword(sharedomain.RawString(valid)); err != nil {
			t.Errorf("%q should be valid: %v", valid, err)
		}
	}
	cases := []struct {
		raw  sharedomain.Raw
		want string
	}{
		{sharedomain.RawOf(12345678), "can't be blank"},
		{sharedomain.RawString(""), "can't be blank"},
		{sharedomain.Raw{}, "can't be blank"},
		{sharedomain.RawString("short"), "is too short (minimum is 8 characters)"},
		{sharedomain.RawString(strings.Repeat("a", 73)), "is too long (maximum is 72 characters)"},
		{sharedomain.RawString(strings.Repeat("é", 37)), "is too long (maximum is 72 bytes)"},
	}
	for _, tc := range cases {
		_, err := NewPlainPassword(tc.raw)
		expectProblem(t, err, "password", tc.want)
	}
}

func TestBio(t *testing.T) {
	if bio, err := NewBio(sharedomain.RawOf(nil)); err != nil || bio != nil || bio.Value() != nil {
		t.Fatalf("null clears the bio: %v %v", bio, err)
	}
	if bio, err := NewBio(sharedomain.RawString("   ")); err != nil || bio != nil {
		t.Fatalf("blank clears the bio: %v %v", bio, err)
	}
	bio, err := NewBio(sharedomain.RawString("  hello  "))
	if err != nil || *bio.Value() != "hello" {
		t.Fatalf("got %v %v", bio, err)
	}
	_, err = NewBio(sharedomain.RawOf(5))
	expectProblem(t, err, "bio", "is invalid")
	_, err = NewBio(sharedomain.RawString(strings.Repeat("a", 301)))
	expectProblem(t, err, "bio", "is too long (maximum is 300 characters)")
	if RestoreBio(nil) != nil {
		t.Error("restoring nil yields nil")
	}
}

func TestProfileImage(t *testing.T) {
	if image, err := NewProfileImage(sharedomain.RawOf(nil)); err != nil || image != nil {
		t.Fatalf("null clears the image: %v %v", image, err)
	}
	if image, err := NewProfileImage(sharedomain.RawString("")); err != nil || image != nil {
		t.Fatalf("blank clears the image: %v %v", image, err)
	}
	image, err := NewProfileImage(sharedomain.RawString(" https://example.com/a.png "))
	if err != nil || *image.Value() != "https://example.com/a.png" {
		t.Fatalf("got %v %v", image, err)
	}
	for _, invalid := range []string{"http://example.com/a.png", "not a url", "https://", "ftp://x.com/a"} {
		_, err := NewProfileImage(sharedomain.RawString(invalid))
		expectProblem(t, err, "image", "must be a valid https URL")
	}
	_, err = NewProfileImage(sharedomain.RawOf(1))
	expectProblem(t, err, "image", "is invalid")
	_, err = NewProfileImage(sharedomain.RawString("https://example.com/" + strings.Repeat("a", 2048)))
	expectProblem(t, err, "image", "is too long (maximum is 2048 characters)")
	if RestoreProfileImage(nil) != nil {
		t.Error("restoring nil yields nil")
	}
}

func TestUserLifecycle(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 123456789, time.UTC)
	username, _ := NewUsername(sharedomain.RawString("john"))
	email, _ := NewEmail(sharedomain.RawString("john@mail.com"))
	user := Register(Registration{ID: "id", Username: username, Email: email, PasswordHash: "hash", Now: now})

	snapshot := user.Snapshot()
	if snapshot.Bio != nil || snapshot.Image != nil || snapshot.CreatedAt.Nanosecond() != 123456000 || !snapshot.CreatedAt.Equal(snapshot.UpdatedAt) {
		t.Fatalf("unexpected snapshot %+v", snapshot)
	}

	newName, _ := NewUsername(sharedomain.RawString("johnny"))
	bio, _ := NewBio(sharedomain.RawString("hi"))
	image, _ := NewProfileImage(sharedomain.RawString("https://example.com/a.png"))
	hash := "new-hash"
	later := now.Add(time.Hour)
	user.UpdateProfile(ProfileChanges{Username: &newName, PasswordHash: &hash, SetBio: true, Bio: bio, SetImage: true, Image: image}, later)
	if user.Username().Value() != "johnny" || user.Email().Value() != "john@mail.com" || user.PasswordHash() != "new-hash" {
		t.Fatalf("unexpected user %+v", user.Snapshot())
	}
	if *user.Bio().Value() != "hi" || *user.Image().Value() != "https://example.com/a.png" || !user.UpdatedAt().Equal(later.Truncate(time.Microsecond)) {
		t.Fatalf("unexpected profile %+v", user.Snapshot())
	}

	empty := ""
	user.UpdateProfile(ProfileChanges{PasswordHash: &empty, SetBio: true, SetImage: true}, later)
	if user.PasswordHash() != "new-hash" || user.Bio() != nil || user.Image() != nil {
		t.Fatalf("empty hashes are ignored and nil clears: %+v", user.Snapshot())
	}

	restored := Restore(user.Snapshot())
	if !reflect.DeepEqual(restored.Snapshot(), user.Snapshot()) || restored.ID() != "id" || !restored.CreatedAt().Equal(now.Truncate(time.Microsecond)) {
		t.Fatal("restore must round trip")
	}
}
