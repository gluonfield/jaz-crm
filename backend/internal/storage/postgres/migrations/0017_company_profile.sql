-- +goose Up
INSERT INTO attributes (object_id, slug, name, type, options)
SELECT objects.id, fields.slug, fields.name, fields.type, fields.options
FROM objects
CROSS JOIN (VALUES
  ('founded_year', 'Founded year', 'number', ARRAY[]::text[]),
  ('size', 'Size', 'select', ARRAY['1-10', '11-50', '51-200', '201-500', '501-1,000', '1,001-5,000', '5,001-10,000', '10,001+'])
) AS fields(slug, name, type, options)
WHERE objects.slug = 'companies'
ON CONFLICT (object_id, slug) DO NOTHING;
