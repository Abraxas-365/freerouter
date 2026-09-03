<h1 align="center">
    🛰️ FreeRouter
</h1>
<p align="center">
    <p align="center">Self-Hosted LLM Gateway</p>
    <p align="center">Open source, multi-tenant AI gateway for 8+ LLM providers. One OpenAI-compatible API with built-in billing, wallets, guardrails, and observability — all in a single Go binary.</p>
</p>
<h4 align="center">
    <a href="https://github.com/Abraxas-365/freerouter" target="_blank">
        <img src="https://img.shields.io/github/stars/Abraxas-365/freerouter?style=social" alt="GitHub Stars">
    </a>
    <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go 1.25">
    <img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT License">
    <img src="https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white" alt="PostgreSQL 16">
    <img src="https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white" alt="Redis 7">
</h4>

---

## What is FreeRouter

FreeRouter is a self-hosted LLM gateway that sits between your applications and AI providers. Send requests in **OpenAI**, **Anthropic**, or **Responses API** format — FreeRouter translates and routes them to the cheapest healthy provider key across OpenAI, Anthropic, Google AI Studio, Mistral, DeepSeek, xAI, Groq, and Together AI.

It's built for teams that resell or meter LLM access: every request is authenticated, guardrail-checked, rate-limited, cost-tracked, and debited from a tenant balance — pay-per-token, no subscriptions.

```
Client ──► Auth ──► Guardrails ──► Rate Limiter ──► Router ──► Provider
                                                       │
              Billing debit ◄── Usage log ◄── Webhooks ┘
```

---

## Why FreeRouter

- **One API, many providers** — OpenAI-compatible `/v1/*` endpoints; swap models across 8 providers without changing client code
- **Cheapest-healthy routing** — picks the lowest-cost healthy provider key per request, with retry, provider fallback, and model fallback chains
- **Multi-tenant by design** — tenants, users, roles, scoped permissions, invitations, and API keys with per-key model restrictions
- **Real billing, not a stub** — credit balances, Stripe checkout for top-ups, spending limits, per-request token-cost debit, and **wallets** (named sub-balances bound to API keys for hard budget isolation per customer/team/environment)
- **Guardrails before the provider** — PII detection, secret detection, and custom regex rules with block or redact actions
- **Full observability** — Prometheus metrics, request/response content logging with debug mode, usage analytics, and webhook event delivery
- **Single Go binary + React dashboard** — no sidecar services beyond PostgreSQL and Redis

---

## Features

| Category | What's included |
|---|---|
| **Gateway** | Chat completions, Anthropic Messages, Responses API, embeddings, image generation, streaming (SSE), cost estimation |
| **Routing** | Cheapest-healthy-key selection, retry + provider fallback, model fallback chains, per-tenant routing config |
| **Auth** | OAuth (Google, Microsoft), passwordless email OTP, JWT with refresh, API keys with scopes + model restrictions |
| **Billing** | Credit balance, Stripe checkout top-ups, manual adjustments, per-request debit, daily/monthly spending limits |
| **Wallets** | Named sub-balances per tenant; fund/withdraw atomically from main balance; bind API keys to a wallet for hard budget caps |
| **Rate limiting** | Per-tenant RPM + concurrency limits (Redis-backed), configurable via API |
| **Guardrails** | PII detection, secret detection, custom regex rules; block or redact; violation logs |
| **Caching** | Response cache with per-tenant invalidation |
| **Logging** | Request/response content logging, debug mode for raw payloads, configurable data retention |
| **Webhooks** | Event subscriptions with delivery tracking and retry |
| **Metrics** | Prometheus endpoint: latency, tokens, errors, retries, rate-limit hits, cache hit/miss, in-flight requests |
| **Dashboard** | React web UI: dashboard, models, providers, provider keys, API keys, billing, wallets, usage, guardrails, webhooks, gateway config |

---

## Supported Providers

FreeRouter seeds **8 providers and 58 models** out of the box. Add any additional OpenAI-compatible provider through the API.

