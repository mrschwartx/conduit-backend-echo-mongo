package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

const defaultDatabaseName = "conduit"

type ConnectionConfig struct {
	URL     string
	PoolMax int
	Timeout time.Duration
}

func Connect(ctx context.Context, cfg ConnectionConfig) (*mongo.Database, error) {
	parsed, err := connstring.ParseAndValidate(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	name := parsed.Database
	if name == "" {
		name = defaultDatabaseName
	}

	clientOptions := options.Client().ApplyURI(cfg.URL)
	if cfg.PoolMax > 0 {
		clientOptions.SetMaxPoolSize(uint64(cfg.PoolMax))
	}
	if cfg.Timeout > 0 {
		clientOptions.SetConnectTimeout(cfg.Timeout).SetServerSelectionTimeout(cfg.Timeout)
	}

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	pingCtx := ctx
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.WithoutCancel(ctx))
		return nil, fmt.Errorf("database connection failed: %w", err)
	}
	return client.Database(name), nil
}

func Close(ctx context.Context, db *mongo.Database) error {
	return db.Client().Disconnect(ctx)
}
