#!/usr/bin/env bash
# Boots an isolated FreeRouter stack for the end-to-end suite:
#   postgres + redis + IAMKit (docker, project frv2e2e, non-default ports)
#   fakellm + webhooksink + FreeRouter server + web console (host processes)
# then provisions IAMKit, seeds personas and a catalog, and writes
# e2e/.run/fixtures.json for the test workstreams.
#
# Usage: e2e/up.sh          (idempotent: re-running restarts host processes)
#        e2e/down.sh        (stops everything, drops volumes)
set -euo pipefail
export LC_ALL=C.UTF-8

ROOT=$(cd "$(dirname "$0")/.." && pwd)
RUN="$ROOT/e2e/.run"
mkdir -p "$RUN"
cd "$ROOT"

export COMPOSE_PROJECT_NAME=frv2e2e
export COMPOSE_FILE="$ROOT/docker-compose.yml:$ROOT/e2e/docker-compose.e2e.yml"

# ── Ports (override with env) ─────────────────────────────────────────
E2E_DB_PORT=${E2E_DB_PORT:-25432}
E2E_REDIS_PORT=${E2E_REDIS_PORT:-16379}
E2E_IAMKIT_PORT=${E2E_IAMKIT_PORT:-28080}
E2E_SERVER_PORT=${E2E_SERVER_PORT:-23000}
E2E_WEB_PORT=${E2E_WEB_PORT:-25173}
E2E_FAKELLM_PORT=${E2E_FAKELLM_PORT:-29100}
E2E_SINK_PORT=${E2E_SINK_PORT:-29200}
IAMKIT_SRC=${IAMKIT_SRC:-$ROOT/../iam}
export E2E_REDIS_PORT

info() { printf '▸ %s\n' "$*"; }
die()  { printf '✗ %s\n' "$*" >&2; exit 1; }

for tool in docker jq curl psql go npm; do
  command -v "$tool" >/dev/null || die "missing tool: $tool"
done
[[ -d "$IAMKIT_SRC" ]] || die "IAMKIT_SRC=$IAMKIT_SRC does not exist"

# Port check only for a brand-new stack (no run dir yet); re-runs reuse ours.
if [[ ! -f "$RUN/.env" ]]; then
  for p in $E2E_DB_PORT $E2E_REDIS_PORT $E2E_IAMKIT_PORT $E2E_SERVER_PORT $E2E_WEB_PORT $E2E_FAKELLM_PORT $E2E_SINK_PORT; do
    if lsof -nP -iTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1; then
      die "port $p is in use (set E2E_*_PORT to change)"
    fi
  done
fi

# ── Stop host processes from a previous run ───────────────────────────
if [[ -f "$RUN/pids" ]]; then
  info "Stopping previous host processes…"
  while read -r pid; do kill "$pid" 2>/dev/null || true; done < "$RUN/pids"
  rm -f "$RUN/pids"
fi
for p in $E2E_SERVER_PORT $E2E_WEB_PORT $E2E_FAKELLM_PORT $E2E_SINK_PORT; do
  lsof -nP -tiTCP:"$p" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
done
sleep 1

# ── .env for this stack ───────────────────────────────────────────────
ENV="$RUN/.env"
if [[ ! -f "$ENV" ]]; then
  info "Generating ${ENV}…"
  cp .env.example "$ENV"
  set_env() { # key value
    if grep -q "^$1=" "$ENV"; then
      sed -i.bak "s|^$1=.*|$1=$2|" "$ENV" && rm -f "$ENV.bak"
    else
      printf '%s=%s\n' "$1" "$2" >> "$ENV"
    fi
  }
  set_env APP_ENV test
  set_env WEBHOOK_ALLOW_PRIVATE true
  set_env SERVER_PORT "$E2E_SERVER_PORT"
  set_env DB_PORT "$E2E_DB_PORT"
  set_env REDIS_ADDR "localhost:$E2E_REDIS_PORT"
  set_env IAMKIT_PORT "$E2E_IAMKIT_PORT"
  set_env IAMKIT_BASE_URL "http://localhost:$E2E_IAMKIT_PORT"
  set_env IAMKIT_JWT_ISSUER "http://localhost:$E2E_IAMKIT_PORT"
  set_env IAMKIT_AUDIENCE "http://localhost:$E2E_SERVER_PORT"
  set_env IAMKIT_SRC "$IAMKIT_SRC"
  set_env CORS_ALLOWED_ORIGINS "http://localhost:$E2E_WEB_PORT,http://127.0.0.1:$E2E_WEB_PORT"
  set_env FREEROUTER_ADMIN_EMAIL admin@e2e.test
  set_env FREEROUTER_ADMIN_PASSWORD "e2e-admin-password-1"
  set_env CACHE_TTL_SECONDS 5
  # Above e2e-slow's 20s answer, below the hung-upstream test's 35s budget.
  set_env UPSTREAM_RESPONSE_TIMEOUT_SECONDS 25
  set_env ENCRYPTION_KEY "$(openssl rand -hex 32)"
  set_env OIDC_HMAC_SECRET "$(openssl rand -hex 32)"
  set_env IAMKIT_ENCRYPTION_KEY "$(openssl rand -base64 32)"
