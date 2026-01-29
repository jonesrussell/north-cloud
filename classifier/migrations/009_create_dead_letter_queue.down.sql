-- Rollback: Remove dead letter queue

DROP INDEX IF EXISTS idx_dlq_error_code;
DROP INDEX IF EXISTS idx_dlq_exhausted;
DROP INDEX IF EXISTS idx_dlq_source;
DROP INDEX IF EXISTS idx_dlq_next_retry;
DROP TABLE IF EXISTS dead_letter_queue;
