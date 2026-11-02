COMPOSE = docker compose -f docker-compose.local.yml
COMPOSE_DEV = docker compose -f docker-compose.dev.yml

.PHONY: docker/env docker/up docker/down docker/logs docker/ps docker/seed docker/clean docker/dev-up docker/dev-down docker/dev-logs

docker/env:
	@test -f .env || { \
		sed -e "s|^DB_PASSWORD=.*|DB_PASSWORD=$$(openssl rand -hex 16)|" \
			-e "s|^AUTH_SECRET=.*|AUTH_SECRET=$$(openssl rand -hex 32)|" \
			.env.example > .env && echo "Created .env with generated secrets"; \
	}

docker/up: docker/env
	$(COMPOSE) up -d --build --wait --wait-timeout 180

docker/down:
	$(COMPOSE) down

docker/logs:
	$(COMPOSE) logs -f conduit-backend

docker/ps:
	$(COMPOSE) ps

docker/seed:
	$(COMPOSE) exec conduit-backend /app/seed

docker/clean:
	$(COMPOSE) down -v --rmi local

docker/dev-up:
	$(COMPOSE_DEV) up -d --wait

docker/dev-down:
	$(COMPOSE_DEV) down

docker/dev-logs:
	$(COMPOSE_DEV) logs -f
