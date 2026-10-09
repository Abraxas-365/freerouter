#!/usr/bin/env bash
# WS-12 Part C — install path on a FRESH clone, isolated from every other stack.
#
#   e2e/install_test.sh                 # defaults below; ~3-6 min
#   WS12_DB_PORT=45432 … e2e/install_test.sh
#
# Clones the repo (committed HEAD) into $WS12_DIR, runs `make init` with its
# own compose project and ports, and checks: missing IAMKIT_SRC error, ports
# configured only in .env, first `make init`, the printed instructions
# (make dev, console admin login via IAMKit, admin key → /api/v1/providers),
# second `make init` / `make bootstrap` refusing, `make reset && make init`.
# Everything (containers, volumes, clone) is removed on exit, even on failure.
# Never touches the e2e stack (project frv2e2e).
set -euo pipefail

REPO=${WS12_REPO:-$(cd "$(dirname "$0")/.." && pwd)}
DIR=${WS12_DIR:-/tmp/ws12-clone}
PROJECT=${WS12_PROJECT:-frv2ws12}
DB_PORT=${WS12_DB_PORT:-35432}
REDIS_PORT=${WS12_REDIS_PORT:-36379}
IAMKIT_PORT=${WS12_IAMKIT_PORT:-38080}
SERVER_PORT=${WS12_SERVER_PORT:-33000}
IAMKIT_SRC_DIR=${IAMKIT_SRC:-/Users/abraxas/Desktop/Proyectos/iam}
ADMIN_EMAIL=admin@ws12.test
ADMIN_PASSWORD=ws12-install-Passw0rd!
LOG=${WS12_LOG:-/tmp/ws12-install.log}

pass=0 fail=0 server_pid=""
: > "$LOG"
say()  { printf '%s\n' "$*" | tee -a "$LOG"; }
ok()   { pass=$((pass+1)); say "  ✓ $*"; }
bad()  { fail=$((fail+1)); say "  ✗ $*"; }
step() { say ""; say "── $* ($(date +%H:%M:%S))"; }
# run NAME CMD…: runs in the clone, output to $DIR/../ws12-<name>.out, returns exit code
run() {
  local name=$1; shift
  local out="/tmp/ws12-$name.out" start=$SECONDS rc=0
  (cd "$DIR" && "$@") >"$out" 2>&1 || rc=$?
  say "  [$name] exit $rc in $((SECONDS-start))s (output: $out)"
  return $rc
}

# `make dev` runs ./bin/server as a child of make: kill whatever listens on SERVER_PORT.
kill_port_server() {
  local pids
  pids=$(lsof -nP -t -iTCP:"$SERVER_PORT" -sTCP:LISTEN 2>/dev/null || true)
  [[ -n "$pids" ]] && kill $pids 2>/dev/null
  for _ in $(seq 1 20); do lsof -nP -iTCP:"$SERVER_PORT" -sTCP:LISTEN >/dev/null 2>&1 || return 0; sleep 0.25; done
}

teardown() {
  set +e
  [[ -n "$server_pid" ]] && kill "$server_pid" 2>/dev/null && wait "$server_pid" 2>/dev/null
  kill_port_server
  if [[ -d "$DIR" ]]; then (cd "$DIR" && docker compose down -v --remove-orphans >/dev/null 2>&1); fi
  docker compose -p "$PROJECT" down -v --remove-orphans >/dev/null 2>&1
  rm -rf "$DIR"
  say ""
  say "teardown: project $PROJECT removed, $DIR deleted"
  say "SUMMARY: $pass passed, $fail failed (transcript: $LOG)"
}
trap teardown EXIT

for t in git docker make jq curl psql openssl go lsof; do command -v "$t" >/dev/null || { say "missing tool: $t"; exit 2; }; done
[[ "$PROJECT" != frv2e2e ]] || { say "refusing to use the e2e project name"; exit 2; }
for p in $DB_PORT $REDIS_PORT $IAMKIT_PORT $SERVER_PORT; do
  lsof -nP -iTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1 && { say "port $p busy (set WS12_*_PORT)"; exit 2; }
done

step "fresh clone"
docker compose -p "$PROJECT" down -v --remove-orphans >/dev/null 2>&1 || true
rm -rf "$DIR"
git clone --quiet "$REPO" "$DIR"
say "  cloned $(git -C "$DIR" rev-parse --short HEAD) into $DIR"

# The base compose file hard-codes redis on 127.0.0.1:6379. Record whether that
# collides here, then work around it. Override files cannot fix it on Compose
# < 2.24 (`!override` appends, `!reset` is ignored for ports in a merged file),
# so the throwaway clone's docker-compose.yml is patched in place.
if lsof -nP -iTCP:6379 -sTCP:LISTEN >/dev/null 2>&1; then
  say "  FINDING: host port 6379 is busy and docker-compose.yml hard-codes redis to it (no REDIS_PORT knob)"
