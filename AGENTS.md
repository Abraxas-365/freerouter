<!-- FOR AI AGENTS - Human readability is a side effect, not a goal -->
<!-- Managed by agent: keep sections and order; edit content, not structure -->
<!-- Last updated: 2026-09-19 | Last verified: 2026-09-19 -->

# AGENTS.md

**Precedence:** the **closest `AGENTS.md`** to the files you're changing wins. Root holds global defaults only.

## Commands
> Source: Makefile

<!-- AGENTS-GENERATED:START commands -->
| Task | Command | ~Time |
|------|---------|-------|
| Vet | `go vet ./...` | ~3s |
| Lint | `golangci-lint run` | ~10s |
| Format | `go fmt ./...` | ~2s |
| Test (single pkg) | `go test -v ./internal/path/to/pkg/...` | ~2s (unit) / ~1-2s per container (integration) |
| Test (all — needs Docker) | `go test ./...` or `make test` | ~90-120s |
| Test (race) | `make test-race` | ~2x plain |
| Build | `go build -o bin/server ./cmd/server` | ~5s |
| Dev server | `make dev` | ongoing |
| Migrate | `make migrate` | ~2s |
| Services up | `make up` (docker compose) | ~5s |
| Full setup | `make init` (jwt-key + services + migrate) | ~15s |
<!-- AGENTS-GENERATED:END commands -->

## Testing
- **Requires Docker** — integration/e2e tests spin real disposable containers via `testcontainers-go` (Postgres 16, Redis 7). No mocks for DB/cache.
- **Unit tests** (no Docker): `internal/errx`, `internal/identity`, `internal/guardrail` (detectors), `internal/webhook` + `internal/webhook/webhooksvc` (URL validation, SSRF guard, HMAC signing) — pure logic, run in milliseconds.
- **Integration tests** (Docker, one container per test function): `internal/provider/adapters/providerpg`, `internal/webhook/adapters/webhookpg`, `internal/guardrail/adapters/guardrailpg`, `internal/ratelimit/adapters/ratelimitredis` — exercise real repos/limiter against a real Postgres/Redis instance.
- **E2E test**: `internal/gateway/e2e_test.go` — wires provider/providerkey/usage/ratelimit/guardrail modules with real Postgres+Redis, points routing at an `httptest.Server` fake upstream (no real provider calls), and drives the actual `gatewayhttp.Handler` through a Fiber app via `app.Test(req, -1)`. Covers: happy path, fallback-on-retryable-error, guardrail block, unknown model 404, `/v1/models` listing.
- **Shared fixtures**: `internal/testutil.PostgresDB(t)` (spins container, runs every `migrations/*.up.sql` in filename order, returns `*sqlx.DB`) and `internal/testutil.RedisClient(t)` (spins container, returns `*redis.Client`). Both auto-register `t.Cleanup` teardown — no manual container management needed in test code.
- Each test function gets its **own** disposable container (~1-1.5s startup overhead) — no shared test-suite state, no cross-test pollution.
- Async paths (usage logging, guardrail violation logging) are fire-and-forget — tests that assert on them must poll with a timeout (see `waitForUsageLog` in `internal/gateway/e2e_test.go`), not assume immediate consistency.
- When adding a new domain module: put unit tests next to pure-logic files (detectors, validators, transport helpers), and Postgres-repo tests in `adapters/<domain>pg/repository_test.go` using `testutil.PostgresDB(t)`, following `internal/provider/adapters/providerpg/repository_test.go` as the template.

## Response Style
- Answer first, elaborate only if needed. No sycophantic openers.
- For yes/no or status questions, lead with the answer.
- Skip preamble. Match response length to task complexity.

## Workflow
1. **Before coding**: Read nearest `AGENTS.md` + check Golden Samples for the area you're touching
2. **After each change**: Run the smallest relevant check (vet → lint → single test)
3. **Before committing**: Run full test suite if changes affect >2 files or touch shared code
4. **Before claiming done**: Run verification and **show output as evidence**

