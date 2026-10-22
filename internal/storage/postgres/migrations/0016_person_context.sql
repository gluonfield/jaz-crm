-- +goose Up
INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'context', 'Context', 'text' FROM objects WHERE slug = 'people'
ON CONFLICT (object_id, slug) DO NOTHING;
