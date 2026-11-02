package application

import (
	"context"
	"reflect"
	"testing"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type fixture struct {
	users    *memoryUsers
	follows  *memoryFollows
	register *RegisterUser
	login    *AuthenticateUser
	current  *GetCurrentUser
	update   *UpdateCurrentUser
	verify   *VerifyAccessToken
}

func newFixture() fixture {
	users := newMemoryUsers()
	follows := newMemoryFollows()
	return fixture{
		users:    users,
		follows:  follows,
		register: NewRegisterUser(users, fakeHasher{}, fakeTokens{}),
		login:    NewAuthenticateUser(users, fakeHasher{}, fakeTokens{}),
		current:  NewGetCurrentUser(users, fakeTokens{}),
		update:   NewUpdateCurrentUser(users, fakeHasher{}, fakeTokens{}),
		verify:   NewVerifyAccessToken(fakeTokens{}, users),
	}
}

func str(value string) sharedomain.Raw { return sharedomain.RawString(value) }

func (f fixture) signUp(t *testing.T, username, email string) UserDTO {
	t.Helper()
	dto, err := f.register.Execute(context.Background(), RegisterUserInput{
		Username: str(username), Email: str(email), Password: str("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return dto
}

func expectFields(t *testing.T, err error, kind sharedomain.ErrorKind, want map[string][]string) {
	t.Helper()
	if !sharedomain.IsKind(err, kind) || !reflect.DeepEqual(sharedomain.FieldsOf(err), want) {
		t.Fatalf("got kind=%v fields=%v, want %s %v", err, sharedomain.FieldsOf(err), kind, want)
	}
}

func TestRegisterUser(t *testing.T) {
	f := newFixture()
	dto, err := f.register.Execute(context.Background(), RegisterUserInput{
		Username: str(" Jane "), Email: str("Jane@Mail.com"), Password: str("password123"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dto.Username != "Jane" || dto.Email != "jane@mail.com" || dto.Token != "token-for-user-1" || dto.Bio != nil || dto.Image != nil {
		t.Fatalf("unexpected dto %+v", dto)
	}
	stored, _, _ := f.users.FindByID(context.Background(), "user-1")
	if stored.PasswordHash() != "hashed:password123" {
		t.Fatalf("password must be hashed, got %q", stored.PasswordHash())
	}
}

func TestRegisterUserReportsEveryInvalidField(t *testing.T) {
	f := newFixture()
	_, err := f.register.Execute(context.Background(), RegisterUserInput{})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{
		"username": {"can't be blank"}, "email": {"can't be blank"}, "password": {"can't be blank"},
	})
	if len(f.users.byID) != 0 {
		t.Fatal("nothing may be stored")
	}
}

func TestRegisterUserRejectsDuplicates(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")
	_, err := f.register.Execute(context.Background(), RegisterUserInput{
		Username: str("JANE"), Email: str("jane@mail.com"), Password: str("password123"),
	})
	expectFields(t, err, sharedomain.KindConflict, map[string][]string{
		"email": {"has already been taken"}, "username": {"has already been taken"},
	})
	_, err = f.register.Execute(context.Background(), RegisterUserInput{
		Username: str("other"), Email: str("jane@mail.com"), Password: str("password123"),
	})
	expectFields(t, err, sharedomain.KindConflict, map[string][]string{"email": {"has already been taken"}})
}

func TestAuthenticateUser(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")

	dto, err := f.login.Execute(context.Background(), AuthenticateUserInput{Email: str(" JANE@mail.com "), Password: str("password123")})
	if err != nil || dto.Username != "jane" || dto.Token != "token-for-user-1" {
		t.Fatalf("got %+v %v", dto, err)
	}

	for _, input := range []AuthenticateUserInput{
		{Email: str("jane@mail.com"), Password: str("wrong-password")},
		{Email: str("nobody@mail.com"), Password: str("password123")},
	} {
		_, err := f.login.Execute(context.Background(), input)
		expectFields(t, err, sharedomain.KindAuthentication, map[string][]string{"credentials": {"are invalid"}})
	}

	_, err = f.login.Execute(context.Background(), AuthenticateUserInput{Email: str("  "), Password: sharedomain.RawOf(5)})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{
		"email": {"can't be blank"}, "password": {"can't be blank"},
	})
}

func TestGetCurrentUser(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")
	dto, err := f.current.Execute(context.Background(), "user-1")
	if err != nil || dto.Email != "jane@mail.com" {
		t.Fatalf("got %+v %v", dto, err)
	}
	_, err = f.current.Execute(context.Background(), "ghost")
	expectFields(t, err, sharedomain.KindAuthentication, map[string][]string{"token": {"is invalid"}})
}

func TestUpdateCurrentUser(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")
	f.signUp(t, "john", "john@mail.com")
	ctx := context.Background()

	dto, err := f.update.Execute(ctx, UpdateCurrentUserInput{UserID: "user-1", Changes: UserChanges{
		Username: str("janet"), Bio: str(" Writer "), Image: str("https://example.com/me.png"), Password: str("newpassword1"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if dto.Username != "janet" || *dto.Bio != "Writer" || *dto.Image != "https://example.com/me.png" || dto.Email != "jane@mail.com" {
		t.Fatalf("unexpected dto %+v", dto)
	}
	stored, _, _ := f.users.FindByID(ctx, "user-1")
	if stored.PasswordHash() != "hashed:newpassword1" {
		t.Fatalf("password not re-hashed: %q", stored.PasswordHash())
	}

	dto, err = f.update.Execute(ctx, UpdateCurrentUserInput{UserID: "user-1", Changes: UserChanges{Bio: sharedomain.RawOf(nil), Image: sharedomain.RawOf(nil)}})
	if err != nil || dto.Bio != nil || dto.Image != nil || dto.Username != "janet" {
		t.Fatalf("null clears optional fields: %+v %v", dto, err)
	}

	dto, err = f.update.Execute(ctx, UpdateCurrentUserInput{UserID: "user-1", Changes: UserChanges{Username: str("JANET"), Email: str("jane@mail.com")}})
	if err != nil || dto.Username != "JANET" {
		t.Fatalf("a user may re-use their own values: %+v %v", dto, err)
	}
}

func TestUpdateCurrentUserFailures(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")
	f.signUp(t, "john", "john@mail.com")
	ctx := context.Background()

	_, err := f.update.Execute(ctx, UpdateCurrentUserInput{UserID: "user-1"})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"user": {"can't be empty"}})

	_, err = f.update.Execute(ctx, UpdateCurrentUserInput{UserID: "user-1", Changes: UserChanges{
		Username: str("x"), Email: str("bad"), Password: str("short"), Image: str("http://x.com/a.png"),
	}})
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{
		"username": {"is too short (minimum is 3 characters)"},
		"email":    {"is invalid"},
		"password": {"is too short (minimum is 8 characters)"},
		"image":    {"must be a valid https URL"},
	})

	_, err = f.update.Execute(ctx, UpdateCurrentUserInput{UserID: "user-1", Changes: UserChanges{Username: str("John"), Email: str("john@mail.com")}})
	expectFields(t, err, sharedomain.KindConflict, map[string][]string{
		"email": {"has already been taken"}, "username": {"has already been taken"},
	})

	_, err = f.update.Execute(ctx, UpdateCurrentUserInput{UserID: "ghost", Changes: UserChanges{Bio: str("x")}})
	expectFields(t, err, sharedomain.KindAuthentication, map[string][]string{"token": {"is invalid"}})
}

func TestVerifyAccessToken(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")
	ctx := context.Background()

	if id, err := f.verify.Execute(ctx, "token-for-user-1"); err != nil || id != "user-1" {
		t.Fatalf("got %q %v", id, err)
	}
	_, err := f.verify.Execute(ctx, "")
	expectFields(t, err, sharedomain.KindAuthentication, map[string][]string{"token": {"is missing"}})
	_, err = f.verify.Execute(ctx, "garbage")
	if err == nil {
		t.Fatal("invalid tokens must fail")
	}
	_, err = f.verify.Execute(ctx, "token-for-deleted")
	expectFields(t, err, sharedomain.KindAuthentication, map[string][]string{"token": {"is invalid"}})
}

func TestProfilesAndFollows(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")
	f.signUp(t, "john", "john@mail.com")
	ctx := context.Background()
	get := NewGetProfile(f.users, f.follows)
	follow := NewFollowUser(f.users, f.follows)
	unfollow := NewUnfollowUser(f.users, f.follows)

	profile, err := get.Execute(ctx, "JOHN", "")
	if err != nil || profile.Username != "john" || profile.Following {
		t.Fatalf("got %+v %v", profile, err)
	}
	if _, err := get.Execute(ctx, "ghost", ""); !sharedomain.IsKind(err, sharedomain.KindNotFound) {
		t.Fatalf("unknown profiles are not found: %v", err)
	}

	profile, err = follow.Execute(ctx, "user-1", "john")
	if err != nil || !profile.Following {
		t.Fatalf("got %+v %v", profile, err)
	}
	if _, err := follow.Execute(ctx, "user-1", "john"); err != nil {
		t.Fatalf("following twice is idempotent: %v", err)
	}
	profile, _ = get.Execute(ctx, "john", "user-1")
	if !profile.Following {
		t.Fatal("viewer must see following=true")
	}
	profile, _ = get.Execute(ctx, "john", "user-2")
	if profile.Following {
		t.Fatal("other viewers must not")
	}

	_, err = follow.Execute(ctx, "user-1", "jane")
	expectFields(t, err, sharedomain.KindValidation, map[string][]string{"username": {"you cannot follow yourself"}})
	if _, err := follow.Execute(ctx, "user-1", "ghost"); !sharedomain.IsKind(err, sharedomain.KindNotFound) {
		t.Fatalf("got %v", err)
	}

	profile, err = unfollow.Execute(ctx, "user-1", "john")
	if err != nil || profile.Following {
		t.Fatalf("got %+v %v", profile, err)
	}
	if _, err := unfollow.Execute(ctx, "user-1", "john"); err != nil {
		t.Fatalf("unfollowing twice is idempotent: %v", err)
	}
	if _, err := unfollow.Execute(ctx, "user-1", "ghost"); !sharedomain.IsKind(err, sharedomain.KindNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthorDirectory(t *testing.T) {
	f := newFixture()
	f.signUp(t, "jane", "jane@mail.com")
	f.signUp(t, "john", "john@mail.com")
	ctx := context.Background()
	f.follows.Follow(ctx, "user-1", "user-2")
	directory := NewAuthorDirectory(f.users, f.follows)

	profiles, err := directory.FindProfiles(ctx, []string{"user-2", "user-2", "user-1", "ghost"}, "user-1")
	if err != nil || len(profiles) != 2 || !profiles["user-2"].Following || profiles["user-1"].Following {
		t.Fatalf("got %+v %v", profiles, err)
	}
	anonymous, _ := directory.FindProfiles(ctx, []string{"user-2"}, "")
	if anonymous["user-2"].Following {
		t.Fatal("anonymous viewers follow nobody")
	}
	if empty, err := directory.FindProfiles(ctx, nil, "user-1"); err != nil || len(empty) != 0 {
		t.Fatalf("got %+v %v", empty, err)
	}

	if id, ok, _ := directory.FindIDByUsername(ctx, "JOHN"); !ok || id != "user-2" {
		t.Fatalf("got %q %v", id, ok)
	}
	if _, ok, _ := directory.FindIDByUsername(ctx, "ghost"); ok {
		t.Fatal("unknown usernames resolve to nothing")
	}
	if ids, _ := directory.ListFollowedIDs(ctx, "user-1"); !reflect.DeepEqual(ids, []string{"user-2"}) {
		t.Fatalf("got %v", ids)
	}
}

var _ domain.Repository = (*memoryUsers)(nil)
var _ domain.FollowRepository = (*memoryFollows)(nil)
