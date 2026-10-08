#!/usr/bin/env bash
# Seeds personas and a fake-LLM-backed catalog into a running e2e stack and
# writes $E2E_RUN_DIR/fixtures.json. Idempotent per stack: skips if the
# fixtures file already exists (delete it to reseed).
#
# Inputs (env): E2E_RUN_DIR E2E_SERVER_URL E2E_IAMKIT_URL E2E_WEB_URL
#               E2E_FAKELLM_URL E2E_SINK_URL E2E_DB_URL E2E_REDIS_ADDR
# Reads:        $E2E_RUN_DIR/.env (IAMKit ids), $E2E_RUN_DIR/secrets/*
set -euo pipefail
export LC_ALL=C.UTF-8
cd "$(dirname "$0")/.."

RUN=$E2E_RUN_DIR
OUT="$RUN/fixtures.json"
if [[ -f "$OUT" ]]; then
  echo "▸ fixtures already present ($OUT); skipping seed"
  exit 0
fi

set -a; source "$RUN/.env"; set +a
ADMIN_KEY=$(cat "$RUN/secrets/admin-service-account")
MGMT=$(jq -er .management_key "$RUN/secrets/owner.json")
API="$E2E_SERVER_URL/api/v1"
permissions=$(source scripts/permissions.sh && jq -c . <<<"$FREEROUTER_PERMISSIONS")

die() { echo "ERROR: $*" >&2; exit 1; }
api() { # METHOD PATH [JSON]
  local out
  out=$(curl -sS --fail-with-body -X "$1" "$API$2" -H "Authorization: Bearer $ADMIN_KEY" -H "Content-Type: application/json" ${3:+-d "$3"}) \
    || die "$1 $2 failed: $out"
  printf '%s' "$out"
}
mgmt() { # METHOD PATH [JSON]
  local out
  out=$(curl -sS --fail-with-body -X "$1" "$E2E_IAMKIT_URL/management/v1$2" -H "X-API-Key: $MGMT" -H "Content-Type: application/json" ${3:+-d "$3"}) \
    || die "mgmt $1 $2 failed: $out"
  printf '%s' "$out"
}

PW="e2e-password-12345"
ENVP="/environments/$IAMKIT_ENVIRONMENT_ID"

# ── Users + roles ─────────────────────────────────────────────────────
mkuser() { api POST /access/users "$(jq -nc --arg e "$1" --arg n "$2" --arg p "$PW" '{email:$e,name:$n,password:$p}')" | jq -r .id; }
mkrole() { api POST /access/roles "$(jq -nc --arg n "$1" --argjson p "$2" '{name:$n,permissions:$p}')" | jq -r .id; }
assign() { api POST /access/role-assignments "$(jq -nc --arg u "$1" --arg r "$2" '{user_id:$u,role_id:$r}')" >/dev/null; }

read_perms=$(jq -c '[.[] | select(endswith(":read"))]' <<<"$permissions")
viewer_role=$(mkrole "e2e-viewer" "$read_perms")
providers_role=$(mkrole "e2e-providers-only" '["freerouter:providers:read","freerouter:providers:write"]')
admin_role=$(mkrole "e2e-admin" "$permissions")

viewer=$(mkuser viewer@e2e.test "E2E Viewer");            assign "$viewer" "$viewer_role"
providers_only=$(mkuser providers@e2e.test "E2E Providers"); assign "$providers_only" "$providers_role"
noperm=$(mkuser noperm@e2e.test "E2E No Permissions")
suspended=$(mkuser suspended@e2e.test "E2E Suspended"); assign "$suspended" "$viewer_role"
api DELETE "/access/users/$suspended" >/dev/null
admin_user=$(api GET /access/users | jq -r '.[] | select(.email=="admin@e2e.test") | .id')
[[ -n "$admin_user" ]] || die "console admin admin@e2e.test not found (FREEROUTER_ADMIN_* in .env?)"

# ── Service accounts ──────────────────────────────────────────────────
gw_key=$(api POST /service-accounts '{"name":"e2e-gateway-only","permissions":["freerouter:gateway:invoke"]}')
gw_key_id=$(jq -r .id <<<"$gw_key"); gw_key_secret=$(jq -r .secret <<<"$gw_key")
expired=$(api POST /service-accounts '{"name":"e2e-expired","permissions":["freerouter:gateway:invoke"],"expires_in":"1h"}')
expired_secret=$(jq -r .secret <<<"$expired"); expired_id=$(jq -r .id <<<"$expired")
# IAMKit's minimum TTL is 1h: backdate the expiry in its database.
docker compose -p frv2e2e exec -T postgres psql -q -U iamkit -d iamkit \
  -c "UPDATE service_accounts SET expires_at = now() - interval '1 minute' WHERE id = '$expired_id'" >/dev/null \
  || die "could not backdate expired key"