## File Map
<!-- AGENTS-GENERATED:START filemap -->
```
cmd/server/           -> Entrypoint: signal handling, config load, container init
internal/
  bootstrap/          -> Composition root: wires infra + modules, registers routes
  config/             -> Config structs loaded from env vars
  errx/               -> Typed error system: Error struct, HTTP status mapping
  identity/           -> Typed generic IDs (ProviderID, ModelID, MappingID, ModelFallbackID, etc.)
  query/              -> Pagination types: Pagination, Paginated[T] (transport-agnostic)
  httpx/              -> HTTP helpers: PaginationFromCtx (Fiber query params → query.Pagination)
  server/             -> Fiber server, error middleware, IAMKit auth middleware
  provider/           -> LLM provider domain module (providers, models, mappings, fallbacks)
    provider.go       -> Provider entity + Status enum + Create/Update/Filter
    model.go          -> Model entity + Status/Stability enums + Create/Update/Filter
    mapping.go        -> ModelProviderMapping entity (pricing, capabilities) + Create/Update/Filter
    fallback.go       -> ModelFallback entity + Create
    ports.go          -> All interfaces: Commands/Queries/Repository per aggregate
    adapters/
      providerhttp/   -> Inbound: HTTP handlers (providers, models, mappings, fallbacks)
      providerpg/     -> Outbound: PostgreSQL repositories (4 repos)
    providersvc/      -> Service: business logic for all aggregates
    providermodule/   -> Assembler: wires repos → svc → handler
  providerkey/        -> Encrypted upstream API keys per provider
    providerkey.go    -> ProviderKey entity + KeyStatus enum + Create/Update/Filter
    ports.go          -> Commands/Queries/Repository + TokenEncryptor interface
    adapters/
      providerkeyhttp/   -> Inbound: HTTP handlers
      providerkeypg/     -> Outbound: PostgreSQL repository
      providerkeyinfra/  -> Token encryption (NaCl secretbox)
    providerkeysvc/      -> Service: encrypt-on-write, decrypt-on-read
    providerkeymodule/   -> Assembler: wires encryptor + repo → svc → handler
  gateway/            -> LLM proxy layer: routing, translation, upstream calls
    gateway.go        -> ChatRequest/Response, RouteResult, Usage, ModelList types
    modalities.go     -> Transcription, Speech, Moderation, Rerank types + cost calc
    profiles.go       -> Provider profiles registry (URL builders, auth styles)
    translator.go     -> ProviderTranslator interface
    translator_openai.go     -> OpenAI passthrough translator
    translator_anthropic.go  -> Anthropic Messages API translator
    translator_google.go     -> Google Gemini API translator
    translator_cohere.go     -> Cohere v2 API translator
    upstream.go       -> HTTP client: Call (sync), Stream (SSE), CallRaw
    router.go         -> Model→provider+key resolution with fallback
    keyhealth.go      -> In-memory key health tracker (sliding window)
    retry.go          -> Retryable status codes, exponential backoff
    strategy.go       -> Routing strategies (cheapest, lowest-latency, round-robin)
    adapters/
      gatewayhttp/    -> Inbound: OpenAI-compatible endpoints (/v1/chat/completions, /v1/models, modalities)
    gatewaymodule/    -> Assembler: wires router + upstream + health → handler
    e2e_test.go       -> Full-stack test: real Postgres+Redis + fake upstream httptest.Server, drives gatewayhttp.Handler
  usage/             -> Usage logging: async request logging, summaries
    usage.go         -> UsageLog entity, Filter, Summary, ModelSummary types
    ports.go         -> Commands (LogRequest), Queries (Find/List/Summary), Repository
    adapters/
      usagehttp/     -> Inbound: HTTP handlers (list, detail, summary)
      usagepg/       -> Outbound: PostgreSQL repository
    usagesvc/        -> Service: async buffered logging, query methods
    usagemodule/     -> Assembler: wires repo → svc → handler
  ratelimit/         -> Rate limiting: per-subject RPM + concurrency enforcement
    ratelimit.go     -> RateLimitConfig entity, Create/Update/Filter, RateLimitResult
    ports.go         -> Commands/Queries/Repository + Limiter interface
    adapters/
      ratelimithttp/    -> Inbound: HTTP handlers (config CRUD)
      ratelimitpg/      -> Outbound: PostgreSQL repository (config storage)
      ratelimitredis/   -> Outbound: Redis limiter (sliding window RPM + concurrency)
    ratelimitsvc/    -> Service: config CRUD + cached Check/Release for gateway
    ratelimitmodule/ -> Assembler: wires pg repo + redis limiter → svc → handler
  guardrail/         -> Content safety: system detectors + custom rules + violation log
    guardrail.go     -> GuardrailConfig/GuardrailRule/GuardrailViolation entities, CheckResult, Action enum
    detector.go      -> Regex-based PII/secrets/jailbreak/prompt-injection detectors + redaction
    ports.go         -> Commands/Queries/Repository + Evaluator interface (gateway-facing)
    adapters/
      guardrailhttp/    -> Inbound: HTTP handlers (config, custom rules, violations)
      guardrailpg/      -> Outbound: PostgreSQL repository (config, rules, violations)
    guardrailsvc/    -> Service: config/rule CRUD + CheckMessages evaluation logic
    guardrailmodule/ -> Assembler: wires repo → svc → handler
  webhook/           -> Outbound event notifications: subscriptions + signed delivery + retry
    webhook.go       -> WebhookConfig/WebhookDelivery/WebhookPayload entities, event type constants
    ports.go         -> Commands/Queries/Repository + Dispatcher interface (gateway-facing Fire)
    adapters/
      webhookhttp/      -> Inbound: HTTP handlers (subscription CRUD, deliveries, test-fire)
      webhookpg/        -> Outbound: PostgreSQL repository (configs, deliveries)
    webhooksvc/      -> Service: CRUD + HMAC-signed delivery, SSRF-hardened client, retry worker
    webhookmodule/   -> Assembler: wires repo → svc → handler; StartWorker/Stop lifecycle
  apikey/            -> API key management: IAMKit service-account passthrough (no local DB)
    apikey.go        -> APIKey/APIKeyCredential/CreateAPIKey entities
    ports.go         -> Commands/Queries/Store interfaces (Store = IAMKit adapter)
    adapters/
      apikeyhttp/    -> Inbound: HTTP handlers (create, list, revoke)
      apikeyiamkit/  -> Outbound: IAMKit management SDK adapter (service accounts)
    apikeysvc/       -> Service: validate + delegate to IAMKit store
    apikeymodule/    -> Assembler: wires IAMKit store → svc → handler
  testutil/          -> Test-only fixtures: PostgresDB(t)/RedisClient(t) (testcontainers, auto-migrate, auto-cleanup)
migrations/           -> SQL migrations (run in order)
docker-compose.yml    -> Postgres 16 + Redis 7 + IAMKit
Makefile              -> Dev commands
```
<!-- AGENTS-GENERATED:END filemap -->

