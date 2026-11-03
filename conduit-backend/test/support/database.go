package support

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"conduit/backend/internal/shared/infrastructure/mongodb"
)

const (
	DefaultTestDatabaseURL = "mongodb://dev_user:dev_pass@localhost:27017/?authSource=admin"
	databaseTimeout        = 10 * time.Second
)

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
	parsed, err := url.Parse(adminURL)
	if err != nil {
		tb.Fatalf("invalid TEST_DATABASE_URL: %v", err)
	}

	name := RandomDatabaseName()
	parsed.Path = "/" + name
	query := parsed.Query()
	if parsed.User != nil && query.Get("authSource") == "" {
		query.Set("authSource", "admin")
	}
	parsed.RawQuery = query.Encode()
	database := &TestDatabase{Name: name, URL: parsed.String()}

	db := ConnectTestDatabase(tb, database.URL)
	tb.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), databaseTimeout)
		defer cancel()
		if err := db.Drop(ctx); err != nil {
			tb.Errorf("cannot drop the test database: %v", err)
		}
	})
	return database
}

func ConnectTestDatabase(tb testing.TB, databaseURL string) *mongo.Database {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), databaseTimeout)
	defer cancel()

	db, err := mongodb.Connect(ctx, mongodb.ConnectionConfig{URL: databaseURL, PoolMax: 4, Timeout: databaseTimeout})
	if err != nil {
		tb.Fatalf("cannot connect to the test MongoDB server: %v", err)
	}
	tb.Cleanup(func() {
		if err := mongodb.Close(context.Background(), db); err != nil {
			tb.Errorf("cannot close the test connection: %v", err)
		}
	})
	return db
}

func RandomDatabaseName() string {
	suffix := make([]byte, 5)
	if _, err := rand.Read(suffix); err != nil {
		panic(err)
	}
	return "conduit_test_" + hex.EncodeToString(suffix)
}
