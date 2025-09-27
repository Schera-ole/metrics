DROP INDEX IF EXISTS idx_metrics_deleted_at;
ALTER TABLE metrics DROP COLUMN IF EXISTS deleted_at;