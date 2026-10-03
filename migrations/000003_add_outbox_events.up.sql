-- 000003_add_outbox_events.up.sql
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic TEXT NOT NULL,
    payload JSONB NOT NULL,
    event_key TEXT NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    headers JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON outbox_events (created_at) WHERE processed = false;
