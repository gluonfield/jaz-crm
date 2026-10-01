-- +goose Up
INSERT INTO attributes (object_id, slug, name, type, multi)
SELECT id, 'tags', 'Tags', 'select', true FROM objects WHERE slug = 'people'
ON CONFLICT (object_id, slug) DO NOTHING;

CREATE TABLE saved_filters (
  id text PRIMARY KEY,
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  object_id uuid NOT NULL REFERENCES objects(id) ON DELETE CASCADE,
  name text NOT NULL,
  query text NOT NULL DEFAULT '',
  filters jsonb NOT NULL DEFAULT '[]'
);
CREATE UNIQUE INDEX saved_filters_name ON saved_filters (workspace_id, object_id, lower(name));
