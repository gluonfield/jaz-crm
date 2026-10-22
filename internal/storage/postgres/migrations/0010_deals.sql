-- +goose Up
-- A status is a select whose options are ordered stages.
ALTER TABLE attributes DROP CONSTRAINT attributes_type_check;
ALTER TABLE attributes ADD CONSTRAINT attributes_type_check
  CHECK (type IN ('text', 'number', 'date', 'checkbox', 'url', 'select', 'status', 'email', 'domain', 'phone', 'reference'));

-- Objects list in the order they were created, including those a new
-- workspace creates in one transaction.
ALTER TABLE objects ALTER COLUMN created_at SET DEFAULT clock_timestamp();

-- Existing workspaces get the standard deals object (records.StandardObjects)
-- unless they made their own; one statement per attribute keeps their order.
CREATE TEMP TABLE seeded ON COMMIT DROP AS
SELECT gen_random_uuid() AS id, workspaces.id AS workspace_id FROM workspaces
WHERE NOT EXISTS (SELECT 1 FROM objects WHERE objects.workspace_id = workspaces.id AND objects.slug = 'deals');

INSERT INTO objects (id, workspace_id, slug, name) SELECT id, workspace_id, 'deals', 'Deals' FROM seeded;
INSERT INTO attributes (object_id, slug, name, type) SELECT id, 'name', 'Name', 'text' FROM seeded;
INSERT INTO attributes (object_id, slug, name, type, options) SELECT id, 'stage', 'Stage', 'status', ARRAY['Lead', 'In progress', 'Won', 'Lost'] FROM seeded;
INSERT INTO attributes (object_id, slug, name, type) SELECT id, 'value', 'Value', 'number' FROM seeded;
INSERT INTO attributes (object_id, slug, name, type, target_object_id)
SELECT seeded.id, 'company', 'Company', 'reference', objects.id FROM seeded
JOIN objects ON objects.workspace_id = seeded.workspace_id AND objects.slug = 'companies';
INSERT INTO attributes (object_id, slug, name, type, multi, target_object_id)
SELECT seeded.id, 'people', 'People', 'reference', true, objects.id FROM seeded
JOIN objects ON objects.workspace_id = seeded.workspace_id AND objects.slug = 'people';
