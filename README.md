# outbox-lib

A Go library implementing the **Transactional Outbox pattern** for reliable asynchronous HTTP webhook delivery. Enqueue outbound calls atomically with your business writes; a background dispatcher handles retries, backoff, and dead-letter handling.

## Overview

```
Your service                      outbox-lib                       Target
──────────────                    ──────────                       ──────
BEGIN tx
  write business data   ──────►  INSERT outbox row (pending)
COMMIT tx                         ↓
                                  dispatcher polls DB
                                  POST payload ──────────────────► https://...
                                  on success → status=sent
                                  on failure → status=failed, schedule retry
                                  after N failures → status=dead
```

Key properties:

- **At-least-once delivery** — no webhook is silently dropped
- **Transactional safety** — `EnqueueTx()` lets you enqueue inside an existing `pgx.Tx`
- **Multi-tenant** — all rows are scoped by `tenant_id`
- **Admin API** — list, retry, and delete messages via a Gin REST endpoint

## Prerequisites

| Tool              | Version | Install                                                                      |
| ----------------- | ------- | ---------------------------------------------------------------------------- |
| Go                | 1.25+   | https://go.dev/dl                                                            |
| PostgreSQL        | 14+     | `docker compose` target below                                                |
| sqlc              | latest  | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`                        |
| oapi-codegen      | latest  | `go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest` |
| openapi-generator | latest  | `npm install -g @openapitools/openapi-generator-cli`                         |

## Quickstart

```bash
# 1. Copy and edit environment config
cp .env.example .env   # edit DATABASE_URL, DATABASE_USERNAME, DATABASE_PASSWORD

# 2. Start local PostgreSQL
make postgresup

# 3. Run migrations (via your migration runner — see pkg/db/migration/)

# 4. Build
make build
```

## Using the Library

### Enqueue a webhook (without transaction)

Use when there is no surrounding database transaction — the enqueue itself is the only write.

```go
import (
    "github.com/cto-up/outbox-lib/pkg/db"
    "github.com/cto-up/outbox-lib/pkg/service"
)

store := db.NewStore(pool)
svc   := service.NewService(store)

err := svc.Enqueue(ctx, service.Message{
    EventType:   "order.created",
    TargetURL:   "https://partner.example.com/webhooks",
    Payload:     orderPayload,   // any JSON-serializable value
    TenantID:    tenantID,
    CreatedBy:   userID,
    MaxAttempts: 5,              // optional, default 5
})
```

### Enqueue a webhook (inside a transaction)

Use when you need the webhook row and your business data to be committed atomically. If the transaction rolls back, the webhook is never enqueued.

```go
tx, err := pool.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)

// Your business write
_, err = tx.Exec(ctx, "INSERT INTO orders ...", ...)
if err != nil {
    return err
}

// Webhook enqueued in the same transaction
err = svc.EnqueueTx(ctx, tx, service.Message{
    EventType:   "order.created",
    TargetURL:   "https://partner.example.com/webhooks",
    Payload:     orderPayload,
    TenantID:    tenantID,
    CreatedBy:   userID,
    MaxAttempts: 5,
})
if err != nil {
    return err
}

return tx.Commit(ctx)
```

### Register the admin API

```go
import (
    outboxapi "github.com/cto-up/outbox-lib/pkg/api"
    api       "github.com/cto-up/outbox-lib/api/openapi"
)

outboxapi.RegisterHandler(store, api.GinServerOptions{}, router)
```

This mounts the following routes under `/admin-api/v1/outbox`:

| Method   | Path                   | Role                | Description                                 |
| -------- | ---------------------- | ------------------- | ------------------------------------------- |
| `GET`    | `/messages`            | ADMIN / SUPER_ADMIN | List messages with filtering and pagination |
| `POST`   | `/messages/{id}/retry` | ADMIN / SUPER_ADMIN | Reset a failed/dead message to pending      |
| `DELETE` | `/messages/{id}`       | ADMIN / SUPER_ADMIN | Hard-delete a message                       |

SUPER_ADMIN can query across all tenants. ADMIN is automatically scoped to their own `tenant_id`.

### Query parameters for `GET /messages`

| Param        | Type   | Description                                   |
| ------------ | ------ | --------------------------------------------- |
| `status`     | string | Filter by `pending`, `sent`, `failed`, `dead` |
| `event_type` | string | Filter by event type label                    |
| `tenant_id`  | string | SUPER_ADMIN only                              |
| `page`       | int    | Page number (default `1`)                     |
| `page_size`  | int    | Results per page (default `50`)               |

## Message lifecycle

```
pending ──► sent       (delivery succeeded)
        ──► failed     (attempt failed, will retry)
               ──► pending  (next retry window elapsed)
               ──► dead     (max_attempts reached)
                      ──► pending  (manual retry via API)
```

The dispatcher (external to this library) should call `store.ClaimPendingMessages()` on a schedule. That query uses `FOR UPDATE SKIP LOCKED` so multiple dispatcher instances can run safely.

## Development

### Make targets

| Target                                    | Description                                      |
| ----------------------------------------- | ------------------------------------------------ |
| `make build`                              | Compile all packages                             |
| `make postgresup`                         | Start local PostgreSQL via Docker Compose        |
| `make postgresdown`                       | Stop local PostgreSQL                            |
| `make sqlc`                               | Regenerate `pkg/db/repository/` from SQL queries |
| `make openapi`                            | Regenerate Go server stubs and TypeScript client |
| `make update-core-backend VERSION=v0.x.x` | Bump `ctoup.com/coreapp` dependency              |
| `make release VERSION=v0.x.x NOTES="..."` | Create a GitHub release                          |

### Code generation

All generated files are committed. Regenerate after changing source definitions:

```
pkg/db/query/outbox.sql          ──► make sqlc     ──► pkg/db/repository/*.go
pkg/api/openapi/outbox-api.yaml  ──► make openapi  ──► api/openapi/outbox-service.go
pkg/api/openapi/outbox-schema.yaml                 ──► api/openapi/outbox-schema.go
                                                   ──► ../outbox-fe-lib/ (TypeScript)
```

### Project layout

```
outbox-lib/
├── api/openapi/           # Generated Go server stubs (oapi-codegen output)
├── pkg/
│   ├── service/           # Enqueue() / EnqueueTx() — public API for callers
│   ├── api/               # Gin handler — admin REST endpoints
│   │   └── openapi/       # OpenAPI source specs + code-gen configs
│   └── db/
│       ├── store.go       # Store struct + ExecTx helper
│       ├── migration/     # Embedded SQL migrations
│       ├── query/         # sqlc query definitions
│       └── repository/    # Generated DB access code (sqlc output)
├── docker/                # Docker Compose files
├── Makefile
└── go.mod
```

### Database schema

Single table `outb_outbox_messages` with indexes optimised for the dispatcher:

- `(status, next_retry_at)` — partial index on `pending`/`failed` rows
- `tenant_id` — tenant-scoped admin queries
- `event_type` — filtered listings

See `pkg/db/migration/20260402000000_outbox.sql` for the full DDL.

## Configuration

Copy `.env.example` to `.env`. Required variables:

```ini
DATABASE_URL       = 127.0.0.1:5432/mydb?sslmode=disable
DATABASE_USERNAME  = myuser
DATABASE_PASSWORD  = mypassword
```

## Releasing

```bash
make release VERSION=v0.2.0 NOTES="Add retry endpoint"
```

This creates a GitHub release via `gh`. The module is consumed via Go module proxy, so tag format must be `vMAJOR.MINOR.PATCH`.