fi
sed -i.bak "s|\"127.0.0.1:6379:6379\"|\"127.0.0.1:${REDIS_PORT}:6379\"|" "$DIR/docker-compose.yml" && rm -f "$DIR/docker-compose.yml.bak"
grep -q "127.0.0.1:${REDIS_PORT}:6379" "$DIR/docker-compose.yml" || { say "could not patch redis port"; exit 2; }

# Ports as a user would configure them: edit .env after `make setup`.
configure_env() {
  local f="$DIR/.env" tmp
  tmp=$(mktemp)
  awk -v db="$DB_PORT" -v rp="$REDIS_PORT" -v ip="$IAMKIT_PORT" -v sp="$SERVER_PORT" -v ae="$ADMIN_EMAIL" -v ap="$ADMIN_PASSWORD" '
    /^SERVER_PORT=/            {print "SERVER_PORT=" sp; next}
    /^DB_PORT=/                {print "DB_PORT=" db; next}
    /^REDIS_ADDR=/             {print "REDIS_ADDR=localhost:" rp; next}
    /^IAMKIT_BASE_URL=/        {print "IAMKIT_BASE_URL=http://localhost:" ip; next}
    /^IAMKIT_JWT_ISSUER=/      {print "IAMKIT_JWT_ISSUER=http://localhost:" ip; next}
    /^IAMKIT_PORT=/            {print "IAMKIT_PORT=" ip; next}
    /^FREEROUTER_ADMIN_EMAIL=/    {print "FREEROUTER_ADMIN_EMAIL=" ae; next}
    /^FREEROUTER_ADMIN_PASSWORD=/ {print "FREEROUTER_ADMIN_PASSWORD=" ap; next}
    {print}' "$f" > "$tmp" && cat "$tmp" > "$f" && rm -f "$tmp"
  echo "COMPOSE_PROJECT_NAME=$PROJECT" >> "$f"
  sed -i.bak "s|^IAMKIT_SRC=.*|IAMKIT_SRC=$IAMKIT_SRC_DIR|" "$f" && rm -f "$f.bak"
}

step "make setup"
run setup make setup && ok "make setup created .env + secrets/jwt.pem" || bad "make setup failed"
[[ -f "$DIR/.env" && -f "$DIR/secrets/jwt.pem" ]] && ok ".env and secrets/jwt.pem exist" || bad ".env or jwt.pem missing"
[[ "$(stat -f %Lp "$DIR/.env" 2>/dev/null || stat -c %a "$DIR/.env")" == 600 ]] && ok ".env is mode 600" || bad ".env not mode 600"
grep -q '^ENCRYPTION_KEY=[0-9a-f]\{64\}$' "$DIR/.env" && ok "ENCRYPTION_KEY generated (64 hex)" || bad "ENCRYPTION_KEY not generated"
configure_env
(cd "$DIR" && docker compose config >/dev/null 2>&1) && ok "compose config valid" || bad "compose config invalid"
(cd "$DIR" && docker compose config 2>/dev/null | grep -q "published: \"6379\"") && bad "redis still publishes 6379" || ok "redis publishes only $REDIS_PORT"

step "missing IAMKIT_SRC → clear error"
if run nosrc env IAMKIT_SRC=/nonexistent/iam make init; then
  bad "make init succeeded with IAMKIT_SRC=/nonexistent/iam"
else
  grep -iE "nonexistent/iam|IAMKIT_SRC" /tmp/ws12-nosrc.out | head -3 | sed 's/^/    | /' | tee -a "$LOG"
  if grep -q "IAMKIT_SRC" /tmp/ws12-nosrc.out; then ok "error names IAMKIT_SRC"
  elif grep -q "/nonexistent/iam" /tmp/ws12-nosrc.out; then
    bad "FINDING: missing IAMKIT_SRC fails with a raw docker error (path shown, variable not named)"
  else bad "missing IAMKIT_SRC error mentions neither the path nor the variable"; fi
fi
(cd "$DIR" && docker compose down -v >/dev/null 2>&1) || true

step "make init with ports set only in .env (as .env.example suggests)"
if run init-envonly make init; then
  ok "make init honours ports from .env"
else
  grep -m3 -E "psql|error|ERROR|refused" /tmp/ws12-init-envonly.out | sed 's/^/    | /' | tee -a "$LOG"
  if grep -q "port 5432" /tmp/ws12-init-envonly.out; then
    bad "FINDING: make migrate ignores DB_PORT from .env (Makefile never loads .env) — non-default ports need exporting"
  else
    bad "make init with .env-only ports failed"
  fi
fi
run reset0 make reset >/dev/null || true

# Workaround for the finding above: export the DB settings for make's recipes.
export DB_PORT

