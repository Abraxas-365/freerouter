#!/usr/bin/env bash
# bootstrap-iamkit.sh — provision IAMKit for FreeRouter development and
# write FreeRouter's IAMKit ids and backend credential to .env.
#
# Creates: workspace owner credential (iamkit bootstrap CLI, kept in
# .dev-secrets/owner.json), project, environment, organization "Default",
# application "FreeRouter Gateway", resource "FreeRouter API" (prefix
# freerouter), the binding, an admin service account with every freerouter
# permission (.dev-secrets/admin-service-account), and FreeRouter's backend
# service account on the environment's IAM resource (least privilege, see
# scripts/permissions.sh).
#
# Requires: docker compose stack running (make up), curl, jq.
# Idempotency: refuses to run twice (IAMKIT_ENVIRONMENT_ID already in .env).
# Never enable shell tracing: the management key and secrets pass through it.
set -euo pipefail
cd "$(dirname "$0")/.."

die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "▸ $*"; }

command -v curl >/dev/null || die "curl is required"
command -v jq   >/dev/null || die "jq is required"
[[ -f .env ]] || die ".env missing: run make setup"

set -a; source .env; set +a
if [[ -n "${IAMKIT_ENVIRONMENT_ID:-}" ]]; then
  die "IAMKit already provisioned (IAMKIT_ENVIRONMENT_ID set in .env). Run 'make reset' for a fresh stack."
fi

base="http://localhost:${IAMKIT_PORT:-8080}"
audience="${IAMKIT_AUDIENCE:-http://localhost:${SERVER_PORT:-3000}}"
operator="${IAMKIT_OPERATOR_EMAIL:-admin@freerouter.dev}"
permissions=$(source scripts/permissions.sh && jq -c . <<<"$FREEROUTER_PERMISSIONS")
backend_permissions=$(source scripts/permissions.sh && jq -c . <<<"$BACKEND_IAM_PERMISSIONS")

info "Waiting for IAMKit at ${base}…"
for _ in $(seq 1 60); do
  curl --silent --fail "$base/health" >/dev/null 2>&1 && break
  sleep 1
done
curl --silent --fail "$base/health" >/dev/null || die "IAMKit is not answering at $base (make up; docker compose logs iamkit)"

# ── Owner credential (one-time management key, private file) ────────
umask 077
mkdir -p .dev-secrets
if [[ ! -f .dev-secrets/owner.json ]]; then
  info "Bootstrapping workspace for ${operator}…"
  docker compose exec -T iamkit sh -c 'rm -f /tmp/owner.json && iamkit bootstrap --email "$0" --workspace FreeRouter --output /tmp/owner.json >/dev/null && cat /tmp/owner.json && rm -f /tmp/owner.json' \
    "$operator" > .dev-secrets/owner.json.tmp
  mv .dev-secrets/owner.json.tmp .dev-secrets/owner.json
fi
MGMT=$(jq -er .management_key .dev-secrets/owner.json) || die ".dev-secrets/owner.json has no management_key"

management() {
  curl --fail-with-body --silent --show-error --request "$1" "$base/management/v1$2" \
    -H "X-API-Key: $MGMT" -H 'Content-Type: application/json' --data-binary @-
}

info "Creating project, environment, organization…"
project=$(jq -n '{name:"FreeRouter"}' | management POST /projects | jq -er .id)
environment=$(jq -n '{name:"Development"}' | management POST "/projects/$project/environments" | jq -er .id)
p="/environments/$environment"
organization=$(jq -n '{name:"Default"}' | management POST "$p/organizations" | jq -er .id)

info "Creating application and resource…"
application=$(jq -n '{name:"FreeRouter Gateway",redirect_uris:["http://localhost:5173/callback"]}' | management POST "$p/applications" | jq -er .id)
resource=$(jq -n --arg a "$audience" --argjson perms "$permissions" \
  '{name:"FreeRouter API",prefix:"freerouter",audience:$a,permissions:$perms}' | management POST "$p/resources" | jq -er .id)
jq -n --arg a "$application" --arg r "$resource" '{application_id:$a,resource_id:$r}' \
  | management POST "$p/application-resources" >/dev/null

