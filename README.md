# FreeRouter

A self-hosted, multi-tenant LLM gateway that sits between your applications and AI providers. Route requests across OpenAI, Anthropic, Google, Mistral, DeepSeek, xAI, Groq, and Together AI through a single API -- with built-in billing, auth, rate limiting, guardrails, and usage tracking.

## Why FreeRouter

- **One API, many providers** -- Send requests in OpenAI, Anthropic, or Responses API format. FreeRouter translates and routes to the cheapest healthy provider key.
- **Multi-tenant from day one** -- Tenants, users, roles, scopes, API keys with model restrictions. Everything is tenant-scoped.
- **Pay-per-token billing** -- Automatic cost tracking, balance management, spending limits, and per-request debit. No external billing dependency required.
- **Resilient routing** -- Health checks, automatic retry with fallback across providers, model fallback chains, response caching.
- **Content safety** -- Guardrail rules (PII detection, secrets, regex filters) with block/redact actions before requests hit providers.
- **Full observability** -- Prometheus metrics, request/response content logging with debug mode, webhook notifications, usage analytics.

## Features

| Category | What's included |
|---|---|
| **Gateway** | Chat completions, Anthropic messages, Responses API, embeddings, image generation |
| **Routing** | Cheapest-healthy-key selection, retry + fallback, model fallback chains, streaming retry |
| **Auth** | OAuth (Google, Microsoft), passwordless OTP, JWT, API keys with scopes and model restrictions |
| **Billing** | Credit balance, top-up, adjustments, per-request token-cost debit, daily/monthly spending limits |
| **Rate Limiting** | Per-tenant RPM + concurrency limits, configurable per tenant |
| **Guardrails** | PII detection, secret detection, custom regex rules, block or redact actions |
| **Caching** | Response cache with per-tenant invalidation |
| **Logging** | Full request/response content logging, debug mode for raw payloads, configurable data retention |
| **Webhooks** | Event subscriptions (request.completed, request.failed, etc.), delivery tracking with retry |
| **Metrics** | Prometheus endpoint with request latency, token counts, error rates, cache hit/miss ratios |
| **IAM** | Tenants, users, roles, invitations, scoped permissions, API key management |

## Supported Providers

FreeRouter ships with 8 pre-configured providers and 57+ models:

- **OpenAI** -- GPT-4o, GPT-4.1, o1, o3, o4-mini, DALL-E, text-embedding-3
- **Anthropic** -- Claude Sonnet 4, Claude 3.5 Haiku
- **Google AI Studio** -- Gemini 2.5 Pro/Flash
- **Mistral** -- Large, Small, Codestral
- **DeepSeek** -- Chat, Reasoner
- **xAI** -- Grok 3, Grok 3 Mini
- **Groq** -- Llama 3, Gemma 2, Mixtral (ultra-fast inference)
- **Together AI** -- Llama 3, Qwen, DeepSeek R1

Add any OpenAI-compatible provider by registering it through the API.

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- (Optional) `air` for hot reload

### Setup

```bash
# Clone and enter the project
git clone https://github.com/Abraxas-365/freerouter.git
cd freerouter

# Start PostgreSQL and Redis
make up

# Run migrations and seed providers/models
make dev
```

The server starts at `http://localhost:8080`. Check health:

```bash
curl http://localhost:8080/health
```

### First Request

1. Create a tenant and user through the auth flow or API
2. Add a provider key (your OpenAI/Anthropic API key)
3. Create an API key with `gateway:chat` scope
4. Send requests:

```bash
# OpenAI-compatible chat
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer fr_live_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Anthropic-compatible messages
curl http://localhost:8080/v1/messages \
  -H "Authorization: Bearer fr_live_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Embeddings
curl http://localhost:8080/v1/embeddings \
  -H "Authorization: Bearer fr_live_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-3-small",
    "input": "The food was delicious"
  }'

# Image generation
curl http://localhost:8080/v1/images/generations \
  -H "Authorization: Bearer fr_live_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dall-e-3",
    "prompt": "A sunset over mountains",
    "size": "1024x1024"
  }'
```

## Configuration

FreeRouter is configured via environment variables. Key settings:

| Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | HTTP server port |
| `ENVIRONMENT` | `development` | `development` or `production` |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `freerouterdb` | Database name |
| `DB_USER` | `freerouter` | Database user |
| `DB_PASSWORD` | | Database password |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `JWT_SECRET_KEY` | | JWT signing key (min 32 bytes) |
| `AI_ENCRYPTION_KEY` | | 32-byte hex key for provider key encryption |
| `FORCE_DEBUG_MODE` | `false` | Log raw request/response payloads globally |
| `CORS_ORIGINS` | | Comma-separated allowed origins |

