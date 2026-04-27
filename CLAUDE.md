# stratix-core — Claude Instructions

## Run the server

```bash
go run ./cmd/server
```

Build and run:

```bash
go build -o stratix-core ./cmd/server && ./stratix-core
```

## Database

```bash
go run ./cmd/migrate up          # apply all pending migrations
go run ./cmd/migrate down        # roll back 1 migration
go run ./cmd/migrate down 3      # roll back 3 migrations
go run ./cmd/migrate version     # show current version
go run ./cmd/migrate seed        # insert dummy data (dev only)
```

> **Warning:** `down` drops tables and data — never run against production.
> **Warning:** `seed` is dev-only — verify `DATABASE_URL` in `.env` before running.

Seed users: `alice@example.com` and `bob@example.com`, password `password123`.

## Common commands

```bash
go build ./...          # verify the project compiles
go vet ./...            # static analysis
go test ./...           # run all tests
```

## Required environment variables

```env
DATABASE_URL=           # Neon / PostgreSQL connection string
JWT_SECRET=             # HMAC secret for JWT signing
ANTHROPIC_API_KEY=      # Anthropic API key (needed for /v1/ai/parse)
```

## Architecture

Clean Architecture — dependencies point inward only:

```
handler → service → domain interface ← repository
```

- `internal/domain/` — entities, interfaces, events (no external deps)
- `internal/*/service/` — business logic
- `internal/*/handler/` — HTTP layer (Echo)
- `internal/*/repository/` — PostgreSQL (pgx/v5, raw SQL)
- `cmd/server/bootstrap.go` — wires everything together

See README.md for the full layer guide, module map, and API routes.

## Key rules

- Never edit an already-applied migration in `db/migrations/` — always create a new one.
- Domain changes (`internal/domain/`) ripple to all layers — search before renaming.
- Cross-module communication goes through the event dispatcher only (`internal/events/`).
- Parameterized queries only (`$1, $2, ...`) — never string concatenation in SQL.
- Go 1.21+ built-ins: use `min(a, b)` / `max(a, b)` directly, no helper needed.
