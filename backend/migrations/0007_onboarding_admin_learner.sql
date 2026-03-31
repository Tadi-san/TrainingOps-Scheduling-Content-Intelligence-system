BEGIN;

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS policies JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE uploaded_files
    ADD COLUMN IF NOT EXISTS approved BOOLEAN NOT NULL DEFAULT true;

CREATE TABLE IF NOT EXISTS learner_reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    booking_id UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    learner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'reserved',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, booking_id, learner_user_id)
);

CREATE INDEX IF NOT EXISTS idx_learner_reservations_tenant_user
    ON learner_reservations (tenant_id, learner_user_id, created_at DESC);

COMMIT;
