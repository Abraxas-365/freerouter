# ── FreeRouter ────────────────────────────────────────────────────────
.PHONY: build dev test test-race lint vet fmt up down logs migrate setup init reset clean jwt-key encryption-key iamkit-logs bootstrap tidy

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
up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

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
	docker compose logs -f iamkit

# Provision IAMKit (workspace owner, project, environment, app, resource,
# backend service account) and write the ids to .env. Refuses to run twice.
bootstrap:
	@./scripts/bootstrap-iamkit.sh

# ── Setup ────────────────────────────────────────────────────────────
# Create .env from .env.example with fresh local secrets (never overwrites).
setup:
	@if [ -f .env ]; then echo ".env exists; leaving it alone"; else \
		sed -e "s|^ENCRYPTION_KEY=.*|ENCRYPTION_KEY=$$(openssl rand -hex 32)|" \
			-e "s|^OIDC_HMAC_SECRET=.*|OIDC_HMAC_SECRET=$$(openssl rand -hex 32)|" \
			-e "s|^IAMKIT_ENCRYPTION_KEY=.*|IAMKIT_ENCRYPTION_KEY=$$(openssl rand -base64 32)|" \
			.env.example > .env && chmod 600 .env && echo ".env created with fresh secrets"; fi
	@[ -f secrets/jwt.pem ] || $(MAKE) jwt-key

init: setup
	docker compose up -d --build --wait
	$(MAKE) migrate
	$(MAKE) bootstrap
	@echo ""
	@echo "✅ FreeRouter initialized — run: make dev"

# Fresh stack: drop containers, volumes (both databases), IAMKit credentials
# and the provisioned ids. IAMKit's schema has no upgrade path between
# squashed migrations, so this is also how to move to a newer IAMKit.
reset:
	docker compose down -v
	rm -rf .dev-secrets
	@[ -f .env ] && awk '!/^IAMKIT_(AUDIENCE|ENVIRONMENT_ID|APPLICATION_ID|RESOURCE_ID|ORGANIZATION_ID|SERVICE_SECRET)=/' .env > .env.tmp && mv .env.tmp .env && chmod 600 .env || true
	@echo "Stack reset — run: make init"

tidy:
	go mod tidy

# ── Clean ────────────────────────────────────────────────────────────
clean:
	rm -rf bin/
	docker compose down -v