step "make init (first run on fresh clone)"
if run init1 make init; then ok "make init succeeded end to end"; else bad "make init failed"; tail -20 /tmp/ws12-init1.out | sed 's/^/    | /' | tee -a "$LOG"; exit 1; fi
grep -q "FreeRouter initialized" /tmp/ws12-init1.out && ok "prints success banner" || bad "no success banner"
grep -qE "ERROR:|psql:.*error" /tmp/ws12-init1.out && bad "migration errors on a fresh DB: $(grep -m1 -E 'ERROR:|psql:.*error' /tmp/ws12-init1.out)" || ok "migrations ran without SQL errors"
for k in IAMKIT_ENVIRONMENT_ID IAMKIT_APPLICATION_ID IAMKIT_RESOURCE_ID IAMKIT_ORGANIZATION_ID; do
  grep -qE "^$k=[0-9a-f-]{36}$" "$DIR/.env" && ok "$k written to .env" || bad "$k missing in .env"
done
grep -q "^IAMKIT_SERVICE_SECRET=ik_svc_" "$DIR/.env" && ok "IAMKIT_SERVICE_SECRET is ik_svc_" || bad "IAMKIT_SERVICE_SECRET missing"
grep -q "^IAMKIT_AUDIENCE=http://localhost:$SERVER_PORT$" "$DIR/.env" && ok "IAMKIT_AUDIENCE follows SERVER_PORT" || bad "IAMKIT_AUDIENCE wrong: $(grep ^IAMKIT_AUDIENCE "$DIR/.env")"
[[ -s "$DIR/.dev-secrets/admin-service-account" ]] && ok "admin key file written" || bad "no admin key file"
[[ "$(stat -f %Lp "$DIR/.dev-secrets" 2>/dev/null || stat -c %a "$DIR/.dev-secrets")" == 700 ]] && ok ".dev-secrets is mode 700" || bad ".dev-secrets not 700"
if grep -qE "ik_(svc|mgmt)_[A-Za-z0-9]{8,}" /tmp/ws12-init1.out; then bad "FINDING: make init output contains a raw credential"; else ok "no credentials printed"; fi
grep -q "localhost:$SERVER_PORT/api/v1/providers" /tmp/ws12-init1.out && ok "printed curl uses SERVER_PORT $SERVER_PORT" || bad "printed curl does not use SERVER_PORT"
grep -q "localhost:8080" /tmp/ws12-init1.out && say "  note: output still mentions localhost:8080" || true

