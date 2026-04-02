-- +goose Up
CREATE TABLE outb_outbox_messages (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type     varchar(128) NOT NULL,
    target_url     text         NOT NULL,
    payload        jsonb        NOT NULL DEFAULT '{}',
    status         varchar(32)  NOT NULL DEFAULT 'pending',
    attempts       int          NOT NULL DEFAULT 0,
    max_attempts   int          NOT NULL DEFAULT 5,
    next_retry_at  timestamptz  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_error     text,
    sent_at        timestamptz,
    tenant_id      varchar(64),
    created_by     varchar(128),
    created_at     timestamptz  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     timestamptz  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON COLUMN outb_outbox_messages.status IS 'pending | sent | failed | dead';

CREATE INDEX idx_outb_outbox_status_retry
    ON outb_outbox_messages (status, next_retry_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX idx_outb_outbox_tenant_id
    ON outb_outbox_messages (tenant_id);

CREATE INDEX idx_outb_outbox_event_type
    ON outb_outbox_messages (event_type);

CREATE TRIGGER update_outb_outbox_messages_modtime
    BEFORE UPDATE ON outb_outbox_messages
    FOR EACH ROW EXECUTE FUNCTION update_modified_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_outb_outbox_messages_modtime ON outb_outbox_messages;
DROP INDEX IF EXISTS idx_outb_outbox_event_type;
DROP INDEX IF EXISTS idx_outb_outbox_tenant_id;
DROP INDEX IF EXISTS idx_outb_outbox_status_retry;
DROP TABLE IF EXISTS outb_outbox_messages;
