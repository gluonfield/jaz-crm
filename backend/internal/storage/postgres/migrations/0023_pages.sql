-- +goose Up
-- Pages are documents nested by parent, and every record of a workspace's
-- own tables is a page too: each has markdown content.
ALTER TABLE attributes DROP CONSTRAINT attributes_type_check;
ALTER TABLE attributes ADD CONSTRAINT attributes_type_check
  CHECK (type IN ('text', 'number', 'date', 'checkbox', 'url', 'select', 'status', 'member', 'email', 'domain', 'phone', 'reference', 'markdown'));

INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'content', 'Content', 'markdown' FROM objects
WHERE slug NOT IN ('companies', 'people', 'deals', 'follow_ups', 'pages')
ON CONFLICT (object_id, slug) DO NOTHING;

INSERT INTO objects (workspace_id, slug, name)
SELECT id, 'pages', 'Pages' FROM workspaces
ON CONFLICT (workspace_id, slug) DO NOTHING;

INSERT INTO attributes (object_id, slug, name, type, target_object_id)
SELECT objects.id, fields.slug, fields.name, fields.type, CASE WHEN fields.type = 'reference' THEN objects.id END
FROM objects
CROSS JOIN (VALUES (1, 'name', 'Name', 'text'), (2, 'parent', 'Parent', 'reference'), (3, 'content', 'Content', 'markdown')) AS fields(position, slug, name, type)
WHERE objects.slug = 'pages'
ORDER BY objects.id, fields.position
ON CONFLICT (object_id, slug) DO NOTHING;
