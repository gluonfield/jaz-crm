-- +goose Up
ALTER TABLE parts ALTER COLUMN at DROP NOT NULL;
ALTER TABLE parts ADD CONSTRAINT parts_at_check CHECK (kind = 'transcript' OR at IS NOT NULL);
ALTER TABLE parts ADD COLUMN position integer NOT NULL DEFAULT 0;

WITH turns AS (
  SELECT p.id, row_number() OVER (PARTITION BY p.interaction_id ORDER BY p.id) AS position
  FROM parts p JOIN interactions i ON i.id = p.interaction_id
  WHERE p.kind = 'transcript' AND i.source IN ('manual', 'webhook')
)
UPDATE parts SET position = turns.position FROM turns WHERE parts.id = turns.id;
