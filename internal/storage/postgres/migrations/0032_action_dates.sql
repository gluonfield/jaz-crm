-- +goose Up
ALTER TABLE workspaces ADD COLUMN timezone text NOT NULL DEFAULT 'UTC';
ALTER TABLE attributes DROP CONSTRAINT attributes_type_check;
ALTER TABLE attributes ADD CONSTRAINT attributes_type_check
  CHECK (type IN ('text', 'number', 'date', 'datetime', 'checkbox', 'url', 'select', 'status', 'member', 'email', 'domain', 'phone', 'reference', 'markdown'));

UPDATE attributes SET slug = 'action_date', name = 'Action date', type = 'datetime'
FROM objects WHERE attributes.object_id = objects.id AND objects.slug = 'follow_ups' AND attributes.slug = 'review_on';

INSERT INTO attributes (object_id, slug, name, type, options)
SELECT objects.id, additions.slug, additions.name, additions.type, additions.options
FROM objects CROSS JOIN (VALUES
  ('action_date_basis', 'Date basis', 'select', ARRAY['Stated', 'Suggested', 'Manual']::text[]),
  ('action_date_reason', 'Date reason', 'text', ARRAY[]::text[]),
  ('action_date_source', 'Date source', 'text', ARRAY[]::text[])
) AS additions(slug, name, type, options)
WHERE objects.slug = 'follow_ups' ON CONFLICT (object_id, slug) DO NOTHING;

INSERT INTO record_values (record_id, attribute_id, text, source)
SELECT v.record_id, basis.id, CASE WHEN v.source = 'user' THEN 'Manual' ELSE 'Suggested' END, v.source
FROM record_values v JOIN attributes a ON a.id = v.attribute_id
JOIN attributes basis ON basis.object_id = a.object_id AND basis.slug = 'action_date_basis'
WHERE a.slug = 'action_date' AND v.active_until IS NULL;

INSERT INTO record_values (record_id, attribute_id, text, source)
SELECT v.record_id, reason.id, 'Preserved from the previous review date; its original scheduling reason was not recorded.', 'agent'
FROM record_values v JOIN attributes a ON a.id = v.attribute_id
JOIN attributes reason ON reason.object_id = a.object_id AND reason.slug = 'action_date_reason'
WHERE a.slug = 'action_date' AND v.active_until IS NULL AND v.source <> 'user';

UPDATE record_values SET active_until = clock_timestamp()
FROM attributes a JOIN objects o ON o.id = a.object_id
WHERE record_values.attribute_id = a.id AND record_values.active_until IS NULL
  AND o.slug = 'follow_ups' AND a.slug IN ('action_date', 'action_date_basis', 'action_date_reason', 'action_date_source')
  AND EXISTS (SELECT 1 FROM record_values status JOIN attributes sa ON sa.id = status.attribute_id
    WHERE status.record_id = record_values.record_id AND status.active_until IS NULL
      AND sa.slug = 'status' AND status.text IN ('Done', 'Dismissed'));

WITH stale AS (
  SELECT draft.record_id FROM record_values draft JOIN attributes a ON a.id = draft.attribute_id
  JOIN objects o ON o.id = a.object_id
  WHERE o.slug = 'follow_ups' AND a.slug = 'draft' AND draft.source = 'agent' AND draft.active_until IS NULL
    AND EXISTS (SELECT 1 FROM record_values v JOIN attributes va ON va.id = v.attribute_id
      WHERE v.record_id = draft.record_id AND v.active_until IS NULL
        AND (va.slug = 'waiting_on' AND v.text = 'Them' OR va.slug = 'status' AND v.text IN ('Done', 'Dismissed')))
    AND NOT EXISTS (SELECT 1 FROM record_values v JOIN attributes va ON va.id = v.attribute_id
      WHERE v.record_id = draft.record_id AND v.active_until IS NULL AND va.slug = 'draft_status' AND v.text = 'Sending')
)
UPDATE record_values SET active_until = clock_timestamp()
FROM attributes a WHERE record_values.attribute_id = a.id AND record_values.active_until IS NULL
  AND a.slug IN ('draft', 'draft_status') AND record_values.record_id IN (SELECT record_id FROM stale);

UPDATE saved_filters SET filters = (
  SELECT coalesce(jsonb_agg(CASE WHEN item->>'attribute' = 'review_on'
    THEN jsonb_set(item, '{attribute}', '"action_date"') ELSE item END), '[]')
  FROM jsonb_array_elements(saved_filters.filters) AS item
) FROM objects WHERE saved_filters.object_id = objects.id AND objects.slug = 'follow_ups';

UPDATE active_filters SET filters = (
  SELECT coalesce(jsonb_agg(CASE WHEN item->>'attribute' = 'review_on'
    THEN jsonb_set(item, '{attribute}', '"action_date"') ELSE item END), '[]')
  FROM jsonb_array_elements(active_filters.filters) AS item
) FROM objects WHERE active_filters.object_id = objects.id AND objects.slug = 'follow_ups';

-- A date-only deadline expires at the next local midnight, including on DST changes.
-- +goose StatementBegin
CREATE FUNCTION record_instant(value text, zone text) RETURNS timestamptz LANGUAGE sql STABLE AS $$
  SELECT CASE
    WHEN value = 'now' THEN CURRENT_TIMESTAMP
    WHEN value = 'today' THEN (date_trunc('day', CURRENT_TIMESTAMP AT TIME ZONE zone) + interval '1 day') AT TIME ZONE zone
    WHEN length(value) = 10 THEN (value::date + 1)::timestamp AT TIME ZONE zone
    ELSE value::timestamptz
  END
$$;
-- +goose StatementEnd
