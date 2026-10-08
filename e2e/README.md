# e2e — functional test harness

Black-box tests that drive FreeRouter the way a human would: the real server,
the real console, the real IAMKit, a scripted fake LLM upstream and a webhook
sink. Plan: `~/.rness/plans/freerouterv2/e2e-functional-testing.md`.

## Run

```sh
e2e/up.sh                       # boots isolated stack (docker project frv2e2e) + seeds
go test -tags e2e -count=1 ./e2e/api/...          # API tests (Go, build tag e2e)
(cd e2e/ui && npx playwright test)                # console tests (Playwright)
e2e/down.sh                     # tears everything down (drops volumes)
```

Ports (override with `E2E_*_PORT`): postgres 25432, redis 16379, iamkit 28080,
server 23000, console 25173, fakellm 29100, webhook sink 29200. Nothing clashes
with `make up`.

State lives in `e2e/.run/` (git-ignored): `.env`, `secrets/`, logs, binaries
and **`fixtures.json`** — the single source of ids, URLs, personas and secrets
for the tests. `up.sh` is idempotent (restarts host processes, skips
provisioning/seeding when already done). Delete `e2e/.run/fixtures.json` to
reseed; `down.sh` for a clean slate.

## Fixtures (`e2e/.run/fixtures.json`)

| Key | What |
|-----|------|
| `urls.*` | server, api (`/api/v1`), gateway (`/v1`), iamkit, web, fakellm, webhook_sink, db, redis |
| `personas.admin` | console user, all freerouter permissions |
| `personas.viewer` | user with every `*:read` |
| `personas.providers_only` | user with `providers:read/write` only |
| `personas.noperm` | org member, no role |
| `personas.suspended` | deactivated user |
| `personas.foreign` | user in another organization |
| `personas.admin_key` | `ik_svc_` with all permissions (setup/teardown) |
| `personas.gw_key` | `ik_svc_` with `gateway:invoke` only |
| `personas.noperm_key` | `ik_svc_` with `metrics:read` only |
| `personas.expired_key` | expired `ik_svc_` |
| `boundary.*` | foreign role / org / user ids and the backend service-account id (must be invisible) |
| `providers.*`, `provider_keys.*` | one provider per protocol (openai, anthropic, google, cohere) → fakellm |
| `models.*` | `e2e-ok`, `e2e-ok-{anthropic,google,cohere}`, `e2e-500`, `e2e-429`, `e2e-400`, `e2e-slow`, `e2e-stream-abort`, `e2e-malformed`, `e2e-nousage`, `e2e-flaky` (fails 2x), `e2e-fallback` (500 → falls back to e2e-ok) |

## fakellm (`e2e/fakellm`)

Behaviour is chosen by the upstream model name (the mapping's `external_id`):
`fake-ok`, `fake-500`, `fake-429`, `fake-400`, `fake-slow`, `fake-stream-abort`,
`fake-malformed`, `fake-nousage`, `fake-flaky-N`. Speaks OpenAI, Anthropic,
Google and Cohere shapes (sync + SSE). Introspection: `GET /_requests`,
`DELETE /_requests`, `POST /_reset`.

## webhooksink (`e2e/webhooksink`)

`POST /hook/<name>` records and returns 200; `POST /hook/fail/<n>/<name>`
returns 500 for the first n deliveries. `GET|DELETE /_deliveries`.

## Writing tests

- API: `e2e/api/harness.go` — `FX(t)`, `AdminAPI(t)`, `AsUser(t, "viewer")`,
  `Bearer(url, secret)`, `Anon(url)`, `Management(t)`, `Eventually`, fakellm /
  sink helpers. One file per workstream (`ws01_authn_test.go`, …); every test
  cleans up what it creates (use `t.Cleanup`) and never mutates seeded objects.
- Console: `e2e/ui/tests/helpers.ts` — `fx`, `login`, `loginAndWait`,
  `adminApi(request)`, `uniq`. One spec per workstream.
- Findings (bugs, UX gaps, security notes) go in
  `~/.rness/plans/freerouterv2/e2e-findings/WS-XX.md`, not in the repo.