See `internal/config/config.go` for all options.

## API Overview

### Gateway (OpenAI-compatible)

| Method | Path | Description |
|---|---|---|
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/chat/completions` | Chat completion (streaming supported) |
| `POST` | `/v1/messages` | Anthropic Messages API |
| `POST` | `/v1/responses` | OpenAI Responses API |
| `POST` | `/v1/embeddings` | Text embeddings |
| `POST` | `/v1/images/generations` | Image generation |
| `POST` | `/v1/cost/estimate` | Estimate cost before sending |

### Management (`/api/v1/*`)

| Area | Endpoints |
|---|---|
| **Providers** | CRUD for LLM providers, models, and model-provider mappings |
| **Provider Keys** | Manage encrypted API keys for providers (BYOK + managed) |
| **Billing** | Balance, top-up, transactions, spending limits |
| **Usage** | Query logs, usage summaries, data retention config |
| **Guardrails** | Content filtering rules and violation logs |
| **Rate Limits** | Per-tenant RPM and concurrency config |
| **Cache** | Invalidate response cache per tenant or globally |
| **Webhooks** | Event subscriptions and delivery history |
| **API Keys** | Create keys with scopes and model restrictions |
| **Users** | User management within tenants |
| **Roles** | Role-based access control with scopes |
| **Invitations** | Invite users to tenants |
| **Tenants** | Multi-tenant organization management |

### Auth

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/login` | OAuth login |
| `GET` | `/auth/callback/:provider` | OAuth callback |
| `POST` | `/auth/passwordless/login/initiate` | Passwordless OTP login |
| `POST` | `/auth/passwordless/login/verify` | Verify OTP |
| `POST` | `/auth/refresh` | Refresh JWT |

## Architecture

```
Client Request
    |
    v
[Fiber HTTP Server]
    |
    v
[Auth Middleware] -- JWT or API Key validation, scope + model checks
    |
    v
[Guardrails] -- PII/secret detection, regex rules, block/redact
    |
    v
[Rate Limiter] -- RPM + concurrency per tenant (Redis-backed)
    |
    v
[Router] -- Resolve model -> find cheapest healthy provider key
    |
    v
[Upstream] -- Translate request, call provider, retry on failure
    |
    v
[Post-processing] -- Debit billing, log usage, fire webhooks, cache response
```

### Project Structure

```
cmd/                          # Entry point, DI container, server setup
internal/
  ai/
    gateway/                  # Core LLM proxy: routing, upstream, caching, rate limiting
      gatewayapi/             # HTTP handlers for all gateway endpoints
    provider/                 # Provider, model, mapping, fallback entities + CRUD
    providerkey/              # Encrypted provider API key management
    guardrails/               # Content filtering rules engine
    usage/                    # Usage logging, data retention
  billing/                    # Credit balance, transactions, spending limits
  iam/
    auth/                     # OAuth, passwordless, JWT, API key middleware
    apikey/                   # API key entity, service, per-key model restrictions
    user/                     # User management
    role/                     # Roles and scopes
    tenant/                   # Multi-tenant management
    invitation/               # Invitation flow
    scopes/                   # Scope constants (no hardcoded strings)
  webhook/                    # Event subscriptions and delivery
  kernel/                     # Typed IDs, AuthContext, shared types
  config/                     # Environment-based configuration
  errx/                       # Typed error handling
migrations/                   # PostgreSQL schema (13 migrations)
tests/e2e/                    # End-to-end tests with testcontainers
```

## Development

```bash
make help           # Show all available commands
make dev            # Run development server
make dev-watch      # Run with hot reload (requires air)
make test           # Run all tests
make up             # Start PostgreSQL + Redis
make down           # Stop services
make psql           # Open PostgreSQL shell
make lint           # Run golangci-lint
```

### Running E2E Tests

E2E tests use [testcontainers](https://testcontainers.com/) to spin up isolated PostgreSQL and Redis instances:

```bash
go test ./tests/e2e/ -v -timeout 600s
```

Currently covers 100+ test cases across all features including scope enforcement, CRUD operations, billing, gateway pipeline, and per-key model restrictions.

## Metrics

FreeRouter exposes a Prometheus endpoint at `GET /metrics` with:

- `freerouter_request_duration_seconds` -- Request latency by model, provider, and protocol
- `freerouter_tokens_total` -- Token counts (prompt + completion) by model
- `freerouter_errors_total` -- Error counts by provider and status code
- `freerouter_rate_limit_hits_total` -- Rate limit rejections
- `freerouter_cache_hits_total` / `freerouter_cache_misses_total` -- Cache performance
- `freerouter_retries_total` -- Retry attempts by provider and reason

## License

MIT