fi
[[ -f secrets/jwt.pem ]] || make -s jwt-key

set -a; source "$ENV"; set +a

# ── Docker services ───────────────────────────────────────────────────
info "Starting postgres/redis/iamkit (project $COMPOSE_PROJECT_NAME)…"
docker compose up -d --build --wait

info "Running migrations…"
if [[ ! -f "$RUN/migrated" ]]; then
  for f in migrations/*.up.sql; do
    PGPASSWORD="$DB_PASSWORD" psql -q -v ON_ERROR_STOP=1 -h localhost -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$f" >/dev/null
  done
  touch "$RUN/migrated"
fi

# ── IAMKit provisioning (only once per stack) ─────────────────────────
if ! grep -q '^IAMKIT_SERVICE_SECRET=ik_svc_' "$ENV"; then
  info "Provisioning IAMKit…"
  ENV_FILE="$ENV" SECRETS_DIR="$RUN/secrets" bash scripts/bootstrap-iamkit.sh >"$RUN/bootstrap.log" 2>&1 \
    || { cat "$RUN/bootstrap.log"; die "IAMKit bootstrap failed"; }
  set -a; source "$ENV"; set +a
fi

# ── Host processes ────────────────────────────────────────────────────
info "Building server, fakellm, webhooksink…"
go build -o "$RUN/server" ./cmd/server
go build -o "$RUN/fakellm" ./e2e/fakellm
go build -o "$RUN/webhooksink" ./e2e/webhooksink

: > "$RUN/pids"
# Detached (nohup + new session) so they outlive this script.
spawn() { nohup "$@" >/dev/null 2>&1 < /dev/null & echo $! >> "$RUN/pids"; }
PORT=$E2E_FAKELLM_PORT FAKE_SLOW_SECONDS=${FAKE_SLOW_SECONDS:-20} spawn sh -c "exec \"$RUN/fakellm\" >\"$RUN/fakellm.log\" 2>&1"
PORT=$E2E_SINK_PORT spawn sh -c "exec \"$RUN/webhooksink\" >\"$RUN/webhooksink.log\" 2>&1"
spawn sh -c "exec \"$RUN/server\" >\"$RUN/server.log\" 2>&1"

wait_http() { # url
  for _ in $(seq 1 60); do curl -sf -o /dev/null "$1" && return 0; sleep 0.5; done
  die "timeout waiting for $1"
}
wait_http "http://localhost:$E2E_FAKELLM_PORT/_health"
wait_http "http://localhost:$E2E_SINK_PORT/_health"
wait_http "http://localhost:$E2E_SERVER_PORT/health"

# ── Web console ───────────────────────────────────────────────────────
cat > web/.env.e2e <<EOF
VITE_API_URL=http://localhost:$E2E_SERVER_PORT
VITE_IAMKIT_URL=http://localhost:$E2E_IAMKIT_PORT
VITE_IAMKIT_ENVIRONMENT_ID=$IAMKIT_ENVIRONMENT_ID
VITE_IAMKIT_ORGANIZATION_ID=$IAMKIT_ORGANIZATION_ID
VITE_IAMKIT_APPLICATION_ID=$IAMKIT_APPLICATION_ID
VITE_IAMKIT_RESOURCE_ID=$IAMKIT_RESOURCE_ID
VITE_IAMKIT_AUDIENCE=$IAMKIT_AUDIENCE
EOF
[[ -d web/node_modules ]] || (cd web && npm ci --silent)
(cd web && nohup npx vite --mode e2e --port "$E2E_WEB_PORT" --strictPort >"$RUN/web.log" 2>&1 < /dev/null & echo $! >> "$RUN/pids")
wait_http "http://localhost:$E2E_WEB_PORT/"

# ── Seed personas + catalog ───────────────────────────────────────────
info "Seeding personas and catalog…"
E2E_RUN_DIR="$RUN" \
E2E_SERVER_URL="http://localhost:$E2E_SERVER_PORT" \
E2E_IAMKIT_URL="http://localhost:$E2E_IAMKIT_PORT" \
E2E_WEB_URL="http://localhost:$E2E_WEB_PORT" \
E2E_FAKELLM_URL="http://localhost:$E2E_FAKELLM_PORT" \
E2E_SINK_URL="http://localhost:$E2E_SINK_PORT" \
E2E_DB_URL="postgres://$DB_USER:$DB_PASSWORD@localhost:$DB_PORT/$DB_NAME?sslmode=disable" \
E2E_REDIS_ADDR="localhost:$E2E_REDIS_PORT" \
bash e2e/seed.sh

echo
echo "✅ e2e stack ready — fixtures: $RUN/fixtures.json"
echo "   server  http://localhost:$E2E_SERVER_PORT   console http://localhost:$E2E_WEB_PORT"
echo "   iamkit  http://localhost:$E2E_IAMKIT_PORT   fakellm http://localhost:$E2E_FAKELLM_PORT   sink http://localhost:$E2E_SINK_PORT"
echo "   logs    $RUN/{server,web,fakellm,webhooksink}.log"