## Golden Samples (follow these patterns)
<!-- AGENTS-GENERATED:START golden-samples -->
| For | Reference | Key patterns |
|-----|-----------|--------------|
| Domain entity + validation | `internal/provider/provider.go` | Typed IDs, Status enum, Create/Update structs with Validate(), Filter struct |
| Multi-aggregate module | `internal/provider/model.go`, `mapping.go`, `fallback.go` | Multiple entities in one module, per-aggregate Commands/Queries/Repository |
| Ports (interfaces) | `internal/provider/ports.go` | Per-aggregate Commands/Queries/Repository segregation, query.Pagination/Paginated[T] |
| Service | `internal/provider/providersvc/service.go` | Constructor injection, validate-then-delegate, `var _` checks, multi-repo |
| PostgreSQL repo | `internal/provider/adapters/providerpg/repository.go` | Multiple repos in one file, sqlx, errx wrapping, pq conflict detection, dynamic WHERE + COUNT/LIMIT/OFFSET, query.Page envelope |
| HTTP handler | `internal/provider/adapters/providerhttp/handler.go` | Multi-resource routes, Parse → call → respond, httpx.PaginationFromCtx, filter from query params |
| Module assembler | `internal/provider/providermodule/module.go` | Only place importing concrete adapters |
| Typed ID | `internal/identity/entities.go` | Tag + alias + New/Parse/MustParse trio |
| Error constructors | `internal/errx/common.go` | Validation/NotFound/Forbidden/RateLimited/Conflict/etc. |
| Gateway router | `internal/gateway/router.go` | Model→provider+key resolution, health-based key selection, fallback chain |
| Gateway handler | `internal/gateway/adapters/gatewayhttp/handler.go` | OpenAI-compatible endpoints, retry with fallback, SSE streaming, rate limiting, guardrails, usage logging, webhook events |
| Provider profiles | `internal/gateway/profiles.go` | Per-provider auth style, URL builder, translator registry |
| Provider translator | `internal/gateway/translator_anthropic.go` | OpenAI↔native format conversion (request, response, streaming events) |
| Usage entity | `internal/usage/usage.go` | UsageLog entity with token/cost/timing, Filter, Summary aggregation types |
| Async logging | `internal/usage/usagesvc/service.go` | Channel-buffered non-blocking LogRequest, background goroutine persistence |
| Rate limit entity | `internal/ratelimit/ratelimit.go` | RateLimitConfig entity, Create/Update/Filter, RateLimitResult, defaults |
| Rate limit ports | `internal/ratelimit/ports.go` | Commands/Queries/Repository + Limiter interface (Redis-backed) |
| Rate limit service | `internal/ratelimit/ratelimitsvc/service.go` | Config cache (30s TTL), Check/Release for gateway, subject→config resolution |
| Redis limiter | `internal/ratelimit/adapters/ratelimitredis/limiter.go` | Sorted-set sliding window RPM + INCR/DECR concurrency counter |
| Guardrail entity | `internal/guardrail/guardrail.go` | GuardrailConfig (global singleton), GuardrailRule, CheckResult, Action enum |
| Guardrail detectors | `internal/guardrail/detector.go` | Regex detector framework, PII/secrets/jailbreak/injection patterns, redaction |
| Guardrail service | `internal/guardrail/guardrailsvc/service.go` | Config/rule CRUD + CheckMessages evaluation, async violation logging |
| Webhook entity | `internal/webhook/webhook.go` | WebhookConfig/WebhookDelivery/WebhookPayload, event type constants + IsValidEvent |
| Webhook transport | `internal/webhook/webhooksvc/transport.go` | SSRF-hardened HTTP client: DNS-pinned dial, public-IP allowlist, no redirect follow |
| Webhook service | `internal/webhook/webhooksvc/service.go` | Config/delivery CRUD, HMAC-SHA256 signing, async Fire, background retry worker |
| API key entity | `internal/apikey/apikey.go` | APIKey/APIKeyCredential, CreateAPIKey + Validate(), no local DB |
| IAMKit adapter (non-DB) | `internal/apikey/adapters/apikeyiamkit/store.go` | Outbound adapter calling IAMKit management SDK, wraps errors via errx |
| API key module | `internal/apikey/apikeymodule/module.go` | No DB, no migration — IAMKit-backed assembler |
<!-- AGENTS-GENERATED:END golden-samples -->

