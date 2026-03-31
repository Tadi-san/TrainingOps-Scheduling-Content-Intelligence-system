BEGIN;

CREATE TABLE IF NOT EXISTS ingestion_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    proxy TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ingestion_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id UUID NULL REFERENCES ingestion_sessions(id) ON DELETE SET NULL,
    url TEXT NOT NULL,
    proxy TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    delay_ms INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL,
    reason TEXT NULL,
    requires_manual_review BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS manual_review_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id)
);

CREATE INDEX IF NOT EXISTS idx_ingestion_sessions_tenant_last_seen
    ON ingestion_sessions (tenant_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_tenant_created
    ON ingestion_jobs (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_manual_review_queue_tenant_status
    ON manual_review_queue (tenant_id, status, created_at DESC);

COMMIT;
