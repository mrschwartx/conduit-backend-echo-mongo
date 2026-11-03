# Conduit Backend (Echo, MongoDB)

The backend of Conduit, a Medium-style blogging platform built on the RealWorld API contract: sign up and sign in, edit your profile, follow other authors, publish articles with tags, favorite them, read a personal feed and discuss in comments. It is a REST API built with Go, Echo v5 and MongoDB, organised as a modular monolith with Domain-Driven Design and Clean Architecture.

The browser client is the separate Conduit React frontend. The API follows `conduit-openapi/openapi.yml`, the same contract the frontend and the other Conduit backends are written against.

**Stack:** Go 1.27 · Echo v5 · MongoDB 8 · mongo-driver v2 · JWT (golang-jwt) · bcrypt · log/slog · go test · Docker · GitHub Actions

## Contents

- [Quick start](#quick-start)
- [Features](#features)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Database](#database)
- [API](#api)
- [Contract decisions](#contract-decisions)
- [Testing](#testing)
- [CI](#ci)
- [Limits](#limits)
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

Paste the generated value into `AUTH_SECRET` in `.env`. The API runs at http://localhost:8080 and mongo-express at http://localhost:8081 (no login, connected as `dev_user` / `dev_pass`). Migrations run automatically on startup and create the indexes and collection rules. The `.env` file is loaded when present, and real environment variables always take precedence.

| Goal               | Command                                    |
| ------------------ | ------------------------------------------ |
| Run in development | `make run`                                 |
| Build binaries     | `make build` (writes to `bin/`)            |
| Apply migrations   | `make migrate`                             |
| Seed demo data     | `make seed`                                |
| Format code        | `make fmt`                                 |
| Run with Docker    | `make docker/up` (from `conduit-docker`)   |
| Seed in Docker     | `make docker/seed` (from `conduit-docker`) |
| Stop Docker        | `make docker/down` (from `conduit-docker`) |

`make docker/up` creates `conduit-docker/.env` with a random database password and `AUTH_SECRET` on the first run, then starts MongoDB and the API and waits until both are healthy. `PORT` overrides the host port in the Docker setup. The container always listens on port 8080. Demo logins after seeding: `johndoe123@mail.com` / `password123` and `jane@mail.com` / `securepassword456`. Seeding stops quietly when a demo user already exists, so running it twice never duplicates data, and it refuses to run when `APP_LEVEL` is `production` unless `SEED_ALLOW_PRODUCTION` is `true`.

## Features

- **Accounts**: sign up, sign in, read and update the current user (username, email, password, bio, image).
- **Profiles**: public profiles, follow and unfollow.
- **Articles**: create, read, update and delete, with tags, generated slugs, favorites, filtering by tag, author and favoriter, pagination and a feed of followed authors.
- **Comments**: list, add and delete comments on an article.
- **Tags**: the 20 most used tags.
- **Platform**: versioned MongoDB migrations (indexes and collection rules) applied from the binaries, structured logging with daily rotated files, graceful shutdown on `SIGINT` and `SIGTERM`, `GET /health` for health checks.

## Configuration

| Variable                | Default                 | Purpose                                                          |
| ----------------------- | ----------------------- | ---------------------------------------------------------------- |
| `APP_NAME`              | `conduit-api`           | Application name, also the prefix of log files                   |
| `APP_HOST`              | `0.0.0.0`               | Bind address                                                     |
| `APP_PORT`              | `8080`                  | Port the API listens on                                          |
| `APP_LEVEL`             | `development`           | Environment level, `production` switches the console to JSON logs and blocks seeding |
| `DATABASE_URL`          | required                | MongoDB connection string, `mongodb://user:password@host:27017/db?authSource=admin`, the database name is its path and defaults to `conduit` |
| `DATABASE_POOL_MAX`     | `10`                    | Maximum connections in the pool                                  |
| `DATABASE_TIMEOUT`      | `30000`                 | Connect and server selection timeout in milliseconds             |
| `AUTH_SECRET`           | required                | JWT secret, at least 32 characters                               |
| `AUTH_TOKEN_TTL`        | `7d`                    | Lifetime of issued tokens: seconds or `30s`, `15m`, `2h`, `7d`, `2w`, any other value falls back to `7d` |
| `BCRYPT_ROUNDS`         | `10`                    | bcrypt cost factor                                               |
| `CLIENT_URL`            | `http://localhost:3000` | Allowed CORS origin                                              |
| `LOG_LEVEL`             | `debug`                 | Log level, `info` when `APP_LEVEL` is `production`               |
| `LOG_TO_FILE`           | `true`                  | Write logs to files in `LOG_DIR`                                 |
| `LOG_DIR`               | `logs`                  | Directory for log files                                          |
| `LOG_SILENT`            | `false`                 | Disable all logging                                              |
| `SEED_ALLOW_PRODUCTION` | `false`                 | Allows `make seed` when `APP_LEVEL` is `production`              |

Log files are written next to the console output as one JSON file per day, also rotated at 20 MB, compressed and kept for 14 days. The Docker image sets `LOG_TO_FILE=false`, so containers log to the console only.

Docker only, in `conduit-docker/.env`:

| Variable         | Default                 | Purpose                                 |
| ---------------- | ----------------------- | --------------------------------------- |
| `PORT`           | `8080`                  | Host port used by Docker                |
| `DB_NAME`        | `conduit`               | MongoDB database name |
| `DB_USER`        | `conduit`               | MongoDB root user |
| `DB_PASSWORD`    | required                | MongoDB root password, generated by make |
| `AUTH_SECRET`    | required                | JWT secret, generated by make           |
| `AUTH_TOKEN_TTL` | `7d`                    | Lifetime of issued tokens               |
| `APP_LEVEL`      | `development`           | Environment level                       |
| `CLIENT_URL`     | `http://localhost:3000` | Allowed CORS origin                     |
| `LOG_LEVEL`      | `info`                  | Log level                               |

The frontend reaches the API through its own `/api` proxy (the Vite dev server or the nginx image), so no CORS setup is needed for the default stack. The backend still allows `CLIENT_URL` for direct cross-origin use.

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
    shared/      errors, validation, pagination, event bus, logging, HTTP and MongoDB helpers
    modules/     users, articles, comments
  test/
    support/     helpers shared by integration and e2e suites
    integration/ repositories and HTTP modules on a real MongoDB
    e2e/         built binaries run as real processes
conduit-docker/
  conduit-backend-echo/Dockerfile
  docker-compose.local.yml
  docker-compose.dev.yml
  Makefile
conduit-openapi/openapi.yml
.github/workflows/ci.yml
.github/workflows/commitlint.yml
.commitlintrc.json
```

### Modules

| Module     | Owns                                              | Uses from other modules                                  |
| ---------- | ------------------------------------------------- | -------------------------------------------------------- |
| `users`    | users, credentials, tokens, profiles, follows     | none                                                     |
| `articles` | articles, tags, favorites                         | author profiles, followed ids (`users`)                  |
| `comments` | comments                                          | author profiles (`users`), article lookup (`articles`)   |

```
internal/modules/<name>/
  domain/                  entities, value objects, repository contracts
  application/             use cases and ports
  infrastructure/          MongoDB and other adapters
  presentation/rest/       Echo handlers, middleware and routes
  module.go                wires the module and registers its routes
```

### Dependency rules

- Dependencies point inward: presentation and infrastructure depend on application, application depends on domain.
- Modules never import each other. What articles and comments need from users is the `AuthorDirectory` port declared in `shared/application`, which the users module implements. Comments need `ArticleCatalog`, which the articles module implements. The composition root in `bootstrap` wires them.
- Reactions travel through the in-process event bus. Deleting an article publishes `article.deleted`, and `comments` subscribes to remove the article's comments.
- Every I/O call takes a `context.Context`, so client disconnects and shutdown propagate to the database.
- Every write touches a single document, so no multi-document transaction is needed and a standalone MongoDB server is enough. An article keeps its tags and the ids of the users who favorited it inside its own document.

### Request pipeline

1. CORS, then request logging.
2. `JSONBody` for everything under `/api/v1`: a non-JSON content type returns 415, a body above 256 KB returns 413, and a body that is not a JSON object or array returns 400.
3. Authentication per route: required, optional (public reads that personalise flags when a token is sent) or none.
4. The handler reads the envelope (`user`, `article` or `comment`) as raw fields, so an absent field, `null` and a value stay distinguishable.
5. Use cases check in a fixed order: authentication, existence, ownership, then validation.

## Database

Collections and indexes are defined in `internal/shared/infrastructure/mongodb/migrator.go` as named migrations (`0001_create_users` to `0004_create_comments`). The migrator applies pending migrations in order and records them in `schema_migrations`. Index creation is idempotent, so concurrent starts do not race and each migration is recorded once. It runs on every startup and through `make migrate`. The database name comes from the path of `DATABASE_URL`.

| Collection  | Notes                                                                                                   |
| ----------- | ------------------------------------------------------------------------------------------------------- |
| `users`     | `_id` UUID string, `username`, `email`, `passwordHash`, nullable `bio` and `image`, timestamps. Unique indexes on the lowercased `usernameLower` and `emailLower` make both case-insensitive |
| `follows`   | One document per `(followerId, followedId)` with a unique index, a collection rule rejects following yourself |
| `articles`  | `_id` UUID string, `authorId`, unique `slug` (a collection rule rejects `feed`), `title`, `description`, `body`, ordered `tagList`, `favoritedBy` user ids, timestamps, `seq` for stable ordering |
| `comments`  | `_id` integer from a counter, `articleId`, `authorId`, `body`, timestamps                               |
| `counters`  | Atomic sequences behind the article `seq` and the comment ids                                           |

- Article listings are ordered by `createdAt` descending, then `seq` descending, so equal timestamps keep a stable order.
- Deleting an article removes its tags and favorites with the document. Its comments are removed through the `article.deleted` event.
- Timestamps are stored with millisecond precision and returned as ISO 8601 UTC strings, for example `2026-12-08T10:00:00.000Z`.
- Popular tags are ordered by usage, then alphabetically.

## API

All routes are under `/api/v1` and follow `conduit-openapi/openapi.yml`. Protected routes expect `Authorization: Token <jwt>`. Every error body has the shape `{ "errors": { "<field>": ["message"] } }`.

| Method | Path                           | Auth     | Purpose                                    |
| ------ | ------------------------------ | -------- | ------------------------------------------ |
| POST   | `/users`                       | none     | Register, returns the user with a token    |
| POST   | `/users/login`                 | none     | Sign in                                    |
| GET    | `/user`                        | required | Current user with a fresh token            |
| PUT    | `/user`                        | required | Update the current user                    |
| GET    | `/profiles/:username`          | optional | Get a profile                              |
| POST   | `/profiles/:username/follow`   | required | Follow a user                              |
| DELETE | `/profiles/:username/follow`   | required | Unfollow a user                            |
| GET    | `/articles`                    | optional | List with `tag`, `author`, `favorited`, `limit`, `offset` |
| GET    | `/articles/feed`               | required | Articles of followed authors               |
| POST   | `/articles`                    | required | Create an article, returns 201             |
| GET    | `/articles/:slug`              | optional | Get an article                             |
| PUT    | `/articles/:slug`              | required | Update an own article                      |
| DELETE | `/articles/:slug`              | required | Delete an own article, returns 204         |
| POST   | `/articles/:slug/favorite`     | required | Favorite, idempotent                       |
| DELETE | `/articles/:slug/favorite`     | required | Unfavorite, idempotent                     |
| GET    | `/articles/:slug/comments`     | optional | List comments, newest first                |
| POST   | `/articles/:slug/comments`     | required | Add a comment, returns 201                 |
| DELETE | `/articles/:slug/comments/:id` | required | Delete an own comment, returns 204         |
| GET    | `/tags`                        | none     | Up to 20 popular tags                      |

`GET /health` (outside `/api/v1`) returns `200` with the plain text `OK`.

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

## Contract decisions

Where the contract leaves room, the API behaves as follows.

- **Check order** on protected routes: authentication, existence, ownership, then validation.
- **Authentication**: only the `Token` scheme is accepted. Optional-auth routes work anonymously without a header, but a header that is present must be valid. A valid token for a user that no longer exists is rejected.
- **Login** answers `401` identically for an unknown email and a wrong password, and spends comparable time in both cases.
- **Validation** trims text before measuring and counts Unicode characters. Passwords are not trimmed, need 8 to 72 characters and at most 72 bytes because bcrypt ignores the rest. All failing fields are reported together.
- **Registration conflicts** list every taken field (`email`, `username`), matched without regard to case. Emails are stored lowercase.
- **Missing envelope**: a body without the envelope key is treated as empty input, so it fails validation instead of crashing.
- **`PUT /user`** with no fields returns `422` with `user: can't be empty`. `PUT /articles/:slug` with no fields returns the article unchanged.
- **`bio` and `image`** are `null` when unset. Sending `null` or a blank string clears them, and `image` must be an `https` URL.
- **Slugs** come from the title (diacritics removed, lowercase, ASCII letters and digits separated by dashes). Collisions get `-2`, `-3` and so on, `feed` is reserved, and a changed title regenerates the slug, so the previous slug stops resolving.
- **Tags**: at most 10 per article after trimming and de-duplication, case sensitive, order kept. A blank tag is a validation error, and `null` or an empty list clears the tags.
- **Pagination**: `limit` defaults to 20 and is capped at 50, `offset` defaults to 0, and non-integers, `limit` below 1 or `offset` below 0 answer `422`. Unknown `author` or `favorited` usernames return an empty page.
- **Comment ids** that are not integers answer `422`, and ids too large to exist answer `404`.
- **Following yourself** answers `422`. Follow, unfollow, favorite and unfavorite are idempotent.

| Field               | Rule                                              |
| ------------------- | ------------------------------------------------- |
| Request body        | up to 256 KB                                      |
| Username            | 3 to 30 characters, letters, digits, `_` and `-`  |
| Email               | valid format, up to 254 characters                |
| Password            | 8 to 72 characters and at most 72 bytes           |
| Bio                 | up to 300 characters                              |
| Image               | `https` URL, up to 2048 characters                |
| Article title       | up to 100 characters                              |
| Article description | up to 200 characters                              |
| Article body        | up to 20000 characters                            |
| Tags per article    | 10, each 1 to 30 characters without whitespace or commas |
| Comment body        | up to 1000 characters                             |
| Slug                | up to 80 characters                               |
| `limit`             | default 20, maximum 50                            |
| Popular tags        | 20                                                |

## Testing

| Suite       | Command                 | What it covers                                                                      |
| ----------- | ----------------------- | ----------------------------------------------------------------------------------- |
| Unit        | `make test/unit`        | Domain rules, use cases with in-memory fakes, HTTP helpers, config, JWT and bcrypt, run with the race detector |
| Integration | `make test/integration` | Repositories, migrations and every HTTP route on a real MongoDB                 |
| End to end  | `make test/e2e`         | Built binaries run as processes: startup, shutdown, seed and a user journey         |
| Coverage    | `make test/coverage`    | Unit and integration tests with coverage, writes `coverage.out`                     |

Integration and e2e tests create a throwaway database per test through `TEST_DATABASE_URL`, which defaults to the MongoDB server from `make docker/dev-up`. Each test uses its own randomly named database and drops it afterwards. `make test/coverage` writes `coverage.out`. Other checks: `make fmt-check`, `make vet`, `make tidy-check`.

```sh
TEST_DATABASE_URL=mongodb://localhost:27017/ make test
```

## CI

`.github/workflows/ci.yml` runs on pushes to `main` and on pull requests, with three jobs in sequence:

1. **conduit code quality**: `make fmt-check`, `make vet` and `make tidy-check`.
2. **conduit code tests**: unit, integration and end-to-end suites against a MongoDB 8 service.
3. **conduit docker container**: builds the image, starts the Compose stack, then checks `/health`, registers a user, runs the seed and reads `/api/v1/articles`.

`.github/workflows/commitlint.yml` is a separate workflow that lints commit messages on pull requests, configured in `.commitlintrc.json`.

## Limits

- The API needs a running MongoDB. A standalone server is enough.
- Profile images are URLs. The API does not store or serve uploads.
- Events are delivered in-process, so the app runs as a single instance.
- There is no account deletion or token revocation. Tokens simply expire after `AUTH_TOKEN_TTL`.
- There are no foreign keys. Deleting an article removes its comments through the `article.deleted` event, so a failure in that handler can leave comments behind.
- Favorites live inside the article document, which is fast for normal use but bounded by the 16 MB document limit.
- Data from the PostgreSQL version or from the former Taskly version is not migrated. Start with an empty database.

## References

- [RealWorld API specification](https://realworld-docs.netlify.app)
- [Echo](https://echo.labstack.com) · [MongoDB Go driver](https://github.com/mongodb/mongo-go-driver) · [golang-jwt](https://github.com/golang-jwt/jwt)
- [Go testing](https://pkg.go.dev/testing) · [log/slog](https://pkg.go.dev/log/slog)
- [MongoDB manual](https://www.mongodb.com/docs/manual/)
- [Docker multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
