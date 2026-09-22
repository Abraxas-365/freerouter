#!/usr/bin/env bash
# bootstrap-iamkit.sh — Create IAMKit project/environment/application/resource
# for FreeRouter development. Idempotent: skips if .env already has valid IDs.
#
# Usage: ./scripts/bootstrap-iamkit.sh
# Requires: curl, jq
# Reads/writes: .env

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$ROOT_DIR/.env"

# ── Helpers ──────────────────────────────────────────────────────────

die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "▸ $*"; }

command -v curl >/dev/null || die "curl is required"
command -v jq   >/dev/null || die "jq is required"

# Load .env (simple key=value, no export)
if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source <(grep -v '^\s*#' "$ENV_FILE" | grep -v '^\s*$')
  set +a
fi

IAMKIT_BASE_URL="${IAMKIT_BASE_URL:-http://localhost:8090}"

# ── Get management key from running container ────────────────────────

info "Fetching management key from IAMKit container logs…"
MGMT_KEY=$(docker compose -f "$ROOT_DIR/docker-compose.yml" logs iamkit 2>/dev/null \
  | grep -oE 'ik_mgmt_[A-Za-z0-9_-]+' | tail -1) || true

if [[ -z "$MGMT_KEY" ]]; then
  die "Could not find management key in IAMKit logs. Is the container running? (make up)"
fi
info "Management key: ${MGMT_KEY:0:20}…"

BASE="$IAMKIT_BASE_URL/management/v1"

# ── API helper ───────────────────────────────────────────────────────

api() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-s -f -H "X-API-Key: $MGMT_KEY" -H "Content-Type: application/json")
  if [[ -n "$body" ]]; then
    args+=(-d "$body")
  fi
  curl "${args[@]}" -X "$method" "$BASE$path"
}

api_no_fail() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-s -H "X-API-Key: $MGMT_KEY" -H "Content-Type: application/json")
  if [[ -n "$body" ]]; then
    args+=(-d "$body")
  fi
  curl "${args[@]}" -X "$method" "$BASE$path"
}

# ── 1. Project ───────────────────────────────────────────────────────

info "Creating project…"
PROJECT_ID=$(api POST "/projects" '{"name":"FreeRouter"}' | jq -r '.id')
[[ "$PROJECT_ID" != "null" && -n "$PROJECT_ID" ]] || die "Failed to create project"
info "  Project: $PROJECT_ID"

# ── 2. Environment ──────────────────────────────────────────────────

info "Creating environment…"
ENV_ID=$(api POST "/projects/$PROJECT_ID/environments" '{"name":"Development"}' | jq -r '.id')
[[ "$ENV_ID" != "null" && -n "$ENV_ID" ]] || die "Failed to create environment"
info "  Environment: $ENV_ID"

# ── 3. Application ──────────────────────────────────────────────────

info "Creating application…"
APP_ID=$(api POST "/environments/$ENV_ID/applications" \
  '{"name":"FreeRouter Gateway","redirect_uris":["https://localhost:5173/callback"]}' | jq -r '.id')
[[ "$APP_ID" != "null" && -n "$APP_ID" ]] || die "Failed to create application"
info "  Application: $APP_ID"

# ── 4. Resource (with all FreeRouter permissions) ────────────────────

info "Creating resource with permissions…"
PERMISSIONS='[
  "freerouter:gateway:invoke",
  "freerouter:gateway:write",
  "freerouter:metrics:read",
  "freerouter:providers:read",
  "freerouter:providers:write",
  "freerouter:provider-keys:read",
  "freerouter:provider-keys:write",
  "freerouter:usage:read",
  "freerouter:usage:write",
  "freerouter:rate-limits:read",
  "freerouter:rate-limits:write",
  "freerouter:routing:read",
  "freerouter:routing:write",
  "freerouter:guardrails:read",
  "freerouter:guardrails:write",
  "freerouter:webhooks:read",
  "freerouter:webhooks:write",
  "freerouter:service-accounts:read",
  "freerouter:service-accounts:write",
  "freerouter:users:read",
  "freerouter:users:write",
  "freerouter:roles:read",
  "freerouter:roles:write"
]'
# Compact JSON for curl
PERMS_COMPACT=$(echo "$PERMISSIONS" | jq -c .)
RES_ID=$(api POST "/environments/$ENV_ID/resources" \
  "{\"name\":\"FreeRouter API\",\"prefix\":\"freerouter\",\"audience\":\"freerouter\",\"permissions\":$PERMS_COMPACT}" | jq -r '.id')
[[ "$RES_ID" != "null" && -n "$RES_ID" ]] || die "Failed to create resource"
info "  Resource: $RES_ID"

# ── 5. Bind resource → application ──────────────────────────────────

info "Binding resource to application…"
api_no_fail POST "/environments/$ENV_ID/application-resources" \
  "{\"application_id\":\"$APP_ID\",\"resource_id\":\"$RES_ID\"}" >/dev/null
info "  Bound ✓"

# ── 6. Organization ─────────────────────────────────────────────────

info "Creating organization…"
ORG_ID=$(api POST "/environments/$ENV_ID/organizations" '{"name":"Default"}' | jq -r '.id')
[[ "$ORG_ID" != "null" && -n "$ORG_ID" ]] || die "Failed to create organization"
info "  Organization: $ORG_ID"

# ── 7. Admin service account (all permissions) ──────────────────────

info "Creating admin service account…"
SA=$(api POST "/environments/$ENV_ID/service-accounts" \
  "{\"name\":\"FreeRouter Admin\",\"application_id\":\"$APP_ID\",\"resource_id\":\"$RES_ID\",\"permissions\":$PERMS_COMPACT}")
SA_ID=$(echo "$SA" | jq -r '.id')
SA_SECRET=$(echo "$SA" | jq -r '.secret')
SA_EXPIRES=$(echo "$SA" | jq -r '.expires_at')
[[ "$SA_SECRET" != "null" && -n "$SA_SECRET" ]] || die "Failed to create service account"
info "  Service Account: $SA_ID"
info "  Secret: ${SA_SECRET:0:20}…"
info "  Expires: $SA_EXPIRES"

# ── 8. Update .env ──────────────────────────────────────────────────

info "Updating .env…"

update_env() {
  local key="$1" value="$2"
  if grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then
    sed -i '' "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
  else
    echo "${key}=${value}" >> "$ENV_FILE"
  fi
}

update_env "IAMKIT_MANAGEMENT_KEY" "$MGMT_KEY"
update_env "IAMKIT_ENVIRONMENT_ID" "$ENV_ID"
update_env "IAMKIT_APPLICATION_ID" "$APP_ID"
update_env "IAMKIT_RESOURCE_ID"    "$RES_ID"
update_env "IAMKIT_ORGANIZATION_ID" "$ORG_ID"

info ".env updated ✓"

# ── Done ─────────────────────────────────────────────────────────────

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  ✅ IAMKit bootstrapped for FreeRouter"
echo ""
echo "  IAMKit Console:  $IAMKIT_BASE_URL"
echo "  Login:           ${IAMKIT_BOOTSTRAP_EMAIL:-admin@freerouter.dev}"
echo ""
echo "  Admin service account secret (save this!):"
echo "  $SA_SECRET"
echo ""
echo "  Exchange for JWT:"
echo "  curl -X POST -H 'Authorization: Bearer $SA_SECRET' \\"
echo "       $IAMKIT_BASE_URL/identity/v1/machine-token"
echo "═══════════════════════════════════════════════════════════"
