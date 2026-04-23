# stratix-core

Modular monolith REST API for **Stratix** — a task management mobile app (iOS/Android). Built with Go and Clean Architecture. Handles task management, recurrence scheduling, analytics, push notifications, and AI-powered natural language task parsing.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Infrastructure](#infrastructure)
3. [Architecture Overview](#architecture-overview)
4. [Layer Guide — Where to Find, Add, Update, or Delete Features](#layer-guide)
5. [Dangerous Layers — What to Be Careful With](#dangerous-layers)
6. [Module Map](#module-map)
7. [Domain Model](#domain-model)
8. [Event System](#event-system)
9. [API Routes](#api-routes)
10. [Configuration](#configuration)
11. [Database](#database)
12. [Q&A — Business and Technical](#qa)

---

## Getting Started

### Prerequisites

- Go 1.21+
- A running PostgreSQL database (or a [Neon](https://neon.tech) connection string)

### 1. Clone and install dependencies

```bash
git clone https://github.com/luissebastian953/stratix-core
cd stratix-core
go mod download
```

### 2. Set environment variables

Copy the required variables into a `.env` file (or export them directly):

```env
DATABASE_URL=postgres://user:password@host/dbname
JWT_SECRET=your-secret-here
ANTHROPIC_API_KEY=sk-ant-...
```

See the [Configuration](#configuration) section for the full list of variables.

### 3. Run the server

```bash
go run ./cmd/server
```

Or build first then run:

```bash
go build -o stratix-core ./cmd/server
./stratix-core
```

The server starts on port `8080` by default (`APP_PORT` to override).

---

## Infrastructure

```
Mobile app (iOS / Android)
        │
        ▼
Cloudflare  (CDN · DDoS · TLS termination)
        │
        ▼
Koyeb  ──── Go binary (Docker container)
               │
               ├── Neon PostgreSQL
               ├── Cloudflare R2  (file attachments)
               └── FCM / APNs     (push notifications)
```

| Service | Role |
|---|---|
| **Koyeb** | Runs the Go binary as a Docker container |
| **Cloudflare** | CDN, DDoS protection, TLS — the app server speaks plain HTTP |
| **Neon** | Managed serverless PostgreSQL |
| **Cloudflare R2** | S3-compatible object storage for task attachments |
| **FCM / APNs** | Push notifications to Android and iOS devices |
| **Anthropic** | Claude Haiku for natural language task parsing |

---

## Architecture Overview

stratix-core uses **Clean Architecture** organized in three concentric layers:

```
┌─────────────────────────────────────────────────┐
│                   HTTP Layer                    │  ← handlers, middleware, request/response DTOs
│              internal/*/handler/                │
├─────────────────────────────────────────────────┤
│                 Application Layer               │  ← business logic, use cases, orchestration
│              internal/*/service/                │
├─────────────────────────────────────────────────┤
│                  Domain Layer                   │  ← entities, rules, interfaces, events
│              internal/domain/                   │
├─────────────────────────────────────────────────┤
│               Infrastructure Layer              │  ← database, external APIs, storage
│    internal/*/repository/ · internal/ai/        │
└─────────────────────────────────────────────────┘
```

**The golden rule:** dependencies only point inward. Handlers import services. Services import domain interfaces. Repositories implement domain interfaces. Nothing in domain imports anything outside domain.

```
handler → service → domain interface ← repository (implements it)
```

No module ever imports another module directly. Cross-module communication happens exclusively through the **event system** (`internal/events.Dispatcher`).

---

## Layer Guide

This section tells you exactly where to go for every kind of change.

### Adding a new feature end-to-end

Follow this checklist in order:

1. **Domain** (`internal/domain/`) — define the entity and its rules
2. **Repository interface** (`internal/domain/repository.go`) — declare what DB operations you need
3. **Migration** (`db/migrations/`) — create the table
4. **Repository implementation** (`internal/*/repository/*_postgres.go`) — implement the interface
5. **Service** (`internal/*/service/*_service.go`) — write the use case
6. **Handler + DTO** (`internal/*/handler/`) — expose the HTTP endpoint
7. **Bootstrap** (`cmd/server/bootstrap.go`) — wire everything together

---

### HTTP Layer — `internal/*/handler/`

**What lives here:**
- Binding and validating HTTP request bodies
- Extracting path/query params and user identity from context
- Calling the service layer
- Mapping results to JSON responses

**What does NOT belong here:**
- Business rules or decisions
- Direct database access
- Anything that would need testing without an HTTP request

**To add an endpoint:**
1. Add the route in `handler.Register(g *echo.Group)`
2. Write the handler method — bind, validate, call service, return JSON
3. Add request/response types in `dto.go` if the module has one

**To change a response shape:**
- Edit the response struct in `dto.go` (or the handler file)
- Response structs are separate from domain entities — changing a response field does not touch the DB

**Files:**
```
internal/auth/handler/auth_handler.go
internal/tasks/handler/task_handler.go
internal/tasks/handler/dto.go
internal/analytics/handler/analytics_handler.go
internal/notifications/handler/notification_handler.go
internal/ai/handler/ai_handler.go
```

---

### Application Layer — `internal/*/service/`

**What lives here:**
- All business logic and use case orchestration
- Ownership and authorization checks
- Calling repositories to read/write data
- Dispatching domain events after state changes
- Calling external services (AI, push)

**This is the most important layer to understand.** When a product requirement changes, this is almost always the file that needs to change.

**To add a use case** (e.g. "bulk archive tasks"):
1. Add a method to the relevant service struct
2. Use existing repository methods; add new ones if needed
3. Dispatch a domain event if other modules should react

**To change business rules** (e.g. "completed tasks can now be reopened after 7 days"):
- Edit the domain entity method (e.g. `task.Reopen()`) or the service method that enforces the rule

**Files:**
```
internal/auth/service/auth_service.go
internal/tasks/service/task_service.go
internal/tasks/service/recurrence_service.go
internal/analytics/service/analytics_service.go
internal/notifications/service/notification_service.go
internal/ai/service/parse_service.go
```

---

### Domain Layer — `internal/domain/`

**What lives here:**
- Entity structs (`Task`, `User`, `Device`, etc.)
- Entity methods that enforce invariants (`task.Complete()`, `task.Schedule()`, etc.)
- Repository interfaces (contracts that the infrastructure layer must fulfill)
- Domain event types (`TaskCompletedEvent`, etc.)
- Enums and value types (`TaskType`, `Status`, `Priority`, etc.)
- Error types (`NotFoundError`, `InvalidTransitionError`)

**This layer has zero external dependencies** — no database, no HTTP, no third-party packages (except `uuid` and stdlib).

**To add a new entity field:**
1. Add the field to the struct in `domain/`
2. Add a setter method if the field has validation rules
3. Update the repository interface if new DB operations are needed
4. Update the migration and the `*_postgres.go` scan/insert queries

**To add a new status or type enum value:**
1. Add the constant in `domain/enums.go`
2. Add it to the relevant `validate*` switch in `domain/task.go`
3. Add it to the DB enum in a new migration (`ALTER TYPE ... ADD VALUE`)

**Files:**
```
internal/domain/task.go          ← Task aggregate + all methods
internal/domain/repository.go    ← all repository interfaces
internal/domain/events.go        ← DomainEvent interface + all event structs
internal/domain/enums.go         ← TaskType, Status, Priority, OccurrenceType, InstanceStatus
internal/domain/recurrance.go    ← RecurrenceRule, RecurrenceType
internal/domain/value_objects.go ← TaskLink, TaskAttachmentRef, etc.
```

---

### Infrastructure Layer — `internal/*/repository/`

**What lives here:**
- Concrete PostgreSQL implementations of domain repository interfaces
- Raw SQL queries using `pgx/v5`
- Row scanning helpers
- No business logic — only data in and data out

**To add a new query:**
1. Add the method signature to the interface in `domain/repository.go`
2. Implement it in the corresponding `*_postgres.go` file
3. Use `$1, $2, ...` parameterized queries — never string concatenation

**Files:**
```
internal/auth/repository/user_postgres.go
internal/auth/repository/refresh_token_postgres.go
internal/tasks/repository/task_postgres.go
internal/tasks/repository/recurrence_postgres.go
internal/tasks/repository/scan.go
internal/analytics/repository/analytics_postgres.go
internal/notifications/repository/device_postgres.go
internal/notifications/repository/notification_postgres.go
```

---

## Dangerous Layers

Some parts of the codebase have outsized blast radius. Changes here can silently break many other things.

### `internal/domain/` — HIGH RISK

This is the foundation everything else is built on. Changes here ripple outward.

| What you change | What can break |
|---|---|
| Rename an entity field | All repository scan functions, all service references, all handler DTOs |
| Change a method signature (e.g. `task.Complete()`) | Every service that calls it |
| Change an enum value string | All existing DB rows with that value, all clients that send/receive it |
| Change a repository interface | The postgres implementation must be updated to match |
| Remove a domain event type | Any handler registered for that event in bootstrap silently becomes unreachable |

**Rule:** treat domain changes like DB migrations — they need to be coordinated across all layers.

---

### `db/migrations/` — HIGH RISK

Migrations are permanent and run in order. A bad migration applied to production is hard to undo.

| What you change | What can break |
|---|---|
| Rename a column | All `SELECT`/`INSERT`/`UPDATE` queries in every `*_postgres.go` that touch that table |
| Change an enum (remove a value) | Existing rows with that value will fail to scan |
| Drop a table | Every repository that queries it |
| Change a column type | Scan functions that assign it to a typed Go variable |

**Rule:** never edit an already-applied migration. Always create a new one.

---

### `cmd/server/bootstrap.go` — MEDIUM RISK

This is the wiring file — it constructs every service and registers every event handler. Easy to introduce subtle bugs:

- Registering an event handler with a typo in the event name means the handler silently never fires
- Passing the wrong config field to a constructor compiles fine but misbehaves at runtime
- Removing a `dispatcher.Register` call silently disables a downstream side effect (e.g. analytics stops recording)

**Rule:** after any change here, verify the event registration table at the bottom of the file matches what the services actually dispatch.

---

### `internal/events/dispatcher.go` — MEDIUM RISK

Event handlers run in goroutines. Bugs here affect all async behavior:

- A panic in a handler is caught and logged, but the side effect is lost
- If the handler context timeout (30s) is too short, long-running handlers will be silently cancelled
- Changing `Dispatch` to be synchronous would change the semantics of every feature that relies on async fan-out

---

## Module Map

```
internal/
├── domain/                   Core entities and contracts
│   ├── task.go               Task aggregate (all task business rules live here)
│   ├── repository.go         All repository interfaces
│   ├── events.go             DomainEvent interface + all event structs
│   ├── enums.go              TaskType, Status, Priority, OccurrenceType, InstanceStatus
│   ├── recurrance.go         RecurrenceRule struct and RecurrenceType
│   └── value_objects.go      TaskLink, TaskAttachmentRef
│
├── events/
│   └── dispatcher.go         Pub/sub event bus (fire-and-forget goroutines)
│
├── auth/
│   ├── handler/auth_handler.go
│   ├── repository/user_postgres.go
│   ├── repository/refresh_token_postgres.go
│   └── service/auth_service.go
│
├── tasks/
│   ├── handler/task_handler.go   8 routes (list, create, get, update, delete, complete, archive, children)
│   ├── handler/dto.go            Request/response types + converters
│   ├── repository/task_postgres.go
│   ├── repository/recurrence_postgres.go
│   ├── repository/scan.go
│   ├── service/task_service.go
│   └── service/recurrence_service.go
│
├── analytics/
│   ├── handler/analytics_handler.go   3 routes (summary, distribution, activity)
│   ├── repository/analytics_postgres.go
│   └── service/analytics_service.go
│
├── notifications/
│   ├── handler/notification_handler.go   3 routes (register device, unregister, list)
│   ├── repository/device_postgres.go
│   ├── repository/notification_postgres.go
│   └── service/notification_service.go + push.go
│
├── ai/
│   ├── handler/ai_handler.go
│   └── service/
│       ├── parse_service.go
│       └── prompts/parse.md     ← Edit this file to change the AI prompt
│
└── middleware/
    ├── jwt.go                   Validates JWT, sets UserIDKey in context
    └── ai_ratelimit.go          Per-user rate limiter (configurable via env)

config/config.go                 All env vars with defaults — os.Getenv only here
pkg/apperror/errors.go           AppError, BadRequest, NotFound, Internal, etc.
pkg/pagination/                  Cursor-based pagination Page[T]
pkg/storage/                     Storage Client interface + FakeClient
db/migrations/                   001–005 SQL migration files
cmd/server/
    main.go                      Entry point
    bootstrap.go                 Wires all modules together
```

---

## Domain Model

### Task

The central entity. A task belongs to a user and has:

| Field | Type | Notes |
|---|---|---|
| `Type` | enum | `todo` `travel` `meeting` `reminder` `focus` `log` |
| `Status` | enum | `backlog` `scheduled` `in_progress` `completed` `cancelled` |
| `EffectiveStatus` | computed | adds `due_passed` when end time has passed — never stored |
| `Priority` | enum | `highest` `high` `medium` `low` `lowest` |
| `OccurrenceType` | enum | `once` `recurring` `unbound` (log tasks only) |
| `RecurrenceRule` | JSONB | stored inline on the task — no separate table |
| `Labels` | `TEXT[]` | arbitrary tags |
| `Links` | relation | `task_links` table |
| `Attachments` | relation | `task_attachments` table |
| `ParentID` | nullable FK | sub-task support |

**Status transitions:**
```
backlog ──→ scheduled
backlog ──→ in_progress
scheduled ──→ in_progress
scheduled ──→ completed
in_progress ──→ completed
in_progress ──→ cancelled
completed ──→ backlog  (reopen)
cancelled    (terminal)
```

`due_passed` is returned by `EffectiveStatus()` when `EndAt` is in the past and the task is not yet completed. It is never written to the database.

### Recurrence

- Rules are stored as JSONB inside the `tasks` table — no separate recurrence rules table.
- Instances (`recurrence_instances`) are generated **lazily** on calendar queries. They are not pre-generated.
- An instance is only persisted when a user acts on it (completes or skips it) or when the scheduler marks it overdue.

---

## Event System

All cross-module side effects go through the event bus. No module imports another.

```
Producer                     Event                        Consumer
────────────────────────────────────────────────────────────────────
task.Complete()          →   task.completed           →   analytics.OnTaskCompleted
task.NewTask()           →   task.created             →   analytics.OnTaskCreated
task.Archive()           →   task.archived            →   analytics.OnTaskArchived
task.Schedule() (update) →   task.rescheduled         →   analytics.OnTaskRescheduled
RecurrenceService        →   recurrence_instance.completed → analytics.OnRecurrenceInstanceCompleted
RecurrenceService        →   task.overdue             →   notifications.OnTaskOverdue
```

Handlers run in goroutines with a 30-second timeout. A handler failure is logged but does not fail the originating request.

To add a new event reaction:
1. Define the event struct in `internal/domain/events.go` if it doesn't exist
2. Call `t.addEvent(...)` in the domain method, or `dispatcher.Dispatch(...)` in the service
3. Write a handler method on the target service (`OnXxx(ctx, event)`)
4. Register it in `bootstrap.go`: `dispatcher.Register("event.name", service.OnXxx)`

---

## API Routes

All routes are under `/v1`. Protected routes require `Authorization: Bearer <jwt>`.

### Auth (public)
| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/auth/register` | Create account |
| `POST` | `/v1/auth/login` | Get access + refresh token |
| `POST` | `/v1/auth/refresh` | Rotate refresh token |
| `POST` | `/v1/auth/logout` | Revoke refresh token |

### Tasks (protected)
| Method | Path | Description |
|---|---|---|
| `GET` | `/v1/tasks` | List tasks (filterable, cursor-paginated) |
| `POST` | `/v1/tasks` | Create task |
| `GET` | `/v1/tasks/:id` | Get single task |
| `PATCH` | `/v1/tasks/:id` | Partial update |
| `DELETE` | `/v1/tasks/:id` | Delete task |
| `POST` | `/v1/tasks/:id/complete` | Mark completed |
| `POST` | `/v1/tasks/:id/archive` | Archive task |
| `GET` | `/v1/tasks/:id/children` | List sub-tasks |

### Analytics (protected)
| Method | Path | Description |
|---|---|---|
| `GET` | `/v1/analytics/summary` | Total/completed/overdue counts |
| `GET` | `/v1/analytics/distribution` | Count by task type |
| `GET` | `/v1/analytics/activity?days=30` | Completions per day |

### Notifications (protected)
| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/notifications/devices` | Register device token |
| `DELETE` | `/v1/notifications/devices/:device_id` | Unregister device |
| `GET` | `/v1/notifications` | List received notifications |

### AI (protected · rate limited)
| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/ai/parse` | Parse natural language into task fields |

`POST /v1/ai/parse` accepts `{ "input": "Call dentist tomorrow at 3pm" }` and returns structured task fields. Rate limited to 20 requests per hour per user (configurable).

---

## Configuration

All values are read in `config/config.go`. Set them as environment variables (Koyeb dashboard) or in `.env` for local development.

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | — | Neon pooled connection string (required) |
| `DB_MAX_CONNS` | `10` | Max DB connections (Neon free tier limit) |
| `DB_MIN_CONNS` | `2` | Min warm connections |
| `DB_MAX_CONN_LIFETIME` | `30m` | Cycle connections before Neon drops them |
| `DB_MAX_CONN_IDLE_TIME` | `5m` | Return idle connections to pool |
| `JWT_SECRET` | — | HMAC secret for JWT signing (required) |
| `JWT_ACCESS_EXP_MINUTES` | `15` | Access token lifetime |
| `JWT_REFRESH_EXP_DAYS` | `30` | Refresh token lifetime |
| `ANTHROPIC_API_KEY` | — | Anthropic API key (required for AI parsing) |
| `ANTHROPIC_API_URL` | `https://api.anthropic.com/v1/messages` | Anthropic endpoint |
| `ANTHROPIC_MODEL` | `claude-haiku-4-5-20251001` | Model to use |
| `ANTHROPIC_API_VERSION` | `2023-06-01` | Anthropic API version header |
| `ANTHROPIC_MAX_TOKENS` | `512` | Max tokens in AI response |
| `ANTHROPIC_TIMEOUT_SECONDS` | `15` | HTTP timeout for Anthropic calls |
| `AI_RATE_LIMIT_REQUESTS` | `20` | Max AI requests per user per window |
| `AI_RATE_LIMIT_WINDOW` | `1h` | Rate limit window duration |

---

## Database

### Migrations

Run in order. Files in `db/migrations/`:

| Migration | Creates |
|---|---|
| `001_create_users` | `users`, `refresh_tokens` |
| `002_create_tasks` | `tasks`, `task_links`, `task_attachments` + enums |
| `003_create_recurrence_instances` | `recurrence_instances` |
| `004_create_notifications` | `devices`, `notifications` |
| `005_create_analytics_events` | `analytics_events` |

**Never edit an already-applied migration.** Create a new one instead.

### Connection

On startup, `newDBPool` parses the connection string, applies pool settings, and pings the database. The server will not start if the DB is unreachable.

---

## Q&A

### Business Questions

**Q: What is a "task" in this system?**
A task is the core unit of the app. It can represent a to-do, a meeting, a travel plan, a reminder, a focus session, or a log entry. Each task belongs to a single user and can have a scheduled time, priority, labels, links, file attachments, and sub-tasks.

**Q: What does "complete a task" actually do?**
Completing a task transitions its status from `in_progress` (or `scheduled`) to `completed`. This also fires a `task.completed` event, which increments the user's analytics counters in the background. A completed task can be reopened (moved back to `backlog`).

**Q: What is the difference between archiving and deleting a task?**
Archiving (`archived = true`) soft-hides the task from the default list view but keeps all data. Deleting permanently removes the task and all its links, attachments, and recurrence instances via database cascades.

**Q: How does the AI parsing work from a user's perspective?**
The user sends a free-text description like `"dentist appointment Friday at 10am, high priority"`. The API calls Claude Haiku with a structured prompt, and Claude returns a JSON object with the title, type, start time, priority, etc. pre-filled. The user can review and confirm before saving.

**Q: What is a recurring task?**
A recurring task has a `RecurrenceRule` (frequency, interval, days, etc.) stored on the task. The system generates individual *instances* of that task for specific dates. Each instance can be completed or skipped independently. This is like a calendar repeat — the parent task holds the rule, the instances are the individual occurrences.

**Q: How do push notifications work?**
When a task goes overdue (detected by the scheduler), a `task.overdue` event is dispatched. The notification service listens for this event, finds all registered device tokens for that user, and sends a push notification to each one via FCM (Android) or APNs (iOS).

**Q: What analytics does the app track?**
Currently: task created, completed, and archived events (stored in `analytics_events`). The analytics API exposes a summary (totals), type distribution (how many todos vs meetings etc.), and a daily completion chart (streak/activity data).

---

### Technical Questions

**Q: Why is there no ORM?**
Raw `pgx` with hand-written SQL keeps the queries explicit and performant. There are no N+1 surprises, no magic field mapping, and no dependency on a code generator. The tradeoff is more boilerplate in the repository layer.

**Q: Why is the event dispatcher fire-and-forget?**
Side effects like analytics recording and push notifications should not slow down or fail the user's request. If analytics recording fails, the task is still saved. Each handler runs in its own goroutine with a 30-second timeout and its own recovery from panics.

**Q: Why does `GetByID` return 404 instead of 403 when a task belongs to another user?**
Returning 403 ("Forbidden") would confirm that a resource with that ID exists, which leaks information. 404 ("Not Found") treats unauthorized access the same as non-existence — the caller cannot tell the difference.

**Q: Why is `due_passed` not stored in the database?**
It is a derived state, not a real status transition — the task hasn't changed, time has. Storing it would require a background job to update rows as time passes. Instead, `task.EffectiveStatus()` computes it on read by comparing `EndAt` to the current time.

**Q: Why is `RecurrenceRule` stored as JSONB instead of a separate table?**
A recurrence rule is not a separate entity — it has no identity of its own, no lifecycle independent of its task, and is always read together with the task. JSONB avoids a join on every task fetch and keeps the schema simpler.

**Q: Why are recurrence instances generated lazily?**
Pre-generating thousands of future instances for every recurring task wastes storage and becomes stale if the rule changes. Lazy generation means instances only exist when they are relevant (within the queried date range or overdue). The tradeoff is that the query that generates them is slightly more complex.

**Q: How do I add a completely new module (e.g. a "projects" feature)?**
1. Define the `Project` entity in `internal/domain/` (or a new file there)
2. Add `ProjectRepository` interface to `internal/domain/repository.go`
3. Write the migration in `db/migrations/`
4. Create `internal/projects/repository/project_postgres.go`
5. Create `internal/projects/service/project_service.go`
6. Create `internal/projects/handler/project_handler.go` + `dto.go`
7. Wire it in `cmd/server/bootstrap.go`
If the projects feature needs to react to task events (e.g. count completed tasks per project), add a handler method on `ProjectService` and register it in bootstrap — no changes to the tasks module.

**Q: How do I change the AI prompt?**
Edit `internal/ai/service/prompts/parse.md`. The file is embedded at compile time — no code changes needed. Use `{{.Now}}` for the current timestamp and `{{.Input}}` for the user's text. Redeploy after editing.

**Q: What happens if Anthropic is down?**
`ParseService.Parse` returns an `apperror.Internal` error. The handler returns HTTP 500. The task is not created. The AI feature degrades gracefully — all other endpoints continue to work normally.

**Q: How is authentication handled in handlers?**
The JWT middleware (`internal/middleware/jwt.go`) validates the token and writes the `user_id` string into the Echo context under `middleware.UserIDKey`. Every protected handler reads it with `c.Get(middleware.UserIDKey).(string)` and passes it to the service for ownership checks.

**Q: What is the blast radius of changing a domain entity field?**
Large. If you rename a field on `Task`, you must update: the repository scan/insert queries, the service layer references, the handler DTO converters, and any event structs that carry that field. Always search the whole codebase (`grep -r "FieldName" internal/`) before renaming.

**Q: Where is the single most important file in the codebase?**
`internal/domain/task.go`. It contains the Task aggregate and all business rules for the core feature. Everything else in the system exists to support or expose what's defined there.