noperm_key=$(api POST /service-accounts '{"name":"e2e-noperm-key","permissions":["freerouter:metrics:read"]}')
noperm_key_secret=$(jq -r .secret <<<"$noperm_key")

# ── Foreign (out-of-boundary) objects, via management API ─────────────
iam_resource=$(mgmt GET "$ENVP/resources" | jq -r '(.items? // .)[] | select(.prefix=="iam") | .id')
foreign_role=$(mgmt POST "$ENVP/roles" "$(jq -nc --arg r "$iam_resource" '{name:"e2e-foreign-iam-role",resource_id:$r,permissions:["iam:users:read"]}')" | jq -r .id)
foreign_org=$(mgmt POST "$ENVP/organizations" '{"name":"E2E Foreign Org"}' | jq -r .id)
foreign_user=$(mgmt POST "$ENVP/users" "$(jq -nc --arg o "$foreign_org" --arg p "$PW" '{name:"E2E Foreign",email:"foreign@e2e.test",password:$p,home_organization_id:$o}')" | jq -r .id)
backend_sa=$(mgmt GET "$ENVP/service-accounts" | jq -r '(.items? // .)[] | select(.name=="freerouter-backend") | .id')

# ── Catalog: one provider per protocol → fakellm ──────────────────────
mkprovider() { # name protocol path
  api POST /providers "$(jq -nc --arg n "$1" --arg p "$2" --arg u "$E2E_FAKELLM_URL$3" '{name:$n,protocol:$p,base_url:$u,description:"e2e fake",website:"https://example.com",streaming:true}')" | jq -r .id
}
mkmodel() { api POST /models "$(jq -nc --arg n "$1" --arg f "$2" '{name:$n,family:$f,description:"e2e"}')" | jq -r .id; }
mkmapping() { # model_id provider_id external_id
  api POST /mappings "$(jq -nc --arg m "$1" --arg p "$2" --arg e "$3" '{model_id:$m,provider_id:$p,external_id:$e,input_price:1.0,output_price:2.0,streaming:true,tools:true,json_output:true}')" | jq -r .id
}
mkkey() { api POST /provider-keys "$(jq -nc --arg p "$1" --arg n "$2" --arg t "$3" '{provider_id:$p,name:$n,key_type:"api_key",token:$t}')" | jq -r .id; }

p_openai=$(mkprovider "e2e-openai" openai /openai/v1)
p_anthropic=$(mkprovider "e2e-anthropic" anthropic /anthropic/v1)
p_google=$(mkprovider "e2e-google" google /google/v1beta)
p_cohere=$(mkprovider "e2e-cohere" cohere /cohere/v2)
k_openai=$(mkkey "$p_openai" "e2e-openai-key" "sk-e2e-openai")
k_anthropic=$(mkkey "$p_anthropic" "e2e-anthropic-key" "sk-ant-e2e")
k_google=$(mkkey "$p_google" "e2e-google-key" "AIza-e2e")
k_cohere=$(mkkey "$p_cohere" "e2e-cohere-key" "co-e2e")

declare -A models
for spec in \
  "e2e-ok:openai:$p_openai:fake-ok" \
  "e2e-ok-anthropic:anthropic:$p_anthropic:fake-ok" \
  "e2e-ok-google:google:$p_google:fake-ok" \
  "e2e-ok-cohere:cohere:$p_cohere:fake-ok" \
  "e2e-500:openai:$p_openai:fake-500" \
  "e2e-429:openai:$p_openai:fake-429" \
  "e2e-400:openai:$p_openai:fake-400" \
  "e2e-slow:openai:$p_openai:fake-slow" \
  "e2e-stream-abort:openai:$p_openai:fake-stream-abort" \
  "e2e-malformed:openai:$p_openai:fake-malformed" \
  "e2e-nousage:openai:$p_openai:fake-nousage" \
  "e2e-flaky:openai:$p_openai:fake-flaky-2"; do
  IFS=: read -r name family prov ext <<<"$spec"
  mid=$(mkmodel "$name" "$family")
  mkmapping "$mid" "$prov" "$ext" >/dev/null
  models[$name]=$mid
done
# Fallback chain: e2e-fallback → fake-500 primary, then falls back to e2e-ok.
fb=$(mkmodel "e2e-fallback" openai); mkmapping "$fb" "$p_openai" "fake-500" >/dev/null
api POST /model-fallbacks "$(jq -nc --arg m "$fb" --arg f "${models[e2e-ok]}" '{model_id:$m,fallback_model_id:$f,priority:1}')" >/dev/null
models[e2e-fallback]=$fb

