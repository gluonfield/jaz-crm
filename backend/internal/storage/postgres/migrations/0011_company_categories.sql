-- +goose Up
INSERT INTO attributes (object_id, slug, name, type, multi)
SELECT id, 'categories', 'Categories', 'select', true
FROM objects WHERE slug = 'companies'
ON CONFLICT (object_id, slug) DO NOTHING;
