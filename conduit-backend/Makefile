GO ?= go
TEST_DATABASE_URL ?= postgres://dev_user:dev_pass@localhost:5432/conduit_dev

.PHONY: run build migrate seed fmt fmt-check vet tidy-check test test/unit test/integration test/e2e test/coverage secret-key

run:
	$(GO) run ./cmd/backend

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/ ./cmd/backend ./cmd/migrate ./cmd/seed

migrate:
	$(GO) run ./cmd/migrate

seed:
	$(GO) run ./cmd/seed

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

vet:
	$(GO) vet ./...
	$(GO) vet -tags=integration ./test/...
	$(GO) vet -tags=e2e ./test/...

tidy-check:
	$(GO) mod tidy -diff

test: test/unit test/integration test/e2e

test/unit:
	$(GO) test -race -count=1 ./...

test/integration:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) $(GO) test -tags=integration -p 1 -count=1 ./test/integration/...

test/e2e:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) $(GO) test -tags=e2e -p 1 -count=1 ./test/e2e/...

test/coverage:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) $(GO) test -tags=integration -p 1 -count=1 -coverpkg=./internal/... -coverprofile=coverage.out ./internal/... ./test/integration/...
	$(GO) tool cover -func=coverage.out | tail -n 1

secret-key:
	@openssl rand -hex 32