## Utilities (check before creating new)
<!-- AGENTS-GENERATED:START utilities -->
| Need | Use | Location |
|------|-----|----------|
| Typed UUID ID | `identity.ID[T]` | `internal/identity/id.go` |
| Pagination types | `query.Pagination`, `query.Page`, `query.Paginated[T]` | `internal/query/query.go` |
| Parse pagination from request | `httpx.PaginationFromCtx(c)` | `internal/httpx/pagination.go` |
| Application error | `errx.Validation(msg)` etc. | `internal/errx/common.go` |
| Error wrapping | `errx.Wrap(err, msg, type)` | `internal/errx/error.go` |
| IAMKit auth | `server.AuthMiddleware(...)` | `internal/server/auth.go` |
| Permission check | `server.RequirePermissions(...)` | `internal/server/auth.go` |
| Get JWT claims | `server.Claims(c)` | `internal/server/auth.go` |
| Token encryption | `providerkeyinfra.NewEncryptor(hexKey)` | `internal/providerkey/adapters/providerkeyinfra/encryptor.go` |
| Config loading | `config.Load()` | `internal/config/config.go` |
| Provider profile | `gateway.GetProfile(slug)` | `internal/gateway/profiles.go` |
| Provider translator | `gateway.GetTranslator(slug)` | `internal/gateway/translator.go` |
| Key health tracking | `gateway.NewKeyHealthTracker()` | `internal/gateway/keyhealth.go` |
| Retry helpers | `gateway.IsRetryable(status)`, `gateway.RetryDelay(n)` | `internal/gateway/retry.go` |
| Route resolution | `gateway.Router.Resolve(ctx, model)` | `internal/gateway/router.go` |
| Usage logging | `usage.Commands.LogRequest(log)` | `internal/usage/ports.go` |
| Usage queries | `usage.Queries.List/Find/GetSummary` | `internal/usage/ports.go` |
| Rate limit check | `ratelimitsvc.Service.Check(ctx, subject)` | `internal/ratelimit/ratelimitsvc/service.go` |
| Rate limit release | `ratelimitsvc.Service.Release(ctx, subject)` | `internal/ratelimit/ratelimitsvc/service.go` |
| Rate limit config CRUD | `ratelimit.Commands.Create/Update/Delete` | `internal/ratelimit/ports.go` |
| Rate limit queries | `ratelimit.Queries.Find/FindBySubject/List` | `internal/ratelimit/ports.go` |
| Guardrail check (gateway) | `guardrail.Evaluator.CheckMessages(ctx, texts, model)` | `internal/guardrail/ports.go` |
| Guardrail config/rule CRUD | `guardrail.Commands.UpsertConfig/CreateRule/UpdateRule/DeleteRule` | `internal/guardrail/ports.go` |
| Guardrail detectors | `guardrail.CheckPII/CheckSecrets/CheckJailbreak/CheckInjection` | `internal/guardrail/detector.go` |
| Webhook fire (gateway) | `webhook.Dispatcher.Fire(event, data)` | `internal/webhook/ports.go` |
| Webhook subscription CRUD | `webhook.Commands.Create/Update/Delete` | `internal/webhook/ports.go` |
| Webhook event types | `webhook.EventRequestCompleted/EventRequestFailed/EventKeyHealthDegraded/EventKeyBlacklisted` | `internal/webhook/webhook.go` |
| API key CRUD | `apikey.Commands.Create/Revoke`, `apikey.Queries.List` | `internal/apikey/ports.go` |
<!-- AGENTS-GENERATED:END utilities -->

