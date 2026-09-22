<!-- FOR AI AGENTS — scoped to internal/ packages -->
<!-- Last updated: 2026-09-19 -->

# internal/ — AGENTS.md

## Package Roles

| Package | Layer | May import | Never imports |
|---------|-------|-----------|---------------|
| `errx` | Foundation | stdlib only | anything in `internal/` |
| `identity` | Foundation | `errx`, stdlib, uuid | domain packages |
| `query` | Foundation | stdlib only | anything in `internal/` |
| `httpx` | Infrastructure | `query`, Fiber | domain packages |
| `config` | Foundation | stdlib only | anything in `internal/` |
| `server` | Infrastructure | `errx`, `config`, IAMKit SDK, Fiber | domain packages |
| `bootstrap` | Composition root | everything | — |
| `testutil` | Test-only | `sqlx`, `redis`, testcontainers-go, stdlib `testing` | domain packages (test-only, not part of the dependency graph) |
| `<domain>/ports.go` | Domain | `identity`, `query` | adapters, svc, other domains |
| `<domain>/<entity>.go` | Domain | `errx`, `identity` | adapters, svc |
| `<domain>/<domain>svc` | Application | domain ports, `identity`, `query` | adapters, other domain internals |
| `<domain>/adapters/*` | Infrastructure | domain package, `errx`, `identity`, `query`, `httpx`, drivers | other adapters |
| `<domain>/<domain>module` | Assembler | domain, adapters, svc | other modules |

## Adding a New Module

1. Add typed ID(s) in `internal/identity/entities.go`
2. Create `internal/<domain>/ports.go` — Commands, Queries, Repository interfaces
3. Create `internal/<domain>/<entity>.go` — struct + Create/Update + Validate()
4. Create `internal/<domain>/adapters/<domain>pg/repository.go` — sqlx impl
5. Create `internal/<domain>/adapters/<domain>http/handler.go` — Fiber handlers
6. Create `internal/<domain>/<domain>svc/service.go` — business logic
7. Create `internal/<domain>/<domain>module/module.go` — assembler
8. Wire in `internal/bootstrap/container.go`
9. Add migration in `migrations/`

Mirror `internal/provider/` exactly for naming and structure.

## Error Handling Rules

- Repository: wrap raw errors → `errx.Wrap(err, msg, errx.TypeInternal)`
- Repository: detect `pq.Error` code `23505` → `errx.Conflict(...)`
- Service: validate input → `input.Validate()` returns `*errx.Error`
- Service: propagate repo errors as-is (already typed)
- Handler: return errors — global middleware converts to HTTP response
- Never: `errors.New(...)`, `fmt.Errorf(...)` for application errors
