---
name: stratix-core-advisor
description: >
  Skill for assisting with the stratix-core Go project.
  Use this when the user is writing, reviewing, or debugging Go code
  in the stratix-core modular monolith REST API.
---

# Stratix-Core Advisor Skill

## Project Context

**stratix-core** is a modular monolith REST API built with Go, PostgreSQL, and Clean Architecture. It powers a task management mobile app (iOS/Android) targeting App Store publication.

---

## Deployment Architecture

```
Mobile app (iOS / Android)
        │
        ▼
Cloudflare (CDN + DDoS + TLS)
        │
        ▼
Koyeb  ──── Go binary (Docker container)
               │
               ├── Neon PostgreSQL
               ├── Cloudflare R2 (attachments)
               └── FCM / APNs (push notifications)
```

Key context when advising:
- **Neon** — use the **pooled** connection string from the Neon dashboard; `pgxpool` is tuned with `DB_MAX_CONNS=10` (free tier limit), `DB_MAX_CONN_LIFETIME=30m` to cycle before Neon drops idle connections
- **TLS** — terminated at Cloudflare; Go server runs plain HTTP inside Koyeb
- **Secrets** — managed via Koyeb env vars; all `os.Getenv` calls are centralized in `config/config.go` — never elsewhere
- **R2 Storage** — S3-compatible; `pkg/storage` has a `Client` interface + `FakeClient` stub (real R2 client pending credentials)
- **Push** — FCM for Android, APNs for iOS; `PushSender` interface with `StubPushSender` in place (real implementation pending)
- **AI** — Anthropic Claude Haiku via raw `net/http`; prompt lives in `internal/ai/service/prompts/parse.md` (embedded at compile time)

---

## Project Stack (always use these, don't suggest alternatives)

| Concern | Choice |
|---|---|
| Router | `github.com/labstack/echo/v4` |
| DB driver | `github.com/jackc/pgx/v5` (`pgxpool.Pool`) |
| Config | `config/config.go` — `os.Getenv` with typed helpers and defaults |
| Logging | `log/slog` (stdlib) |
| Auth | `github.com/golang-jwt/jwt/v5` (HS256) + Echo JWT middleware |
| IDs | `github.com/google/uuid` |
| Passwords | `golang.org/x/crypto/bcrypt` (cost 12) |
| AI | Anthropic Messages API via raw `net/http` — no SDK |
| Events | `internal/events.Dispatcher` — fire-and-forget goroutines |

---

## Module Layout

```
internal/
  domain/           → entities, repository interfaces, event types, enums
  events/           → Dispatcher (dispatcher.go)
  auth/             → handler/, repository/, service/
  tasks/            → handler/ (task_handler.go, dto.go), repository/, service/
  analytics/        → handler/, repository/, service/
  notifications/    → handler/, repository/, service/ (push.go)
  ai/               → handler/, service/ (parse_service.go, prompts/parse.md)
  middleware/       → jwt.go, ai_ratelimit.go
config/             → config.go
pkg/
  apperror/         → AppError + typed constructors (BadRequest, NotFound, Internal, etc.)
  pagination/       → cursor-based Page[T]
  storage/          → Client interface, FakeClient
db/migrations/      → 001–005 up/down SQL files
cmd/server/         → main.go, bootstrap.go
```

---

## Patterns to Reinforce

### Error Handling

Three-layer flow:

```go
// Repository — domain error:
return nil, &domain.NotFoundError{Resource: "task", ID: id.String()}

// Service — translate to HTTP error:
if domain.IsNotFound(err) {
    return nil, apperror.NotFound("task")
}
return nil, apperror.Internal(err)

// Handler — return apperror directly (bootstrap.httpErrorHandler maps it to JSON):
return apperror.BadRequest("title is required")
```

Never use `echo.NewHTTPError` — always use `apperror.*`.

### Dependency Injection

No DI framework. Wired manually in `bootstrap.go`. Services accept interfaces:

```go
func NewTaskService(repo domain.TaskRepository, dispatcher *events.Dispatcher, storage storage.Client) *TaskService
```

### Cross-Module Events

Modules never import each other. Use `internal/events.Dispatcher`:

```go
// 1. Domain aggregate appends event:
t.addEvent(domain.TaskCompletedEvent{TaskID: t.ID, UserID: t.UserID, CompletedAt: now})

// 2. Service dispatches after save:
for _, evt := range task.DomainEvents() {
    s.dispatcher.Dispatch(ctx, evt)
}

// 3. Handlers registered in bootstrap.go only:
dispatcher.Register("task.completed", analyticsService.OnTaskCompleted)
```

Registered events: `task.created`, `task.completed`, `task.archived`, `task.rescheduled`, `task.overdue`, `recurrence_instance.completed`.

### Repository Pattern

- `*_postgres.go` files implement `domain.*Repository` interfaces
- Raw `pgx` queries — no ORM, no sqlc
- Scan rows manually using a shared `scanner` interface (`scan.go` in the repository package)
- Return `&domain.NotFoundError{}` on `pgx.ErrNoRows`

### Config

```go
// Only place os.Getenv is called — config/config.go:
AnthropicModel: envStr("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"),

// bootstrap.go passes config fields into constructors:
aisvc.NewParseService(aisvc.ParseServiceConfig{Model: cfg.AnthropicModel, ...})
```

---

## Auth Flow

```
POST /v1/auth/register → bcrypt (cost 12) → insert user → return access + refresh token
POST /v1/auth/login    → fetch user → compare hash → return access + refresh token
POST /v1/auth/refresh  → validate refresh token → return new access token
POST /v1/auth/logout   → revoke refresh token
```

JWT `sub` = `user_id` (UUID string). Extracted in handlers via:
```go
userID, ok := c.Get(middleware.UserIDKey).(string)
```

---

## Task Status Transitions

```
backlog → scheduled | in_progress
scheduled → in_progress | completed | cancelled
in_progress → completed | cancelled
completed → backlog  (reopen)
cancelled → (terminal)
```

`due_passed` is computed by `task.EffectiveStatus()` — never stored in DB.

---

## Review Checklist

- [ ] `ctx context.Context` is first param in all repo and service methods
- [ ] Errors wrapped with `fmt.Errorf("...: %w", err)`, never discarded
- [ ] `apperror.*` used in handlers — not `echo.NewHTTPError`
- [ ] UUID used as PK, never int
- [ ] No direct module-to-module imports — use Dispatcher
- [ ] Service constructors accept interfaces, not concrete types
- [ ] SQL uses `$1, $2` parameterized inputs — never string concat
- [ ] Passwords never logged or returned in responses
- [ ] `os.Getenv` only in `config/config.go`

---

## Tone & Style

- Be direct and concise — this is an active build session, not a tutorial
- Give targeted suggestions; don't rewrite large blocks unprompted
- When showing code, keep snippets minimal and focused on the change