verify_instructions() { # follows the printed instructions literally
  local tag=$1 envid appid resid orgid aud admin jwt code body
  envid=$(grep ^IAMKIT_ENVIRONMENT_ID= "$DIR/.env" | cut -d= -f2); appid=$(grep ^IAMKIT_APPLICATION_ID= "$DIR/.env" | cut -d= -f2)
  resid=$(grep ^IAMKIT_RESOURCE_ID= "$DIR/.env" | cut -d= -f2);    orgid=$(grep ^IAMKIT_ORGANIZATION_ID= "$DIR/.env" | cut -d= -f2)
  aud=$(grep ^IAMKIT_AUDIENCE= "$DIR/.env" | cut -d= -f2-)
  grep -q "VITE_IAMKIT_ENVIRONMENT_ID=$envid" /tmp/ws12-$tag.out && grep -q "VITE_IAMKIT_ORGANIZATION_ID=$orgid" /tmp/ws12-$tag.out \
    && grep -q "VITE_IAMKIT_APPLICATION_ID=$appid" /tmp/ws12-$tag.out && grep -q "VITE_IAMKIT_RESOURCE_ID=$resid" /tmp/ws12-$tag.out \
    && ok "printed web/.env ids match .env" || bad "printed web/.env ids differ from .env"

  # "Restart FreeRouter so it picks up the ids: make dev"
  (cd "$DIR" && exec make dev) >"/tmp/ws12-dev-$tag.out" 2>&1 &
  server_pid=$!
  for _ in $(seq 1 120); do curl -sf "localhost:$SERVER_PORT/health" >/dev/null && break; sleep 0.5; done
  curl -sf "localhost:$SERVER_PORT/health" >/dev/null && ok "make dev serves /health on :$SERVER_PORT" || { bad "make dev did not come up"; tail -5 "/tmp/ws12-dev-$tag.out" | tee -a "$LOG"; return; }

  # printed: curl -H "Authorization: Bearer $(cat .dev-secrets/admin-service-account)" localhost:PORT/api/v1/providers
  admin=$(cat "$DIR/.dev-secrets/admin-service-account")
  body=$(curl -s -w '\n%{http_code}' -H "Authorization: Bearer $admin" "localhost:$SERVER_PORT/api/v1/providers")
  code=${body##*$'\n'}; body=${body%$'\n'*}
  [[ $code == 200 ]] && ok "admin key → /api/v1/providers 200 ($(jq -r '.page.total' <<<"$body") seeded providers)" || bad "admin key → /api/v1/providers $code $body"

  # console admin (FREEROUTER_ADMIN_EMAIL) logs in at IAMKit with the printed boundary ids
  body=$(jq -n --arg e "$ADMIN_EMAIL" --arg p "$ADMIN_PASSWORD" --arg env "$envid" --arg org "$orgid" --arg app "$appid" --arg res "$resid" \
    '{environment_id:$env,organization_id:$org,application_id:$app,resource_id:$res,email:$e,password:$p}' \
    | curl -s -w '\n%{http_code}' -H 'Content-Type: application/json' --data-binary @- "localhost:$IAMKIT_PORT/identity/v1/login")
  code=${body##*$'\n'}; body=${body%$'\n'*}
  jwt=$(jq -r '.access_token // empty' <<<"$body" 2>/dev/null)
  if [[ $code == 200 && -n $jwt ]]; then ok "console admin logs in at IAMKit :$IAMKIT_PORT"; else bad "console admin login $code $body"; fi
  if [[ -n $jwt ]]; then
    code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $jwt" "localhost:$SERVER_PORT/api/v1/providers")
    [[ $code == 200 ]] && ok "console admin JWT → /api/v1/providers 200" || bad "console admin JWT → /api/v1/providers $code"
    code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $jwt" "localhost:$SERVER_PORT/api/v1/access/users")
    [[ $code == 200 ]] && ok "console admin JWT → /api/v1/access/users 200 (backend service account works)" || bad "console admin JWT → /api/v1/access/users $code"
  fi
  # aud check (the resource audience is what FreeRouter validates)
  if [[ -n $jwt ]]; then
    local payload jaud
    payload=$(cut -d. -f2 <<<"$jwt" | tr '_-' '/+')
    while (( ${#payload} % 4 )); do payload+="="; done
    jaud=$(base64 -d <<<"$payload" 2>/dev/null | jq -r '.aud | if type=="array" then .[0] else . end' 2>/dev/null || true)
    [[ "$jaud" == "$aud" ]] && ok "JWT audience = IAMKIT_AUDIENCE" || bad "JWT aud '$jaud' != IAMKIT_AUDIENCE '$aud'"
  fi
  grep -qE "ik_svc_[A-Za-z0-9]{8,}|$ADMIN_PASSWORD" "/tmp/ws12-dev-$tag.out" && bad "server log contains a secret" || ok "server log has no secrets"
  kill_port_server
  kill "$server_pid" 2>/dev/null; wait "$server_pid" 2>/dev/null || true; server_pid=""
}

step "follow the printed instructions"
verify_instructions init1

step "make init again → bootstrap refuses cleanly"
if run init2 make init; then bad "second make init succeeded (should refuse)"; else
  grep -m1 "already provisioned" /tmp/ws12-init2.out | sed 's/^/    | /' | tee -a "$LOG"
  grep -q "IAMKit already provisioned (IAMKIT_ENVIRONMENT_ID set in .env). Run 'make reset' for a fresh stack." /tmp/ws12-init2.out \
    && ok "second make init refuses with a readable message" || bad "second make init failed without the expected message"
  n=$(grep -c "ERROR:" /tmp/ws12-init2.out || true)
  [[ $n -gt 0 ]] && say "  note: second make init re-applies every migration: $n SQL 'ERROR:' lines (psql continues, exit 0) before bootstrap refuses" || true
fi
if run bootstrap2 make bootstrap; then bad "second make bootstrap succeeded"; else
  grep -q "already provisioned" /tmp/ws12-bootstrap2.out && ok "make bootstrap refuses (exit non-zero, readable)" || bad "make bootstrap failed without message"
fi
grep -q "^IAMKIT_SERVICE_SECRET=ik_svc_" "$DIR/.env" && ok ".env intact after refused re-run" || bad ".env damaged by refused re-run"

step "make reset && make init"
run reset1 make reset && ok "make reset" || bad "make reset failed"
grep -qE "^IAMKIT_(ENVIRONMENT_ID|SERVICE_SECRET)=" "$DIR/.env" && bad "make reset left provisioned ids in .env" || ok "make reset removed provisioned ids"
[[ -d "$DIR/.dev-secrets" ]] && bad ".dev-secrets survived reset" || ok ".dev-secrets removed"
[[ -z "$(docker volume ls -q --filter "name=${PROJECT}_pgdata")" ]] && ok "pgdata volume dropped" || bad "pgdata volume still present"
if run init3 make init; then ok "make init after reset succeeded"; else bad "make init after reset failed"; tail -20 /tmp/ws12-init3.out | sed 's/^/    | /' | tee -a "$LOG"; exit 1; fi
verify_instructions init3

[[ $fail -eq 0 ]]