## Heuristics (quick decisions)
<!-- AGENTS-GENERATED:START heuristics -->
| When | Do |
|------|-----|
| New domain module | Mirror `internal/provider/` exactly: ports.go, entity.go, adapters/, *svc/, *module/ |
| New entity type | Add tag + alias + trio in `internal/identity/entities.go` |
| New error kind | Use `errx.Validation/NotFound/Conflict/...` — never bare `errors.New` |
| Repo wraps DB error | Use `errx.Wrap(err, msg, errx.TypeInternal)` — detect pq unique violations as Conflict |
| Cross-module dependency | Pass interface (Commands/Queries) through module Deps, never concrete types |
| Auth on a route | Add `server.AuthMiddleware(...)` to group, then `server.RequirePermissions(...)` per route |
| Adding dependency | Ask first — we minimize deps |
| Unsure about pattern | Check Golden Samples above |
<!-- AGENTS-GENERATED:END heuristics -->

## Architecture: Hexagonal (Ports & Adapters)

### Five Pillars
1. **Typed generic IDs** — `identity.ID[T]` phantom-typed UUIDs, compile-time safety
2. **One typed error system** — `errx.Error` with automatic HTTP status mapping
3. **Ports & adapters per module** — `ports.go` (interfaces), `adapters/` (I/O), `*svc` (logic), `*module` (wiring)
4. **Commands/Queries/Repository** — read/write interface segregation
5. **Validation on domain structs** — `Create.Validate()` / `Update.Validate()`, never in handlers

### Module Shape
```
internal/<domain>/
  ports.go                    Interfaces: Commands, Queries, Repository
  <entity>.go                 Domain struct + Create/Update + Validate()
  adapters/<domain>http/      Inbound HTTP handlers
  adapters/<domain>pg/        Outbound PostgreSQL repo
  <domain>svc/service.go      Business logic (implements Commands + Queries)
  <domain>module/module.go    Assembler (wires concrete types, only importer of adapters)
```

### Naming: ID Parameters
```go
// Good — type carries "ID" semantics
Find(ctx context.Context, provider identity.ProviderID) (Provider, error)
// Avoid — redundant
Find(ctx context.Context, providerID identity.ProviderID) (Provider, error)
```

### Auth: IAMKit
- IAMKit runs as a sidecar Docker container, handles all IAM
- JWT validation via online introspection (`authclient.Introspect`)
- Permissions checked via `fiberauth.RequirePermissions`
- No billing/payment — open-source, free