info "Creating admin service account (all freerouter permissions)…"
admin_secret=$(jq -n --arg a "$application" --arg r "$resource" --argjson perms "$permissions" \
  '{name:"FreeRouter Admin",application_id:$a,resource_id:$r,permissions:$perms}' \
  | management POST "$p/service-accounts" | jq -er .secret)

# FreeRouter's backend: a separate application bound to the environment's
# built-in IAM resource, with a service account holding only the iam:*
# permissions FreeRouter calls (users, roles, service accounts).
info "Creating backend service account on the IAM resource…"
iam_resource=$(management GET "$p/resources?limit=100" </dev/null | jq -er '[(.items? // .)[] | select(.prefix == "iam")][0].id')
backend=$(jq -n '{name:"FreeRouter Backend"}' | management POST "$p/applications" | jq -er .id)
jq -n --arg a "$backend" --arg r "$iam_resource" '{application_id:$a,resource_id:$r}' \
  | management POST "$p/application-resources" >/dev/null
service_secret=$(jq -n --arg a "$backend" --arg r "$iam_resource" --argjson perms "$backend_permissions" \
  '{name:"freerouter-backend",application_id:$a,resource_id:$r,permissions:$perms,expires_in:"never"}' \
  | management POST "$p/service-accounts" | jq -er .secret)

# Optional console admin: a user in the organization with every freerouter
# permission (direct grant), when FREEROUTER_ADMIN_EMAIL/PASSWORD are set.
if [[ -n "${FREEROUTER_ADMIN_EMAIL:-}" && -n "${FREEROUTER_ADMIN_PASSWORD:-}" ]]; then
  info "Creating console admin ${FREEROUTER_ADMIN_EMAIL}…"
  user=$(jq -n --arg e "$FREEROUTER_ADMIN_EMAIL" --arg pw "$FREEROUTER_ADMIN_PASSWORD" --arg o "$organization" \
    '{name:"FreeRouter Admin",email:$e,password:$pw,home_organization_id:$o}' | management POST "$p/users" | jq -er .id)
  jq -n --arg o "$organization" --arg u "$user" --arg r "$resource" --argjson perms "$permissions" \
    '{organization_id:$o,user_id:$u,resource_id:$r,permissions:$perms}' | management PUT "$p/grants" >/dev/null
fi

# ── .env ────────────────────────────────────────────────────────────
set_env() { # set_env KEY VALUE: replace the line or append it (portable, no sed -i)
  local tmp
  tmp=$(mktemp)
  awk -v k="$1" -v v="$2" 'BEGIN{done=0} index($0, k"=")==1 {print k"="v; done=1; next} {print} END{if(!done) print k"="v}' .env > "$tmp"
  cat "$tmp" > .env && rm -f "$tmp"
}
set_env IAMKIT_AUDIENCE        "$audience"
set_env IAMKIT_ENVIRONMENT_ID  "$environment"
set_env IAMKIT_APPLICATION_ID  "$application"
set_env IAMKIT_RESOURCE_ID     "$resource"
set_env IAMKIT_ORGANIZATION_ID "$organization"
set_env IAMKIT_SERVICE_SECRET  "$service_secret"
printf '%s\n' "$admin_secret" > .dev-secrets/admin-service-account

cat <<EOF

✅ IAMKit provisioned for FreeRouter (environment $environment)

  .env updated: IAMKIT_AUDIENCE, _ENVIRONMENT_ID, _APPLICATION_ID, _RESOURCE_ID, _ORGANIZATION_ID, _SERVICE_SECRET
  Admin API key (all freerouter permissions): .dev-secrets/admin-service-account
    curl -H "Authorization: Bearer \$(cat .dev-secrets/admin-service-account)" localhost:${SERVER_PORT:-3000}/api/v1/providers
  IAMKit console: $base (operator $operator; set a password with "Set up your account")

  web/.env for the FreeRouter console:
    VITE_IAMKIT_ENVIRONMENT_ID=$environment
    VITE_IAMKIT_ORGANIZATION_ID=$organization
    VITE_IAMKIT_APPLICATION_ID=$application
    VITE_IAMKIT_RESOURCE_ID=$resource
    VITE_IAMKIT_AUDIENCE=$audience

Restart FreeRouter so it picks up the ids: make dev
EOF