# ── fixtures.json ─────────────────────────────────────────────────────
umask 077
jq -n \
  --arg server "$E2E_SERVER_URL" --arg iamkit "$E2E_IAMKIT_URL" --arg web "$E2E_WEB_URL" \
  --arg fakellm "$E2E_FAKELLM_URL" --arg sink "$E2E_SINK_URL" --arg db "$E2E_DB_URL" --arg redis "$E2E_REDIS_ADDR" \
  --arg env "$IAMKIT_ENVIRONMENT_ID" --arg org "$IAMKIT_ORGANIZATION_ID" --arg app "$IAMKIT_APPLICATION_ID" \
  --arg res "$IAMKIT_RESOURCE_ID" --arg aud "$IAMKIT_AUDIENCE" --arg mgmt "$MGMT" \
  --arg pw "$PW" --arg admin_pw "$FREEROUTER_ADMIN_PASSWORD" \
  --arg admin_key "$ADMIN_KEY" --arg gw_secret "$gw_key_secret" --arg gw_id "$gw_key_id" \
  --arg expired "$expired_secret" --arg noperm_key "$noperm_key_secret" \
  --arg admin_user "$admin_user" --arg viewer "$viewer" --arg providers_only "$providers_only" --arg noperm "$noperm" --arg suspended "$suspended" \
  --arg viewer_role "$viewer_role" --arg providers_role "$providers_role" --arg admin_role "$admin_role" \
  --arg foreign_role "$foreign_role" --arg foreign_org "$foreign_org" --arg foreign_user "$foreign_user" --arg backend_sa "$backend_sa" --arg iam_resource "$iam_resource" \
  --arg p_openai "$p_openai" --arg p_anthropic "$p_anthropic" --arg p_google "$p_google" --arg p_cohere "$p_cohere" \
  --arg k_openai "$k_openai" --arg k_anthropic "$k_anthropic" --arg k_google "$k_google" --arg k_cohere "$k_cohere" \
  --argjson models "$(for k in "${!models[@]}"; do printf '%s\t%s\n' "$k" "${models[$k]}"; done | jq -Rn '[inputs | split("\t") | {(.[0]): .[1]}] | add')" \
  --argjson permissions "$permissions" \
'{
  urls: {server:$server, api:($server+"/api/v1"), gateway:($server+"/v1"), iamkit:$iamkit, web:$web, fakellm:$fakellm, webhook_sink:$sink, db:$db, redis:$redis},
  iamkit: {environment_id:$env, organization_id:$org, application_id:$app, resource_id:$res, audience:$aud, management_key:$mgmt},
  personas: {
    admin:          {kind:"user", email:"admin@e2e.test",     password:$admin_pw, user_id:$admin_user, note:"all freerouter permissions (direct grant)"},
    viewer:         {kind:"user", email:"viewer@e2e.test",    password:$pw, user_id:$viewer,         role_id:$viewer_role,    note:"every *:read permission"},
    providers_only: {kind:"user", email:"providers@e2e.test", password:$pw, user_id:$providers_only, role_id:$providers_role, note:"providers:read/write only"},
    noperm:         {kind:"user", email:"noperm@e2e.test",    password:$pw, user_id:$noperm,         note:"member of org, no role"},
    suspended:      {kind:"user", email:"suspended@e2e.test", password:$pw, user_id:$suspended,      note:"suspended (active=false)"},
    foreign:        {kind:"user", email:"foreign@e2e.test",   password:$pw, user_id:$foreign_user,   organization_id:$foreign_org, note:"user in another organization"},
    admin_key:      {kind:"service_account", secret:$admin_key,  note:"bootstrap admin key, all freerouter permissions"},
    gw_key:         {kind:"service_account", secret:$gw_secret, id:$gw_id, note:"gateway:invoke only"},
    noperm_key:     {kind:"service_account", secret:$noperm_key, note:"metrics:read only"},
    expired_key:    {kind:"service_account", secret:$expired,   note:"expired 1s after creation"}
  },
  roles: {viewer:$viewer_role, providers_only:$providers_role, admin:$admin_role, foreign_iam:$foreign_role},
  boundary: {foreign_role_id:$foreign_role, foreign_org_id:$foreign_org, foreign_user_id:$foreign_user, backend_service_account_id:$backend_sa, iam_resource_id:$iam_resource},
  providers: {openai:$p_openai, anthropic:$p_anthropic, google:$p_google, cohere:$p_cohere},
  provider_keys: {openai:$k_openai, anthropic:$k_anthropic, google:$k_google, cohere:$k_cohere},
  models: $models,
  permissions: $permissions,
  fakellm_behaviors: "model name decides: fake-ok, fake-500, fake-429, fake-400, fake-slow, fake-stream-abort, fake-malformed, fake-nousage, fake-flaky-N; GET /_requests, DELETE /_requests, POST /_reset",
  webhook_sink: "POST {sink}/hook/<name> records; /hook/fail/<n>/<name> fails n times then 200; GET/DELETE {sink}/_deliveries"
}' > "$OUT"

echo "▸ seeded: $(jq -r '.personas | keys | join(", ")' "$OUT")"
echo "▸ models: $(jq -r '.models | keys | join(", ")' "$OUT")"
