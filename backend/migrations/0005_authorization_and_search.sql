BEGIN;

ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS checked_in_at TIMESTAMPTZ NULL;

ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS created_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_bookings_tenant_user ON bookings (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_content_items_tenant_creator ON content_items (tenant_id, created_by_user_id);

ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS search_vector tsvector;

CREATE OR REPLACE FUNCTION content_items_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', coalesce(NEW.title, '') || ' ' || coalesce(NEW.summary, ''));
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS content_items_search_vector_trigger ON content_items;
CREATE TRIGGER content_items_search_vector_trigger
BEFORE INSERT OR UPDATE ON content_items
FOR EACH ROW EXECUTE FUNCTION content_items_search_vector_update();

UPDATE content_items
SET search_vector = to_tsvector('english', coalesce(title, '') || ' ' || coalesce(summary, ''));

CREATE INDEX IF NOT EXISTS idx_content_items_search_vector ON content_items USING GIN (search_vector);

COMMIT;
