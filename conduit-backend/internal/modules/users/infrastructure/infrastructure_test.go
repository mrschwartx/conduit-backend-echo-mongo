package infrastructure

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	sharedomain "conduit/backend/internal/shared/domain"
)

const (
	secret = "test-secret-value-with-more-than-32-chars"
	userID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func TestBcryptHasher(t *testing.T) {
	hasher := NewBcryptHasher(4)
	ctx := context.Background()

	hash, err := hasher.Hash(ctx, "Password123!")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "Password123!" || !strings.HasPrefix(hash, "$2a$04$") {
		t.Fatalf("unexpected hash %q", hash)
	}
	if !hasher.Compare(ctx, "Password123!", hash) || hasher.Compare(ctx, "other", hash) {
		t.Fatal("compare mismatch")
	}
	if hasher.Compare(ctx, "x", "not-a-hash") {
		t.Fatal("malformed hashes must not match")
	}
	second, _ := hasher.Hash(ctx, "Password123!")
	if second == hash {
		t.Fatal("hashes must be salted")
	}
}

func TestBcryptHasherAcceptsTheTimingHash(t *testing.T) {
	hasher := NewBcryptHasher(4)
	if hasher.Compare(context.Background(), "whatever", "$2b$10$la9MsO03GrXB6wPVIxJHf.AWuYIX2h8xZbkFpc8VuCbg2atOt7OEe") {
		t.Fatal("the dummy timing hash must not match arbitrary passwords")
	}
}

func expectToken(t *testing.T, err error, want string) {
	t.Helper()
	fields := sharedomain.FieldsOf(err)
	if !sharedomain.IsKind(err, sharedomain.KindAuthentication) || len(fields["token"]) != 1 || fields["token"][0] != want {
		t.Fatalf("expected token %q, got %v", want, err)
	}
}

func TestJWTTokenService(t *testing.T) {
	service := NewJWTTokenService(secret, time.Hour)

	token, err := service.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := service.Verify(token); err != nil || id != userID {
		t.Fatalf("verify failed: %q %v", id, err)
	}

	upperToken, _ := service.Issue(strings.ToUpper(userID))
	if id, err := service.Verify(upperToken); err != nil || id != userID {
		t.Fatalf("ids must be lowercased: %q %v", id, err)
	}

	_, err = NewJWTTokenService("another-secret-with-more-than-32-characters", time.Hour).Verify(token)
	expectToken(t, err, "is invalid")
	_, err = service.Verify("garbage")
	expectToken(t, err, "is invalid")
}

func TestJWTTokenServiceReportsExpiredTokens(t *testing.T) {
	service := NewJWTTokenService(secret, time.Hour)
	expired := NewJWTTokenService(secret, -time.Minute)
	token, _ := expired.Issue(userID)
	_, err := service.Verify(token)
	expectToken(t, err, "has expired")
}

func TestJWTTokenServiceRejectsUnsafeTokens(t *testing.T) {
	service := NewJWTTokenService(secret, time.Hour)
	future := jwt.NewNumericDate(time.Now().Add(time.Hour))

	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Subject: userID, ExpiresAt: future}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	_, err := service.Verify(none)
	expectToken(t, err, "is invalid")

	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.RegisteredClaims{Subject: userID, ExpiresAt: future}).SignedString([]byte(secret))
	_, err = service.Verify(hs512)
	expectToken(t, err, "is invalid")

	badSubject, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "not-a-uuid", ExpiresAt: future}).SignedString([]byte(secret))
	_, err = service.Verify(badSubject)
	expectToken(t, err, "is invalid")

	noSubject, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{ExpiresAt: future}).SignedString([]byte(secret))
	_, err = service.Verify(noSubject)
	expectToken(t, err, "is invalid")

	noExpiry, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: userID}).SignedString([]byte(secret))
	_, err = service.Verify(noExpiry)
	expectToken(t, err, "is invalid")
}
