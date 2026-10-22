-- +goose Up
-- A table's position orders it among the pages and tables beside it, as a
-- page's does; a new table goes last.
ALTER TABLE objects ADD COLUMN position double precision NOT NULL DEFAULT extract(epoch FROM now()) * 1000;

-- Tables were listed above every page, in the order they were made; they keep
-- that place below the workspace's lowest page position.
WITH pages AS (
  SELECT records.workspace_id, coalesce(position.text::double precision, extract(epoch FROM records.created_at) * 1000) AS position
  FROM records
  JOIN objects ON objects.id = records.object_id AND objects.slug = 'pages'
  LEFT JOIN attributes ON attributes.object_id = objects.id AND attributes.slug = 'position'
  LEFT JOIN record_values position ON position.record_id = records.id AND position.attribute_id = attributes.id AND position.active_until IS NULL
), firsts AS (
  SELECT workspace_id, min(position) AS first FROM pages GROUP BY workspace_id
)
UPDATE objects SET position = ranked.position FROM (
  SELECT objects.id, coalesce(firsts.first, objects.position) - 1000 * row_number() OVER (PARTITION BY objects.workspace_id ORDER BY objects.created_at DESC, objects.slug DESC) AS position
  FROM objects LEFT JOIN firsts ON firsts.workspace_id = objects.workspace_id
) ranked
WHERE objects.id = ranked.id;
