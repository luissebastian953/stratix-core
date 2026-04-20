# Stratix-Core — Coding Guide

> Personal reference while building the project. Not meant to be exhaustive — just the patterns and decisions we settled on.

---

## Infrastructure & Deployment

```
Mobile app (iOS / Android)
        │
        ▼
Cloudflare (CDN + DDoS + TLS)
        │
        ▼
Koyeb  ──── Go binary (Docker container)
               │
               ├── Neon PostgreSQL (managed, serverless Postgres)
               ├── Cloudflare R2 (attachments, S3-compatible)
               └── FCM / APNs (push notifications)
```

| Service | Role |
|---|---|
| **Koyeb** | Hosts the Go binary as a Docker container |
| **Cloudflare** | CDN, DDoS protection, TLS termination |
| **Neon** | Managed PostgreSQL — use the **pooled** connection string from the Neon dashboard (`?sslmode=require` is already included) |
| **Cloudflare R2** | S3-compatible object storage for file attachments |
| **FCM / APNs** | Push notifications to Android / iOS |
| **Anthropic** | Claude Haiku for NL task parsing (`POST /v1/ai/parse`) |

> All secrets are managed via Koyeb environment variables. Never call `os.Getenv` outside `config/config.go`.

---

## Stack

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
| Events | `internal/events.Dispatcher` — fire-and-forget goroutines, 30s timeout per handler |

---

## Module Structure

```
internal/
  domain/           → entities, repository interfaces, event types, enums
  events/           → Dispatcher (dispatcher.go)
  auth/
    handler/        → auth_handler.go
    repository/     → user_postgres.go, refresh_token_postgres.go
    service/        → auth_service.go
  tasks/
    handler/        → task_handler.go, dto.go
    repository/     → task_postgres.go, recurrence_postgres.go, scan.go
    service/        → task_service.go, recurrence_service.go
  analytics/
    handler/        → analytics_handler.go
    repository/     → analytics_postgres.go
    service/        → analytics_service.go
  notifications/
    handler/        → notification_handler.go
    repository/     → device_postgres.go, notification_postgres.go
    service/        → notification_service.go, push.go
  ai/
    handler/        → ai_handler.go
    service/        → parse_service.go
                       prompts/parse.md  ← edit this to tune the AI prompt
  middleware/       → jwt.go, ai_ratelimit.go
config/             → config.go
pkg/
  apperror/         → AppError + typed constructors
  pagination/       → cursor-based Page[T]
  storage/          → Client interface, FakeClient
db/migrations/      → NNN_name.up.sql / NNN_name.down.sql
cmd/server/         → main.go, bootstrap.go
```

No module imports another module directly. All cross-module communication goes through `internal/events.Dispatcher`.

---

## Core Patterns

### Errors

```go
// Repository — not found:
if errors.Is(err, pgx.ErrNoRows) {
    return nil, &domain.NotFoundError{Resource: "task", ID: id.String()}
}

// Service — translate domain errors to HTTP errors:
if domain.IsNotFound(err) {
    return nil, apperror.NotFound("task")
}
return nil, apperror.Internal(err)

// Handler — return apperror directly (httpErrorHandler in bootstrap maps it):
return apperror.BadRequest("title is required")
```

### Context

```go
// Always first parameter in repos and services:
func (r *postgresTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error)

// In Echo handlers:
result, err := h.svc.DoSomething(c.Request().Context(), ...)
```

### Interfaces in constructors

```go
// Good ✅
func NewTaskService(repo domain.TaskRepository, dispatcher *events.Dispatcher, storage storage.Client) *TaskService

// Bad ❌
func NewTaskService(repo *postgresTaskRepository) *TaskService
```

### Events

```go
// 1. Emit from domain aggregate (domain/task.go):
t.addEvent(TaskCompletedEvent{TaskID: t.ID, UserID: t.UserID, CompletedAt: now})

// 2. Dispatch from service after repo save:
for _, evt := range task.DomainEvents() {
    s.dispatcher.Dispatch(ctx, evt)
}

// 3. Register handlers in bootstrap.go only — never elsewhere:
dispatcher.Register("task.completed", analyticsService.OnTaskCompleted)
```

Event names follow `noun.verb`: `task.created`, `task.completed`, `task.archived`, `task.rescheduled`, `task.overdue`, `recurrence_instance.completed`.

### Config

```go
// config/config.go — the only place os.Getenv is called:
AnthropicModel: envStr("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"),

// bootstrap.go — pass config fields into constructors:
aisvc.NewParseService(aisvc.ParseServiceConfig{
    Model: cfg.AnthropicModel,
    ...
})
```

---

## Auth Flow

```
POST /v1/auth/register → bcrypt (cost 12) → insert user → return access + refresh token
POST /v1/auth/login    → fetch user → compare hash → return access + refresh token
POST /v1/auth/refresh  → validate refresh token → return new access token
POST /v1/auth/logout   → revoke refresh token
```

JWT payload: `sub` = `user_id` (UUID string). Extract in handlers via:
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

`due_passed` is computed — never stored. `task.EffectiveStatus()` returns it when `EndAt` is past and the task is not completed.

---

## Recurrence

- `RecurrenceRule` stored as JSONB in the `tasks` table (not a separate table)
- Instances generated lazily on calendar queries; only persisted when the user acts (complete/skip) or scheduler detects overdue
- `recurrence_instances` table: `id, task_id, scheduled_at, status (pending|completed|skipped), completed_at`
- `RecurrenceService.ProcessOverdue` is called by the scheduler

---

## AI Parsing

```
POST /v1/ai/parse  →  AIRateLimit (configurable via env)
                   →  ParseService.Parse()
                   →  renders internal/ai/service/prompts/parse.md (//go:embed)
                   →  Anthropic Messages API
                   →  ParseResult{Title, Type, Priority, StartAt, EndAt, Location, Labels, Details}
```

To tune the prompt: edit `prompts/parse.md` — no Go code changes needed.

---

## Database

### Connection (Neon)

Paste the **pooled** connection string from the Neon dashboard into `DATABASE_URL`. Pool tuned for Neon free tier:

```
DB_MAX_CONNS=10          # Neon free tier hard limit
DB_MIN_CONNS=2           # keep warm connections to avoid cold latency
DB_MAX_CONN_LIFETIME=30m # cycle before Neon drops idle connections (~5 min)
DB_MAX_CONN_IDLE_TIME=5m
```

On startup, `newDBPool` pings the DB and fails fast if unreachable.

### Migrations

```
001_create_users              → users, refresh_tokens
002_create_tasks              → tasks, task_links, task_attachments
                                 enums: task_type, task_status, task_priority, occurrence_type
003_create_recurrence_instances → recurrence_instances, enum: instance_status
004_create_notifications      → devices, notifications
005_create_analytics_events   → analytics_events
```

---

## Naming Conventions

| Thing | Convention |
|---|---|
| Packages | `lowercase`, single word |
| Types | `PascalCase` |
| Exported functions | `PascalCase` |
| Unexported functions | `camelCase` |
| DB columns | `snake_case` |
| JSON fields | `snake_case` |
| File names | `snake_case.go` |
| Event names | `noun.verb` (`task.completed`) |