## Key Decisions
<!-- AGENTS-GENERATED:START key-decisions -->
- **IAMKit for auth** — no home-grown IAM; all auth/users/orgs/roles delegated to IAMKit
- **No billing** — open-source and free, no Stripe, no wallets, no credit balances
- **Fiber v2** — HTTP framework
- **sqlx + raw SQL** — no ORM; migrations are plain .sql files
- **Single binary** — `cmd/server/main.go` is the only entrypoint
- **PostgreSQL 16 + Redis 7**
<!-- AGENTS-GENERATED:END key-decisions -->

## Boundaries

### Always Do
- Run pre-commit checks before committing
- Add tests for new code paths
- Use conventional commit format: `type(scope): subject`
- Use **atomic commits**
- **Show test output as evidence before claiming work is complete**
- Wrap all DB errors through `errx` — never return raw driver errors
- Use typed IDs from `identity` — never bare `string` for entity identifiers

### Ask First
- Adding new dependencies
- Modifying CI/CD configuration
- Changing public API signatures
- Repo-wide refactoring

### Never Do
- Commit secrets, credentials, or `.env` files
- Use `errors.New` or `fmt.Errorf` for application errors — use `errx`
- Import concrete adapter types outside `*module/` packages
- Put validation logic in handlers or repositories
- Create a second ID system parallel to `identity.ID[T]`
- Delete migration files

## Terminology
| Term | Means |
|------|-------|
| Provider | An LLM API provider (OpenAI, Anthropic, Google, etc.) |
| Model | An LLM model variant (gpt-4o, claude-sonnet, gemini-pro) |
| Mapping | A model-provider link with pricing and capability flags |
| Fallback | A fallback relationship between two models (priority-ordered) |
| ProviderKey | An encrypted upstream API credential for a provider (NaCl secretbox) |
| Gateway | The proxy layer that routes LLM requests to providers |
| Router | Resolves model name → provider + key + mapping (with fallback chain) |
| RouteResult | A resolved route: provider, key, mapping, token, base URL, pricing |
| Profile | Per-provider config: auth style, URL builder, translator |
| Translator | Converts between OpenAI format and provider-native APIs |
| KeyHealthTracker | In-memory sliding-window tracker for key reliability |
| Upstream | HTTP client that calls provider APIs (sync + streaming) |
| Strategy | Route ordering: cheapest, lowest-latency, round-robin |
| UsageLog | A record of a single gateway request: tokens, cost, timing, status |
| RateLimitConfig | Per-subject RPM + max-concurrent settings (stored in Postgres) |
| Limiter | Redis-backed enforcer: sliding-window RPM + concurrency counter |
| GuardrailConfig | Global content-safety config: enable + per-detector system rules (singleton) |
| GuardrailRule | Custom content rule: blocked terms or regex, with block/redact/warn action |
| GuardrailViolation | A logged record of a triggered guardrail rule (async, best-effort) |
| Evaluator | Gateway-facing check: `CheckMessages` runs before routing a request |
| Module | A self-contained domain package with ports/adapters/svc/module |
| Assembler | The `*module` package that wires concrete types |
| IAMKit | External identity service running as Docker sidecar |
| errx | The typed error system — the only way to create application errors |
| identity | The typed ID system — compile-time safe entity identifiers |
| Container | DI composition root in `internal/bootstrap/` (not Docker) |
| WebhookConfig | A subscription: URL + event types + HMAC secret |
| WebhookDelivery | A single delivery attempt record with retry/backoff state |
| Dispatcher | Gateway-facing hook: `Fire(event, data)` enqueues async delivery |
| APIKey | A FreeRouter-managed API key for gateway access, backed by IAMKit service accounts |
| APIKeyCredential | One-time response from API key creation containing the `ik_svc_...` secret |
| Store (apikey) | Adapter interface abstracting IAMKit's service-account management SDK |

## Scoped AGENTS.md (MUST read when working in these directories)
<!-- AGENTS-GENERATED:START scope-index -->
- `internal/AGENTS.md` — Internal packages: domain modules, foundation, infra adapters
<!-- AGENTS-GENERATED:END scope-index -->

> **Agents**: When you read or edit files in a listed directory, you **must** load its AGENTS.md first.

## When instructions conflict
The nearest `AGENTS.md` wins. Explicit user prompts override files.
