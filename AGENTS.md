<!-- FOR AI AGENTS - Human readability is a side effect, not a goal -->
<!-- Managed by agent: keep sections and order; edit content, not structure -->
<!-- Last updated: 2026-08-27 | Last verified: 2026-08-27 -->

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
| Test (single) | `go test -v ./internal/path/to/pkg/...` | ~2s |
| Test (all) | `go test -v ./...` | ~30s |
| Test (race) | `go test -race -v ./...` | ~45s |
| Build | `go build -o bin/server ./cmd` | ~5s |
| Dev server | `make dev` | ongoing |
| Migrate | `make migrate` | ~2s |
| Services up | `make up` (docker compose) | ~5s |
| Full setup | `make init` (tidy + services + migrate + seed) | ~15s |
<!-- AGENTS-GENERATED:END commands -->

## Response Style
- Answer first, elaborate only if needed. No sycophantic openers.
- For yes/no or status questions, lead with the answer.
- Skip preamble. Match response length to task complexity.

## Workflow
1. **Before coding**: Read nearest `AGENTS.md` + check Golden Samples for the area you're touching
2. **After each change**: Run the smallest relevant check (vet -> lint -> single test)
3. **Before committing**: Run full test suite if changes affect >2 files or touch shared code
4. **Before claiming done**: Run verification and **show output as evidence**

## File Map
<!-- AGENTS-GENERATED:START filemap -->
```
cmd/                -> Entrypoint: Fiber server bootstrap, DI container, route registration
internal/
  config/           -> Config structs loaded from env vars (server, db, redis, auth, oauth, jobx, notifx)
  errx/             -> Typed error system: Error struct, Registry per module, HTTP status mapping
  kernel/           -> Shared types: IDs (UserID, TenantID), AuthContext, BindAndValidate[T], Paginated[T]
  logx/             -> Structured logger (levels, JSON/console formatters)
  ptrx/             -> Pointer helpers
  asyncx/           -> Async utilities
  fsx/              -> File system abstraction (fsxlocal/, fsxs3/)
  jobx/             -> Background job queue (jobxredis/ backend)
  notifx/           -> Notification/email system (notifxses/, notifxconsole/)
  iam/              -> IAM bounded context (auth, user, tenant, role, invitation, apikey, otp, scopes)
migrations/         -> SQL migrations (001_genesis.up.sql, 002_invitation_role_id.up.sql)
docker-compose.yml  -> Postgres 16 + Redis 7
manifesto.yaml      -> manifesto CLI project config (code-gen markers)
Makefile            -> Dev commands, Docker, DB operations
```
<!-- AGENTS-GENERATED:END filemap -->

## Golden Samples (follow these patterns)
<!-- AGENTS-GENERATED:START golden-samples -->
| For | Reference | Key patterns |
|-----|-----------|--------------|
| Domain entity + DTOs + errors | `internal/iam/role/role.go` | Struct with `db`/`json` tags, `ToDTO()`, `Validate()` on request types, `errx.NewRegistry` per module |
| Repository port | `internal/iam/role/port.go` | Interface in domain package, context-first args, domain types in/out |
| Service layer | `internal/iam/role/rolesrv/service.go` | Constructor injection, business validation, `errx.Wrap` on infra errors |
| Postgres repo | `internal/iam/role/roleinfra/postgres.go` | `sqlx`, `$1`-style params, persistence model with `toPersistence`/`toDomain`, `pq.Error` handling |
| HTTP handler | `internal/iam/role/roleapi/handler.go` | `auth.GetAuthContext(c)`, `kernel.BindAndValidate[T](c)`, `RegisterRoutes`, scope middleware |
| Module container | `internal/iam/iamcontainer/container.go` | Deps struct, wiring order: infra -> repos -> services -> handlers |
| App container | `cmd/container.go` | Root DI, infra init, module composition, background service start |
| Config loading | `internal/config/config.go` | `Load()` from env vars, `getEnv`/`getEnvInt`/`getEnvDuration` helpers |
<!-- AGENTS-GENERATED:END golden-samples -->

## Utilities (check before creating new)
<!-- AGENTS-GENERATED:START utilities -->
| Need | Use | Location |
|------|-----|----------|
| Parse + validate request body | `kernel.BindAndValidate[T](c)` | `internal/kernel/bind.go` |
| Paginated response | `kernel.Paginated[T]`, `kernel.NewPaginated(...)` | `internal/kernel/store.go` |
| Typed IDs | `kernel.UserID`, `kernel.TenantID`, `kernel.ProviderID`, `kernel.ModelID`, `kernel.MappingID` | `internal/kernel/common_ids.go` |
| Auth context from request | `auth.GetAuthContext(c)` | `internal/iam/auth/` |
| Scope checking | `kernel.ScopesContain(scopes, required)` | `internal/kernel/context.go` |
| Module error codes | `errx.NewRegistry("PREFIX")` then `.Register(...)` | `internal/errx/regestry.go` |
| Wrap errors | `errx.Wrap(err, msg, errx.TypeInternal)` | `internal/errx/error.go` |
| Validation error | `errx.Validation("msg")` | `internal/errx/common.go` |
| Pointer helpers | `ptrx` package | `internal/ptrx/ptrx.go` |
| Structured logging | `logx.Info/Warn/Error/Infof/...` | `internal/logx/` |
| Background jobs | `jobx.Client.Handle()` / `.Enqueue()` | `internal/jobx/jobx.go` |
| File storage | `fsx.FileSystem` interface | `internal/fsx/fsx.go` |
| Email notifications | `notifx.Client` | `internal/notifx/notifx.go` |
<!-- AGENTS-GENERATED:END utilities -->

