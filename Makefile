# ── FreeRouter ────────────────────────────────────────────────────────
.PHONY: build dev test test-race lint vet fmt up down logs migrate setup init init-iam reset clean jwt-key encryption-key iamkit-logs bootstrap tidy

# ── Build ────────────────────────────────────────────────────────────
build:
	go build -o bin/server ./cmd/server

dev: build
	@set -a; [ -f .env ] && . ./.env; set +a; ./bin/server

# ── Quality ──────────────────────────────────────────────────────────
vet:
	go vet ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

test:
	go test -v ./...

test-race:
	go test -race -v ./...

# ── Docker ───────────────────────────────────────────────────────────
# The iamkit service runs only in iam mode: enable its compose profile when
# .env selects FREEROUTER_AUTH=iam, or leaves it unset with IAMKit configured
# (pre-auth-modes .env files), unless COMPOSE_PROFILES is already set.
COMPOSE = COMPOSE_PROFILES="$${COMPOSE_PROFILES:-$$(./scripts/compose-profiles.sh)}" docker compose

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

# ── Database ─────────────────────────────────────────────────────────
migrate:
	@for f in migrations/*.up.sql; do \
		echo "Running $$f..."; \
		PGPASSWORD=$${DB_PASSWORD:-freerouter} psql -h $${DB_HOST:-localhost} -p $${DB_PORT:-5432} \
			-U $${DB_USER:-freerouter} -d $${DB_NAME:-freerouter} -f "$$f"; \
	done

# ── Secrets ──────────────────────────────────────────────────────────
jwt-key:
	mkdir -p secrets && openssl genrsa -out secrets/jwt.pem 4096
	@echo "JWT key generated at secrets/jwt.pem"

encryption-key:
	@echo "ENCRYPTION_KEY=$$(openssl rand -hex 32)"
	@echo "# Add the line above to your .env file"

iamkit-logs:
	COMPOSE_PROFILES=iam docker compose logs -f iamkit

# Provision IAMKit (workspace owner, project, environment, app, resource,
# backend service account) and write the ids to .env. Refuses to run twice.
bootstrap:
	@./scripts/bootstrap-iamkit.sh

# ── Setup ────────────────────────────────────────────────────────────
# Create .env from .env.example with fresh local secrets (never overwrites).
# FREEROUTER_ADMIN_TOKEN is the console/admin login in token mode.
setup:
	@if [ -f .env ]; then echo ".env exists; leaving it alone"; else \
		sed -e "s|^ENCRYPTION_KEY=.*|ENCRYPTION_KEY=$$(openssl rand -hex 32)|" \
			-e "s|^FREEROUTER_ADMIN_TOKEN=.*|FREEROUTER_ADMIN_TOKEN=$$(openssl rand -hex 32)|" \
			-e "s|^OIDC_HMAC_SECRET=.*|OIDC_HMAC_SECRET=$$(openssl rand -hex 32)|" \
			-e "s|^IAMKIT_ENCRYPTION_KEY=.*|IAMKIT_ENCRYPTION_KEY=$$(openssl rand -base64 32)|" \
			.env.example > .env && chmod 600 .env && echo ".env created with fresh secrets"; fi
	@[ -f secrets/jwt.pem ] || $(MAKE) jwt-key

# Token mode (default): Postgres + Redis, no IAMKit. Sign in to the console
# with FREEROUTER_ADMIN_TOKEN from .env. An .env already set up for IAMKit
# (FREEROUTER_AUTH=iam, or IAMKit ids from before auth modes) stays in iam.
init: setup
	@if [ "$$(./scripts/compose-profiles.sh)" != iam ]; then \
		./scripts/env-set.sh FREEROUTER_AUTH token --if-empty; \
		./scripts/env-set.sh FREEROUTER_ADMIN_TOKEN "$$(openssl rand -hex 32)" --if-empty; fi
	$(COMPOSE) up -d --wait
	$(MAKE) migrate
	@echo ""
	@echo "✅ FreeRouter initialized (FREEROUTER_AUTH=$$(sed -n 's/^FREEROUTER_AUTH=//p' .env)) — run: make dev"
	@[ "$$(./scripts/compose-profiles.sh)" = iam ] || echo "   Console login: FREEROUTER_ADMIN_TOKEN in .env"

# IAMKit mode: also builds and provisions the IAMKit sidecar (users, roles,
# service accounts). Writes FREEROUTER_AUTH=iam to .env.
init-iam: setup
	@./scripts/env-set.sh FREEROUTER_AUTH iam
	COMPOSE_PROFILES=iam docker compose up -d --build --wait
	$(MAKE) migrate
	$(MAKE) bootstrap
	@echo ""
	@echo "✅ FreeRouter initialized (FREEROUTER_AUTH=iam) — run: make dev"

# Fresh stack: drop containers, volumes (both databases), IAMKit credentials
# and the provisioned ids. IAMKit's schema has no upgrade path between
# squashed migrations, so this is also how to move to a newer IAMKit.
reset:
	COMPOSE_PROFILES=iam docker compose down -v
	rm -rf .dev-secrets
	@[ -f .env ] && awk '!/^IAMKIT_(AUDIENCE|ENVIRONMENT_ID|APPLICATION_ID|RESOURCE_ID|ORGANIZATION_ID|SERVICE_SECRET)=/' .env > .env.tmp && mv .env.tmp .env && chmod 600 .env || true
	@echo "Stack reset — run: make init (token) or make init-iam"

tidy:
	go mod tidy

# ── Clean ────────────────────────────────────────────────────────────
clean:
	rm -rf bin/
	COMPOSE_PROFILES=iam docker compose down -v
