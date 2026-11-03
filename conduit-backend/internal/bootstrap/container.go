package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"conduit/backend/internal/config"
	"conduit/backend/internal/modules/articles"
	"conduit/backend/internal/modules/comments"
	"conduit/backend/internal/modules/users"
	"conduit/backend/internal/shared/infrastructure/events"
	"conduit/backend/internal/shared/infrastructure/mongodb"
)

const closeTimeout = 5 * time.Second

type Container struct {
	Config   config.Config
	Logger   *slog.Logger
	Database *mongo.Database
	Events   *events.InMemoryBus
	Users    *users.Module
	Articles *articles.Module
	Comments *comments.Module
}

func NewContainer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Container, error) {
	db, err := mongodb.Connect(ctx, mongodb.ConnectionConfig{
		URL:     cfg.Database.URL,
		PoolMax: cfg.Database.PoolMax,
		Timeout: cfg.Database.Timeout,
	})
	if err != nil {
		return nil, err
	}

	if _, err := mongodb.RunMigrations(ctx, db, mongodb.Migrations(), logger); err != nil {
		_ = mongodb.Close(context.WithoutCancel(ctx), db)
		return nil, err
	}

	bus := events.NewInMemoryBus(logger)
	usersModule := users.NewModule(users.Dependencies{DB: db, Config: cfg})
	articlesModule := articles.NewModule(articles.Dependencies{
		DB:                   db,
		Events:               bus,
		Authors:              usersModule.AuthorDirectory,
		Authenticate:         usersModule.Authenticate,
		OptionalAuthenticate: usersModule.OptionalAuthenticate,
	})
	commentsModule := comments.NewModule(comments.Dependencies{
		DB:                   db,
		Events:               bus,
		Authors:              usersModule.AuthorDirectory,
		Articles:             articlesModule.Catalog,
		Authenticate:         usersModule.Authenticate,
		OptionalAuthenticate: usersModule.OptionalAuthenticate,
	})

	return &Container{
		Config:   cfg,
		Logger:   logger,
		Database: db,
		Events:   bus,
		Users:    usersModule,
		Articles: articlesModule,
		Comments: commentsModule,
	}, nil
}

func (c *Container) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	_ = mongodb.Close(ctx, c.Database)
}
