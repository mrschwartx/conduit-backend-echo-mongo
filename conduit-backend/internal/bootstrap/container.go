package bootstrap

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"conduit/backend/internal/config"
	"conduit/backend/internal/modules/articles"
	"conduit/backend/internal/modules/comments"
	"conduit/backend/internal/modules/users"
	"conduit/backend/internal/shared/infrastructure/events"
	"conduit/backend/internal/shared/infrastructure/postgres"
)

type Container struct {
	Config   config.Config
	Logger   *slog.Logger
	Pool     *pgxpool.Pool
	Events   *events.InMemoryBus
	Users    *users.Module
	Articles *articles.Module
	Comments *comments.Module
}

func NewContainer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Container, error) {
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{
		URL:     cfg.Database.URL,
		PoolMax: cfg.Database.PoolMax,
		Timeout: cfg.Database.Timeout,
	})
	if err != nil {
		return nil, err
	}

	if _, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), logger); err != nil {
		pool.Close()
		return nil, err
	}

	bus := events.NewInMemoryBus(logger)
	usersModule := users.NewModule(users.Dependencies{DB: pool, Config: cfg})
	articlesModule := articles.NewModule(articles.Dependencies{
		DB:                   pool,
		Events:               bus,
		Authors:              usersModule.AuthorDirectory,
		Authenticate:         usersModule.Authenticate,
		OptionalAuthenticate: usersModule.OptionalAuthenticate,
	})
	commentsModule := comments.NewModule(comments.Dependencies{
		DB:                   pool,
		Events:               bus,
		Authors:              usersModule.AuthorDirectory,
		Articles:             articlesModule.Catalog,
		Authenticate:         usersModule.Authenticate,
		OptionalAuthenticate: usersModule.OptionalAuthenticate,
	})

	return &Container{
		Config:   cfg,
		Logger:   logger,
		Pool:     pool,
		Events:   bus,
		Users:    usersModule,
		Articles: articlesModule,
		Comments: commentsModule,
	}, nil
}

func (c *Container) Close() {
	c.Pool.Close()
}