| Provider | Example models |
|---|---|
| **OpenAI** | GPT-5.x, GPT-4o, GPT-4.1, o1/o3/o4-mini, DALL·E, text-embedding-3 |
| **Anthropic** | Claude Sonnet 4, Claude 3.5 Haiku |
| **Google AI Studio** | Gemini 2.5 Pro / Flash |
| **Mistral** | Large, Small, Codestral |
| **DeepSeek** | Chat, Reasoner |
| **xAI** | Grok 3, Grok 3 Mini |
| **Groq** | Llama 3, Gemma 2, Mixtral |
| **Together AI** | Llama 3, Qwen, DeepSeek R1 |

---

## Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose

### Run it

```bash
git clone https://github.com/Abraxas-365/freerouter.git
cd freerouter

make setup   # start PostgreSQL + Redis, run migrations, seed providers/models
make dev     # start the server at http://localhost:8080
```

```bash
curl http://localhost:8080/health
```

### Dashboard (optional)

```bash
cd web
npm install
npm run dev   # http://localhost:5173
```

### First request

1. Sign up (passwordless OTP or OAuth) — this creates your tenant
2. Add a provider key (your own OpenAI/Anthropic/... API key) via the API or dashboard
3. Create a FreeRouter API key with the gateway scope
4. Call it like OpenAI:

```bash
# OpenAI-compatible chat (streaming supported)
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <your_freerouter_api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

```bash
# Anthropic Messages format — same gateway
curl http://localhost:8080/v1/messages \
  -H "Authorization: Bearer <your_freerouter_api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

```bash
# Estimate cost before sending
curl http://localhost:8080/v1/cost/estimate \
  -H "Authorization: Bearer <your_freerouter_api_key>" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello!"}]}'
```

---

## API Overview

### Gateway (OpenAI-compatible, `/v1/*`)

