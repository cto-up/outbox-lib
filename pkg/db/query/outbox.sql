-- name: CreateOutboxMessage :one
INSERT INTO outb_outbox_messages (
    event_type,
    target_url,
    payload,
    max_attempts,
    tenant_id,
    created_by
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListOutboxMessages :many
SELECT * FROM outb_outbox_messages
WHERE (sqlc.narg('tenant_id')::text IS NULL OR tenant_id = sqlc.narg('tenant_id'))
  AND (sqlc.narg('status')::text   IS NULL OR status     = sqlc.narg('status'))
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type'))
ORDER BY created_at DESC
LIMIT  sqlc.arg('limit')::int
OFFSET sqlc.arg('offset')::int;

-- name: CountOutboxMessages :one
SELECT COUNT(*) FROM outb_outbox_messages
WHERE (sqlc.narg('tenant_id')::text IS NULL OR tenant_id = sqlc.narg('tenant_id'))
  AND (sqlc.narg('status')::text   IS NULL OR status     = sqlc.narg('status'))
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type'));

-- name: GetOutboxMessageByID :one
SELECT * FROM outb_outbox_messages WHERE id = $1;

-- name: ClaimPendingMessages :many
SELECT * FROM outb_outbox_messages
WHERE status IN ('pending', 'failed')
  AND next_retry_at <= NOW()
ORDER BY next_retry_at ASC
LIMIT 50
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxMessageSent :one
UPDATE outb_outbox_messages
SET status   = 'sent',
    sent_at  = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkOutboxMessageFailed :one
UPDATE outb_outbox_messages
SET status        = $2,
    attempts      = $3,
    last_error    = $4,
    next_retry_at = $5,
    updated_at    = NOW()
WHERE id = $1
RETURNING *;

-- name: ResetOutboxMessage :one
UPDATE outb_outbox_messages
SET status        = 'pending',
    attempts      = 0,
    last_error    = NULL,
    next_retry_at = NOW(),
    sent_at       = NULL,
    updated_at    = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteOutboxMessage :exec
DELETE FROM outb_outbox_messages WHERE id = $1;
