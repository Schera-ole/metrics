ALTER TABLE metrics ADD COLUMN deleted_at TIMESTAMP NULL;
CREATE INDEX idx_metrics_deleted_at ON metrics(deleted_at);