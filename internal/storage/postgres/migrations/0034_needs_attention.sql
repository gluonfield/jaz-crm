-- +goose Up
WITH previous AS (
  SELECT f.id, f.filters FROM saved_filters f JOIN objects o ON o.id = f.object_id
  WHERE o.slug = 'follow_ups' AND lower(f.name) = 'needs attention' AND f.query = ''
    AND f.filters = '[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"action_date","operator":"on_or_before","value":"today"}]'::jsonb
), updated AS (
  UPDATE saved_filters f SET filters = '[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is","value":"Us"}]'::jsonb
  FROM previous WHERE f.id = previous.id RETURNING f.id, f.filters
)
UPDATE active_filters a SET filters = updated.filters
FROM previous JOIN updated ON updated.id = previous.id
WHERE a.saved_id = previous.id AND a.filters = previous.filters;
