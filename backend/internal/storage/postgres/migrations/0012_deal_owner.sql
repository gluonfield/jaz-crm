-- +goose Up
-- A member attribute names a member of the workspace by email.
ALTER TABLE attributes DROP CONSTRAINT attributes_type_check;
ALTER TABLE attributes ADD CONSTRAINT attributes_type_check
  CHECK (type IN ('text', 'number', 'date', 'checkbox', 'url', 'select', 'status', 'member', 'email', 'domain', 'phone', 'reference'));

INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'owner', 'Owner', 'member' FROM objects WHERE slug = 'deals'
ON CONFLICT (object_id, slug) DO NOTHING;
