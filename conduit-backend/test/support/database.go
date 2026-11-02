package support

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

const DefaultTestDatabaseURL = "postgres://dev_user:dev_pass@localhost:5432/conduit_dev"

type TestDatabase struct {
	Name string
	URL  string
}

func StartTestDatabase(tb testing.TB) *TestDatabase {
	tb.Helper()

	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		adminURL = DefaultTestDatabaseURL
	}
	name := RandomDatabaseName()

	withAdminConn(tb, adminURL, func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
		return err
	})
	tb.Cleanup(func() {
		withAdminConn(tb, adminURL, func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
			return err
		})
	})

	parsed, err := url.Parse(adminURL)
	if err != nil {
		tb.Fatalf("invalid TEST_DATABASE_URL: %v", err)
	}
	parsed.Path = "/" + name
	return &TestDatabase{Name: name, URL: parsed.String()}
}

func RandomDatabaseName() string {
	suffix := make([]byte, 5)
	if _, err := rand.Read(suffix); err != nil {
		panic(err)
	}
	return "conduit_test_" + hex.EncodeToString(suffix)
}

func withAdminConn(tb testing.TB, adminURL string, run func(context.Context, *pgx.Conn) error) {
	tb.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		tb.Fatalf("cannot connect to the test PostgreSQL server: %v", err)
	}
	defer conn.Close(ctx)
	if err := run(ctx, conn); err != nil {
		tb.Fatalf("admin statement failed: %v", err)
	}
}