| Method | Path | Description |
|---|---|---|
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/chat/completions` | Chat completions (SSE streaming supported) |
| `POST` | `/v1/messages` | Anthropic Messages API |
| `POST` | `/v1/responses` | OpenAI Responses API |
| `POST` | `/v1/embeddings` | Text embeddings |
| `POST` | `/v1/images/generations` | Image generation |
| `POST` | `/v1/cost/estimate` | Estimate cost before sending |

### Management (`/api/v1/*`, JWT or API key + scopes)

| Area | What you can do |
|---|---|
| **Providers** | CRUD providers, models, model↔provider mappings, fallback chains |
| **Provider keys** | Manage encrypted upstream API keys (BYOK) |
| **Billing** | Balance, Stripe checkout, top-up, adjustments, transactions, spending limits |
| **Wallets** | Create/list/update/delete wallets, fund/withdraw from main balance |
| **Usage** | Query request logs, usage summaries, data retention config |
| **Guardrails** | Content filtering rules and violation logs |
| **Rate limits** | Per-tenant RPM and concurrency config |
| **Webhooks** | Event subscriptions and delivery history |
| **IAM** | API keys, users, roles, invitations |

### Auth

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/login` | OAuth login (Google, Microsoft) |
| `GET` | `/auth/callback/:provider` | OAuth callback |
| `POST` | `/auth/passwordless/signup/initiate` · `/verify` | Passwordless signup with email OTP |
| `POST` | `/auth/passwordless/login/initiate` · `/verify` | Passwordless login |
| `POST` | `/auth/refresh` | Refresh JWT |
| `POST` | `/auth/logout` | Logout |
| `GET` | `/auth/me` | Current user |
| `POST` | `/webhooks/stripe` | Stripe webhook (signature-verified) |

---

## Configuration

Everything is configured via environment variables. Key settings:

| Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | HTTP server port |
| `ENVIRONMENT` | `development` | `development` or `production` |
| `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USER` / `DB_PASSWORD` | `localhost` / `5432` / … | PostgreSQL connection |
| `REDIS_HOST` / `REDIS_PORT` | `localhost` / `6379` | Redis connection |
| `JWT_SECRET_KEY` | — | JWT signing key |
| `AI_ENCRYPTION_KEY` | — | Key for encrypting stored provider API keys |
| `STRIPE_SECRET_KEY` / `STRIPE_WEBHOOK_SECRET` | — | Enable Stripe credit purchases |
| `STRIPE_MIN_TOPUP_USD` / `STRIPE_MAX_TOPUP_USD` | `5` / `10000` | Top-up bounds |
| `FORCE_DEBUG_MODE` | `false` | Log raw request/response payloads globally |
| `CORS_ORIGINS` | — | Comma-separated allowed origins |

See `internal/config/` for the full list.

### Internal / self-hosted deployments (no Stripe)

Stripe is optional. If `STRIPE_SECRET_KEY` is unset:

- The `/webhooks/stripe` route is not registered and checkout requests return a clean error
- The dashboard hides the "Buy credits" flow (`GET /api/v1/billing/config` reports `stripe_enabled: false`)
- Admins grant credits directly with `POST /api/v1/billing/top-up` and `POST /api/v1/billing/adjust` (`billing:admin` scope)
- Everything else keeps working: spending limits, wallets (per-team/env budgets), usage tracking, and metrics — useful as internal budget and chargeback tools

No separate build or branch needed — it's the same binary, just without the Stripe env vars.

---

## Metrics

Prometheus endpoint at `GET /metrics` (namespace `freerouter_gateway_*`):

| Metric | Labels |
|---|---|
| `requests_total` | model, provider, protocol, status |
| `request_duration_seconds` | model, provider, protocol |
| `tokens_total` | model, provider, type (prompt/completion) |
| `errors_total` | provider, status_code |
| `retries_total` | provider, reason |
| `rate_limit_total` | type (rpm/concurrency) |
| `cache_hits_total` / `cache_misses_total` | — |
| `in_flight_requests` | protocol |

---

## Architecture

```
cmd/                          # Entry point, DI container, route registration
internal/
  ai/
    gateway/                  # Core proxy: routing, upstream calls, caching, rate limiting, metrics
    provider/                 # Providers, models, mappings, fallback chains
    providerkey/              # Encrypted upstream API key management
    guardrails/               # Content filtering rules engine
    usage/                    # Usage logging, data retention
  billing/                    # Credit balance, Stripe, transactions, spending limits
  wallet/                     # Tenant sub-balance wallets with API key binding
  iam/                        # OAuth, passwordless OTP, JWT, API keys, users, roles, tenants, invitations, scopes
  webhook/                    # Event subscriptions and delivery
  kernel/                     # Typed IDs, AuthContext, shared types
  config/                     # Env-based configuration
  errx/                       # Typed error system with per-module registries
web/                          # React 19 + Vite + Tailwind dashboard
migrations/                   # PostgreSQL schema (16 migrations, seeded providers/models)
tests/e2e/                    # End-to-end tests (testcontainers)
```

Design principles: hexagonal architecture (domain / service / infra / api per module), raw SQL via `sqlx` (no ORM), typed IDs everywhere, all queries tenant-scoped, AWS-IAM-style scope permissions.

---

## Development

```bash
make help           # All available commands
make dev            # Run development server
make dev-watch      # Hot reload (requires air)
make test           # Run tests
make lint           # golangci-lint
make up / make down # Start/stop PostgreSQL + Redis
make migrate        # Run migrations
make seed           # Seed test data
make psql           # PostgreSQL shell
```

### E2E tests

E2E tests spin up isolated PostgreSQL and Redis via [testcontainers](https://testcontainers.com/) — 30 test functions with 150+ subtests covering scope enforcement, CRUD, billing, and the full gateway pipeline:

```bash
go test ./tests/e2e/ -v -timeout 600s
```

---

## Contributing

Issues and PRs welcome. Before submitting:

```bash
go vet ./...
golangci-lint run
go test ./...
```

Use conventional commits (`type(scope): subject`) and keep commits atomic.

---

## License

MIT
