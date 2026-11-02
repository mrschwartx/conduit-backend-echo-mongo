# Conduit Backend (Echo)

A Medium-style blogging platform backend that implements the [RealWorld](https://realworld-docs.netlify.app/) "Conduit" API contract: register and sign in, manage a profile, follow authors, publish articles with tags, favorite articles and comment on them. It is a REST API built with Go, Echo v5 and PostgreSQL, organised as a modular monolith with Domain-Driven Design and Clean Architecture.

The browser client is the separate Conduit React frontend. This backend is interchangeable with the Express implementation because both follow the same OpenAPI contract in `conduit-openapi/openapi.yml`.

**Stack:** Go 1.27 · Echo v5 · PostgreSQL 18 · pgx · JWT · bcrypt · log/slog · testing · Docker · GitHub Actions

## Contents

- [Quick start](#quick-start)
- [Features](#features)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Database](#database)
- [API](#api)
- [Testing](#testing)
- [CI](#ci)
- [Limits](#limits)
- [Migrated from Taskly](#migrated-from-taskly)
- [References](#references)

## Quick start

Requires Go 1.27+ and Docker.

```sh
cd conduit-docker
make docker/dev-up
cd ../conduit-backend
cp .env.example .env
make secret-key
make run
```

Paste the generated value into `AUTH_SECRET` in `.env`. The API runs at http://localhost:8080 and pgAdmin at http://localhost:8081 (login `admin@conduit.dev` / `admin`, server host `conduit-postgres`, user `dev_user`, password `dev_pass`). Migrations run automatically on startup. The `.env` file is loaded when present, and real environment variables always take precedence.

| Goal               | Command                                    |
| ------------------ | ------------------------------------------ |
| Run in development | `make run`                                 |
| Build binaries     | `make build` (writes to `bin/`)            |
| Apply migrations   | `make migrate`                             |
| Seed demo data     | `make seed`                                |
| Run with Docker    | `make docker/up` (from `conduit-docker`)   |
| Seed in Docker     | `make docker/seed` (from `conduit-docker`) |
| Stop Docker        | `make docker/down` (from `conduit-docker`) |

`make docker/up` creates `conduit-docker/.env` with a random database password and `AUTH_SECRET` on the first run, then starts PostgreSQL and the API and waits until both are healthy. `PORT` overrides the host port in the Docker setup. The container always listens on port 8080.

Demo logins after seeding: `johndoe123@mail.com` / `password123` and `jane@mail.com` / `securepassword456`. Seeding stops quietly when the first demo user already exists, so running it twice never duplicates data.

## Features

- **Authentication**: register and log in, receive a JWT, send it as `Authorization: Token <jwt>`.
- **Users**: read and update the current user (username, email, password, bio, image).
- **Profiles**: view a profile, follow and unfollow authors.
- **Articles**: create, read, update and delete articles; slugs are generated from the title and kept unique.
- **Listing**: filter by tag, author and favorited user, paginate with `limit` and `offset`, and read a personal feed of followed authors.
- **Favorites and tags**: favorite and unfavorite articles, list the most used tags.
- **Comments**: create, list and delete comments; deleting an article removes its comments.
- **Platform**: versioned SQL migrations embedded in the binary, structured logging with daily rotation, graceful shutdown on `SIGINT` and `SIGTERM`, health check on `/health`.

## Configuration

| Variable                | Default                 | Purpose                                              |
| ----------------------- | ----------------------- | ---------------------------------------------------- |
| `APP_NAME`              | `conduit-api`           | Application name                                     |
| `APP_HOST`              | `0.0.0.0`               | Bind address                                         |
| `APP_PORT`              | `8080`                  | Port the API listens on                              |
| `APP_LEVEL`             | `development`           | Environment level                                    |
| `DATABASE_URL`          | required                | PostgreSQL connection string                         |
| `DATABASE_POOL_MAX`     | `10`                    | Maximum connections in the pool                      |
| `DATABASE_TIMEOUT`      | `30000`                 | Connection timeout in milliseconds                   |
| `AUTH_SECRET`           | required                | JWT secret, at least 32 characters                   |
| `AUTH_TOKEN_TTL`        | `7d`                    | Token lifetime: a number of seconds or `30s`, `15m`, `2h`, `7d`, `2w` |
| `BCRYPT_ROUNDS`         | `10`                    | bcrypt cost factor                                   |
| `CLIENT_URL`            | `http://localhost:3000` | Allowed CORS origin                                  |
| `LOG_LEVEL`             | `debug`                 | Log level (`info` in production)                     |
| `LOG_TO_FILE`           | `true`                  | Write logs to the `logs` directory                   |
| `LOG_DIR`               | `logs`                  | Directory for log files                              |
| `LOG_SILENT`            | `false`                 | Disable all logging                                  |
| `SEED_ALLOW_PRODUCTION` | `false`                 | Allow `seed` when `APP_LEVEL=production`             |

Docker only, in `conduit-docker/.env`:

| Variable         | Default   | Purpose                                 |
| ---------------- | --------- | --------------------------------------- |
| `PORT`           | `8080`    | Host port used by Docker                |
| `DB_NAME`        | `conduit` | PostgreSQL database name                |
| `DB_USER`        | `conduit` | PostgreSQL user                         |
| `DB_PASSWORD`    | required  | PostgreSQL password, generated by make  |
| `AUTH_TOKEN_TTL` | `7d`      | Token lifetime                          |

The frontend must set its API base URL to `http://localhost:8080/api/v1`.

## Architecture

### Project layout

```
conduit-backend/
  cmd/
    backend/     HTTP server entrypoint
    migrate/     applies pending migrations
    seed/        demo data
  internal/
    bootstrap/   composition root, app and server factories
    config/      environment parsing and validation
    shared/      errors, validation, pagination, event bus, logging, HTTP and PostgreSQL helpers
    modules/     users, articles, comments
  test/
    support/     helpers shared by integration and e2e suites
    integration/ repositories and HTTP modules on a real PostgreSQL
    e2e/         built binaries run as real processes
conduit-docker/
  conduit-backend-echo/Dockerfile
  docker-compose.local.yml
  docker-compose.dev.yml
  Makefile
conduit-openapi/
  openapi.yml
.github/workflows/ci.yml
```

### Module layout

```
internal/modules/<name>/
  domain/                  entities, value objects, repository contracts
  application/             use cases and ports
  infrastructure/          PostgreSQL and other adapters
  presentation/rest/       Echo handlers, middleware and routes
  module.go                wires the module and registers its routes
```

### Dependency rules

- Dependencies point inward: presentation and infrastructure depend on application, application depends on domain.
- Modules never import each other. What articles and comments need from users is the `AuthorDirectory` port declared in `shared/application`, which the users module implements. Comments need `ArticleCatalog`, which the articles module implements. The composition root in `bootstrap` wires them.
- Modules also communicate through the in-process event bus: deleting an article publishes `article.deleted` and the comments module removes that article's comments.
- Every I/O call takes a `context.Context`, so client disconnects and shutdown propagate to the database.
- Multi-row writes (an article with its tags) run in a transaction through `postgres.WithTransaction`.

### Request pipeline

1. CORS, then request logging.
2. `JSONBody` for everything under `/api/v1`: non-JSON content type returns 415, a body above 256 KB returns 413, and a body that is not a JSON object or array returns 400.
3. Authentication per route: required, optional (public reads that personalise flags when a token is sent) or none.
4. The handler reads the envelope (`user`, `article` or `comment`) as raw fields, so "absent", `null` and a value stay distinguishable.
5. Use cases check in a fixed order: authentication, existence, ownership, then validation.

## Database

Schema changes live in `internal/shared/infrastructure/postgres/migrations` as numbered `.sql` files and are embedded in the binaries. The migrator applies pending files in order, each inside a transaction, records them in `schema_migrations` and holds an advisory lock so concurrent starts do not race. It runs on every startup and through `make migrate`.

| Table          | Notes                                                                                           |
| -------------- | ----------------------------------------------------------------------------------------------- |
| `users`        | `id` UUID, case-insensitive unique `email` and `username`, `password_hash`, nullable `bio` and `image`, timestamps |
| `follows`      | `(follower_id, followed_id)` primary key, both reference `users` with `ON DELETE CASCADE`, a user cannot follow themselves |
| `articles`     | `id` UUID, `author_id`, unique `slug`, `title`, `description`, `body`, timestamps, `seq` for stable ordering |
| `article_tags` | `(article_id, position)` primary key keeps tag order, indexed on `tag`                          |
| `favorites`    | `(user_id, article_id)` primary key                                                              |
| `comments`     | `id` bigint identity, `article_id`, `author_id`, `body`, timestamps                              |

- Listings are ordered by `created_at` and `seq`, newest first.
- Deleting an article cascades to its tags, favorites and comments.
- Timestamps are returned as ISO 8601 UTC strings with milliseconds, for example `2026-12-08T10:00:00.000Z`.

## API

Base path: `/api/v1`. The reference is `conduit-openapi/openapi.yml`. Authenticated routes expect `Authorization: Token <jwt>`.

| Method | Path                               | Auth     | Purpose                            |
| ------ | ---------------------------------- | -------- | ---------------------------------- |
| POST   | `/users`                           | none     | Register, returns 201              |
| POST   | `/users/login`                     | none     | Log in                             |
| GET    | `/user`                            | required | Current user                       |
| PUT    | `/user`                            | required | Update the current user            |
| GET    | `/profiles/:username`              | optional | Profile                            |
| POST   | `/profiles/:username/follow`       | required | Follow                             |
| DELETE | `/profiles/:username/follow`       | required | Unfollow                           |
| GET    | `/articles`                        | optional | List: `tag`, `author`, `favorited`, `limit`, `offset` |
| POST   | `/articles`                        | required | Create, returns 201                |
| GET    | `/articles/feed`                   | required | Articles of followed authors       |
| GET    | `/articles/:slug`                  | optional | Read                               |
| PUT    | `/articles/:slug`                  | required | Update, author only                |
| DELETE | `/articles/:slug`                  | required | Delete, author only, returns 204   |
| POST   | `/articles/:slug/favorite`         | required | Favorite                           |
| DELETE | `/articles/:slug/favorite`         | required | Unfavorite                         |
| GET    | `/articles/:slug/comments`         | optional | List comments, newest first        |
| POST   | `/articles/:slug/comments`         | required | Create comment, returns 201        |
| DELETE | `/articles/:slug/comments/:id`     | required | Delete comment, author only, 204   |
| GET    | `/tags`                            | none     | Most used tags                     |
| GET    | `/health`                          | none     | Plain `OK` for health checks       |

Errors always use one shape: `{ "errors": { "<field>": ["<message>"] } }`.

| Status | Meaning                                                                                      |
| ------ | -------------------------------------------------------------------------------------------- |
| 400    | Body is not a JSON object or array: `body: is not valid JSON`                                |
| 401    | `token: is missing`, `is invalid` or `has expired`; `credentials: are invalid` on login      |
| 403    | `article: forbidden` or `comment: forbidden` when you are not the author                     |
| 404    | `article`, `comment`, `profile` or `path`: `not found`                                       |
| 409    | `email` or `username`: `has already been taken`                                              |
| 413    | `body: is too large`                                                                         |
| 415    | `content-type: must be application/json`                                                     |
| 422    | Validation, for example `title: can't be blank`; also bad `limit`, `offset` or comment `id`  |
| 500    | `server: internal server error`, details are only written to the log                         |

Contract decisions:

- Routes without an envelope key in the body are treated as empty input, so they fail validation instead of crashing.
- `PUT /user` with no fields returns 422 `user: can't be empty`. `PUT /articles/:slug` with no fields returns the unchanged article.
- Changing an article title re-generates the slug. The previous slug stops resolving.
- `bio` and `image` are `null` when unset, and sending `null` or a blank string clears them.
- A malformed `Authorization` header, an invalid token or a token of a deleted user returns 401 even on public routes.
- `feed` is a reserved slug and is never assigned to an article.
- A comment id that is not a plain integer returns 422, an id that is too large or unknown returns 404.

## Testing

| Suite       | Command                | What it covers                                                       |
| ----------- | ---------------------- | -------------------------------------------------------------------- |
| Unit        | `make test/unit`       | Domain rules, use cases with in-memory fakes, HTTP helpers, config, JWT and bcrypt |
| Integration | `make test/integration`| Repositories, migrations and every HTTP route on a real PostgreSQL   |
| End to end  | `make test/e2e`        | Built binaries run as processes: startup, shutdown, seed and a user journey |

Integration and e2e tests create a throwaway database per test through `TEST_DATABASE_URL`, which defaults to the development database from `docker-compose.dev.yml`. `make test/coverage` writes `coverage.out`.

## CI

`.github/workflows/ci.yml` runs formatting, `go vet` and `go mod tidy -diff`, then all three test suites against a PostgreSQL service, then builds the Docker stack, checks `/health`, registers a user and runs the seed.

## Limits

| Item                  | Limit                                           |
| --------------------- | ----------------------------------------------- |
| Request body          | 256 KB                                          |
| Username              | 3 to 30 characters, letters, digits, `_` and `-` |
| Email                 | up to 254 characters                            |
| Password              | 8 to 72 characters and at most 72 bytes         |
| Bio                   | up to 300 characters                            |
| Image                 | `https` URL, up to 2048 characters              |
| Article title         | up to 100 characters                            |
| Article description   | up to 200 characters                            |
| Article body          | up to 20000 characters                          |
| Tags per article      | 10, each up to 30 characters without spaces or commas |
| Comment body          | up to 1000 characters                           |
| Slug                  | up to 80 characters                             |
| `limit`               | default 20, maximum 50                          |
| Popular tags          | 20                                              |

## Migrated from Taskly

This codebase used to be a task manager. The Taskly modules, routes and cookie authentication were replaced by Conduit equivalents:

- The `tasks` module became `articles` and `comments`, and the avatar upload, account deletion and user listing were removed.
- Authentication moved from an HTTP-only cookie to the `Authorization: Token` header.
- Errors changed from `{ "message": ... }` to `{ "errors": { ... } }`.
- Data from the Taskly schema is not migrated. Start with an empty database.

## References

- [RealWorld API spec](https://realworld-docs.netlify.app/)
- `conduit-openapi/openapi.yml`
- [Echo](https://echo.labstack.com/) · [pgx](https://github.com/jackc/pgx) · [golang-jwt](https://github.com/golang-jwt/jwt)
