<!-- Managed by agent: keep sections and order; edit content, not structure. Last updated: 2026-08-27 -->

# AGENTS.md -- internal/

<!-- AGENTS-GENERATED:START overview -->
## Overview
Go backend packages for freerouter. Hexagonal architecture: domain entities in module root, ports as interfaces, services in `*srv/`, infrastructure in `*infra/`, HTTP handlers in `*api/`. All code uses `errx` typed errors and `kernel` shared types.
<!-- AGENTS-GENERATED:END overview -->

<!-- AGENTS-GENERATED:START filemap -->
## Key Files
```
config/             -> Config structs, one file per concern (server, database, redis, auth, oauth, jobx, notifx)
errx/               -> Error system: Error struct, Type enum, Registry for per-module codes, HTTP mapping
  error.go          -> Error type, New(), Wrap(), Wrapf()
  regestry.go       -> Registry type, Register(), New(), NewWithMessage()
  common.go         -> Shortcuts: errx.Validation(), errx.NotFound(), etc.
  types.go          -> Error type enum (TypeValidation, TypeNotFound, etc.)
  http.go           -> HTTP status mapping
kernel/             -> Shared value objects and utilities
  bind.go           -> BindAndValidate[T] generic request parser with compile-time Validate() check
  common_ids.go     -> UserID, TenantID typed string IDs
  context.go        -> AuthContext, scope matching (MatchScope, ScopesContain)
  store.go          -> Paginated[T], PaginationOptions
  proj_ids.go       -> Project-specific IDs
  proj_objvalue.go  -> Project-specific value objects
logx/               -> Structured logger (levels, formatters, fields)
ptrx/               -> Pointer utility helpers
asyncx/             -> Async execution utilities
fsx/                -> File system port + implementations
  fsx.go            -> FileSystem interface
  fsxlocal/         -> Local filesystem implementation
  fsxs3/            -> S3 implementation
jobx/               -> Background job queue
  jobx.go           -> Client (Handle, Enqueue, Start)
  models.go         -> Job, JobHandler types
  jobxredis/        -> Redis-backed queue implementation
notifx/             -> Email/notification system
  notifx.go         -> Client
  models.go         -> Email, Template types
  notifxses/        -> AWS SES provider
  notifxconsole/    -> Console provider (dev)
iam/                -> IAM bounded context
  iam.go            -> Shared IAM errors (ErrUnauthorized, ErrAccessDenied)
  doc.go            -> Package docs
  auth/             -> OAuth, passwordless, JWT, middleware, sessions
  user/             -> User entity, service, repo, handlers
  tenant/           -> Tenant entity, service, repo, handlers
  role/             -> Role entity (scope collections), CRUD + user assignment
  invitation/       -> Invitation system with scope/role assignment
  apikey/           -> API key auth with scoped permissions
  otp/              -> One-time password generation/verification
  scopes/           -> Scope definitions and validation
  iamcontainer/     -> IAM DI container (wires all IAM dependencies)
```
<!-- AGENTS-GENERATED:END filemap -->

<!-- AGENTS-GENERATED:START golden-samples -->
## Golden Samples (follow these patterns)

### Entity file pattern (`role/role.go`)
```
1. Domain struct with `db` + `json` tags
2. Domain methods (HasScope, SetScopes)
3. DTO struct + ToDTO() method
4. Request structs with Validate() error (required for BindAndValidate)
5. Response structs
6. errx.NewRegistry("MODULE") + error code vars + helper funcs
```

### Handler pattern (`role/roleapi/handler.go`)
```go
func (h *RoleHandlers) CreateRole(c *fiber.Ctx) error {
    authContext, ok := auth.GetAuthContext(c)
    if !ok {
        return iam.ErrUnauthorized()
    }
    req, err := kernel.BindAndValidate[role.CreateRoleRequest](c)
    if err != nil {
        return err
    }
    r, err := h.service.CreateRole(c.Context(), authContext.TenantID, req)
    if err != nil {
        return err
    }
    return c.Status(fiber.StatusCreated).JSON(r.ToDTO())
}
```

### Infra pattern (`role/roleinfra/postgres.go`)
```
1. Persistence struct with sql.Null* types for nullable columns
2. toPersistence(domain) and toDomain(persistence) mappers
3. sqlx queries with $1/$2 params
4. pq.Error code "23505" for unique constraint violations
5. errx.Wrap for all infra errors
```
<!-- AGENTS-GENERATED:END golden-samples -->

<!-- AGENTS-GENERATED:START setup -->
## Setup & environment
- Go 1.25.4
- `make init` for full setup (tidy + docker + migrate + seed)
- Env vars configured in `Makefile` for development
<!-- AGENTS-GENERATED:END setup -->

<!-- AGENTS-GENERATED:START commands -->
## Build & tests
- `go vet ./...`
- `go fmt ./...`
- `golangci-lint run`
- `go test -v ./...`
- `go test -race -v ./...`
- `go test -v ./internal/path/to/pkg/...` (single package)
- `go build -o bin/server ./cmd`
<!-- AGENTS-GENERATED:END commands -->

<!-- AGENTS-GENERATED:START code-style -->
## Code style & conventions
- Follow Go 1.25 idioms; use `any` over `interface{}`
- Errors: use `errx` system, not `fmt.Errorf` (except inside `errx.Wrap`)
- Errors in registries: `var ErrRegistry = errx.NewRegistry("PREFIX")` at bottom of entity file
- Request validation: implement `Validate() error` on pointer receiver for `BindAndValidate[T]` to compile
- Naming: `camelCase` private, `PascalCase` exported; ID/URL/HTTP not Id/Url/Http
- All handler funcs: `func (h *Handlers) Method(c *fiber.Ctx) error`
- Always pass `context.Context` to service/repo methods
- SQL: use `sqlx` with `$1` positional params, never string concatenation
- Infra repos: separate persistence model from domain model
<!-- AGENTS-GENERATED:END code-style -->

<!-- AGENTS-GENERATED:START security -->
## Security & safety
- All queries must filter by `TenantID` for tenant isolation
- Use `auth.GetAuthContext(c)` to get authenticated user; never trust raw params for auth
- Scope middleware (`RequireScope`) on all protected routes
- SQL: parameterized queries only (`$1`, `$2`)
- Sensitive data: never log tokens, passwords, API keys
- API keys: stored as bcrypt hashes, only prefix exposed
<!-- AGENTS-GENERATED:END security -->

## House Rules (project-specific)
- Preserve all `manifesto:` comments -- they are code-gen markers
- Each IAM sub-module follows the `entity/port/*srv/*infra/*api` structure
- Container wiring order: infra -> repos -> services -> handlers -> middleware
- New modules: create container struct, wire in `cmd/container.go`, register routes in `cmd/server.go`
