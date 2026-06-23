CREATE TABLE jobs (
    id BIGSERIAL PRIMARY KEY,
    command JSONB NOT NULL,
    scheduled_at TIMESTAMP NOT NULL,
    picked_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    recurring_job_id BIGINT
);

CREATE INDEX idx_jobs_pending ON jobs(scheduled_at) WHERE picked_at IS NULL;

CREATE TABLE recurring_jobs (
    id BIGSERIAL PRIMARY KEY,
    command JSONB NOT NULL,
    cron_expr VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
