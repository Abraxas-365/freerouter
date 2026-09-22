# ── FreeRouter ────────────────────────────────────────────────────────
.PHONY: build dev test lint vet fmt up down migrate init clean jwt-key bootstrap

# ── Build ────────────────────────────────────────────────────────────
build:
	go build -o bin/server ./cmd/server

dev: build
	./bin/server

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
		PGPASSWORD=$${DB_PASSWORD:-freerouter} psql -h $${DB_HOST:-localhost} \
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
	docker compose logs iamkit | grep "management API key"

bootstrap:
	@./scripts/bootstrap-iamkit.sh

# ── Setup ────────────────────────────────────────────────────────────
init: jwt-key up
	@echo "Waiting for services..."
	@sleep 5
	$(MAKE) migrate
	$(MAKE) bootstrap
	@echo ""
	@echo "✅ FreeRouter initialized"

tidy:
	go mod tidy

# ── Clean ────────────────────────────────────────────────────────────
clean:
	rm -rf bin/
	docker compose down -v