## Heuristics (quick decisions)
<!-- AGENTS-GENERATED:START heuristics -->
| When | Do |
|------|-----|
| New domain entity | Create `entity.go` (struct + DTOs + errx registry), `port.go` (repo interface), `*srv/service.go`, `*infra/postgres.go`, `*api/handler.go`. Use typed IDs from `kernel/common_ids.go`, never raw `string` |
| New error code | Add to the module's `errx.NewRegistry`, not a shared file |
| Need pagination | Use `kernel.Paginated[T]` and `kernel.PaginationOptions` |
| New API route | Add to handler's `RegisterRoutes`, use scope middleware |
| New background job | Register handler with `jobx.Client.Handle()`, enqueue with `Enqueue()` |
| New config value | Add to appropriate `internal/config/*.go`, load from env vars |
| DB query | Use `sqlx` with parameterized queries (`$1`, `$2`), wrap errors with `errx.Wrap` |
| Handler body | Use `auth.GetAuthContext(c)` then `kernel.BindAndValidate[T](c)`, return `c.Status(...).JSON(dto)` |
| New module | Create under `internal/`, add container in `iamcontainer`-style, wire in `cmd/container.go` |
| New migration | `make migrate-create name=description`, never edit existing migration files |
| `manifesto:` comments | Preserve them -- they are code-gen markers for the manifesto CLI |
| Adding dependency | Ask first - we minimize deps |
| Unsure about pattern | Check Golden Samples above |
<!-- AGENTS-GENERATED:END heuristics -->

## Key Decisions
<!-- AGENTS-GENERATED:START key-decisions -->
- **Pay-per-token platform**: This is an LLM router (like OpenRouter) where tenants buy and consume tokens. There are NO subscription plans, NO trials, NO trial expiration. Tenants pay per token usage. Never add subscription/trial logic.
- **Multi-tenant**: All entities scoped by `TenantID`; queries must always filter by tenant
- **Scope-based auth**: AWS IAM-inspired; users have direct scopes + role scopes; middleware enforces via `RequireScope`
- **errx over stdlib errors**: All domain/service errors use `errx.Error` with registry codes; global error handler maps to HTTP responses
- **manifesto code-gen**: `manifesto.yaml` tracks installed modules; `manifesto:` comments are code-gen anchors -- never delete them
- **No ORM**: Raw SQL via `sqlx`; persistence models in `*infra/` with `toPersistence`/`toDomain` mappers
- **Container pattern**: Each bounded context has its own container (e.g., `iamcontainer.Container`); root `cmd/Container` composes them
<!-- AGENTS-GENERATED:END key-decisions -->

## Boundaries

### Always Do
- Run `go vet ./...` and `go test ./...` before committing
- Add tests for new code paths
- Use conventional commit format: `type(scope): subject`
- Use **atomic commits** (one logical change per commit)
- **Show test output as evidence before claiming work is complete**
- Before any edit, verify `pwd` resolves inside the intended repo worktree
- Errors: use `errx` types, not bare `fmt.Errorf` (except in infra wrapping)
- Always scope queries by `TenantID`
- Preserve `manifesto:` comments in generated code

### Ask First
- Adding new dependencies
- Modifying CI/CD configuration
- Changing public API signatures or auth middleware
- Repo-wide refactoring or rewrites
- Adding new manifesto modules
- Changing migration files

### Never Do
- Commit secrets, credentials, or sensitive data
- Use raw `string` for entity IDs -- always use typed IDs from `internal/kernel/common_ids.go` (e.g. `kernel.ProviderID`, `kernel.ModelID`); add new ID types there when needed
- Add subscription plans, trial periods, or SaaS billing tiers -- this is a pay-per-token platform
- Delete or edit `manifesto:` code-gen markers
- Edit existing migration files (create new ones instead)
- Push directly to main/master branch
- Return raw `error` from handlers (use `errx.Error`)
- Query without tenant scoping in multi-tenant tables
- Delete `manifesto.yaml` or its module entries

## Terminology
| Term | Means |
|------|-------|
| Scope | A permission string like `roles:read`, `users:write`; checked by middleware |
| Tenant | An organization/company; all data is tenant-scoped |
| errx Registry | Per-module error code registry (`errx.NewRegistry("PREFIX")`) |
| manifesto | CLI tool that scaffolds modules; `manifesto.yaml` tracks installed modules |
| Container | DI container pattern (not Docker); wires repos -> services -> handlers |
| Bounded context | A self-contained module (e.g., IAM) with its own container |

## Scoped AGENTS.md (MUST read when working in these directories)
<!-- AGENTS-GENERATED:START scope-index -->
- `internal/AGENTS.md` -- Internal packages: domain entities, services, infra, kernel utilities
<!-- AGENTS-GENERATED:END scope-index -->

> **Agents**: When you read or edit files in a listed directory, you **must** load its AGENTS.md first.

## When instructions conflict
The nearest `AGENTS.md` wins. Explicit user prompts override files.
