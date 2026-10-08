#!/usr/bin/env bash
# Tears down the e2e stack: host processes, docker project + volumes, run dir.
set -uo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
RUN="$ROOT/e2e/.run"
cd "$ROOT"

if [[ -f "$RUN/pids" ]]; then
  while read -r pid; do kill "$pid" 2>/dev/null || true; done < "$RUN/pids"
fi
# vite spawns a child; kill anything still bound to our ports
for p in ${E2E_WEB_PORT:-25173} ${E2E_SERVER_PORT:-23000} ${E2E_FAKELLM_PORT:-29100} ${E2E_SINK_PORT:-29200}; do
  lsof -nP -tiTCP:"$p" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
done

COMPOSE_PROJECT_NAME=frv2e2e \
COMPOSE_FILE="$ROOT/docker-compose.yml:$ROOT/e2e/docker-compose.e2e.yml" \
OIDC_HMAC_SECRET=x docker compose down -v --remove-orphans 2>/dev/null || true

rm -rf "$RUN" web/.env.e2e
echo "e2e stack stopped"
