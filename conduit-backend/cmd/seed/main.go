package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"conduit/backend/internal/bootstrap"
	"conduit/backend/internal/config"
	articlesapp "conduit/backend/internal/modules/articles/application"
	usersapp "conduit/backend/internal/modules/users/application"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/logging"
)

type seedUser struct {
	username string
	email    string
	password string
}

type seedComment struct {
	author string
	body   string
}

type seedArticle struct {
	author      string
	title       string
	description string
	body        string
	tagList     []string
	comments    []seedComment
	favoritedBy []string
}

var users = []seedUser{
	{username: "johndoe123", email: "johndoe123@mail.com", password: "password123"},
	{username: "jane78", email: "jane@mail.com", password: "securepassword456"},
}

var articles = []seedArticle{
	{
		author:      "johndoe123",
		title:       "Getting started with Conduit",
		description: "A first look at the Conduit publishing platform",
		body:        "Conduit is a Medium-style blogging platform built on the RealWorld API contract.",
		tagList:     []string{"conduit", "welcome"},
		comments:    []seedComment{{author: "jane78", body: "Great introduction, thanks!"}},
		favoritedBy: []string{"jane78"},
	},
	{
		author:      "jane78",
		title:       "Building a modular monolith",
		description: "Why bounded contexts keep a codebase tidy",
		body:        "Modules own their data and expose small public APIs to each other.",
		tagList:     []string{"architecture", "welcome"},
	},
}

func main() {
	os.Exit(run())
}

func run() int {
	_ = godotenv.Load()
	ctx := context.Background()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
		return 1
	}
	logger, closeLogger, err := logging.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
		return 1
	}
	defer closeLogger()

	if cfg.App.IsProduction && os.Getenv("SEED_ALLOW_PRODUCTION") != "true" {
		logger.Error("Refusing to seed a production database")
		fmt.Fprintln(os.Stderr, "Refusing to seed a production database")
		return 1
	}

	container, err := bootstrap.NewContainer(ctx, cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
		return 1
	}
	defer container.Close()

	ids := map[string]string{}
	for _, entry := range users {
		id, err := seedAccount(ctx, container, entry)
		if err != nil {
			if sharedomain.IsKind(err, sharedomain.KindConflict) {
				logger.Warn("Skipped existing user " + entry.username)
				return 0
			}
			fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
			return 1
		}
		ids[entry.username] = id
		logger.Info("Seeded user " + entry.username)
	}

	if err := seedContent(ctx, container, ids); err != nil {
		fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
		return 1
	}
	logger.Info("Seeding completed")
	return 0
}

func seedAccount(ctx context.Context, container *bootstrap.Container, entry seedUser) (string, error) {
	registered, err := container.Users.UseCases.RegisterUser.Execute(ctx, usersapp.RegisterUserInput{
		Username: sharedomain.RawString(entry.username),
		Email:    sharedomain.RawString(entry.email),
		Password: sharedomain.RawString(entry.password),
	})
	if err != nil {
		return "", err
	}
	return container.Users.UseCases.VerifyAccessToken.Execute(ctx, registered.Token)
}

func seedContent(ctx context.Context, container *bootstrap.Container, ids map[string]string) error {
	if _, err := container.Users.UseCases.FollowUser.Execute(ctx, ids["jane78"], "johndoe123"); err != nil {
		return err
	}
	for _, entry := range articles {
		tags := make([]any, 0, len(entry.tagList))
		for _, tag := range entry.tagList {
			tags = append(tags, tag)
		}
		created, err := container.Articles.UseCases.CreateArticle.Execute(ctx, ids[entry.author], articlesapp.ArticleInput{
			Title:       sharedomain.RawString(entry.title),
			Description: sharedomain.RawString(entry.description),
			Body:        sharedomain.RawString(entry.body),
			TagList:     sharedomain.RawOf(tags),
		})
		if err != nil {
			return err
		}
		for _, comment := range entry.comments {
			if _, err := container.Comments.UseCases.CreateComment.Execute(ctx, ids[comment.author], created.Slug, sharedomain.RawString(comment.body)); err != nil {
				return err
			}
		}
		for _, username := range entry.favoritedBy {
			if _, err := container.Articles.UseCases.FavoriteArticle.Execute(ctx, ids[username], created.Slug); err != nil {
				return err
			}
		}
	}
	return nil
}
