-- 000003_add_outbox_events.down.sql
DROP INDEX IF EXISTS idx_outbox_unprocessed;
DROP TABLE IF EXISTS outbox_events CASCADE;
