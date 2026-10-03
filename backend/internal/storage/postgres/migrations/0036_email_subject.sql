-- +goose Up
INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'subject', 'Subject', 'text' FROM objects WHERE slug = 'follow_ups'
ON CONFLICT (object_id, slug) DO NOTHING;

-- Preserve the original draft in history when separating an explicit subject line.
WITH legacy AS (
  SELECT v.*, subject.id AS subject_id,
    regexp_match(v.text, '^Subject:[ \t]*([^\r\n]+)\r?\n[ \t]*\r?\n(.+)$', 'is') AS parts
  FROM record_values v
  JOIN attributes a ON a.id = v.attribute_id AND a.slug = 'draft'
  JOIN objects o ON o.id = a.object_id AND o.slug = 'follow_ups'
  JOIN attributes subject ON subject.object_id = o.id AND subject.slug = 'subject'
  WHERE v.active_until IS NULL
    AND NOT EXISTS (SELECT 1 FROM record_values s WHERE s.record_id = v.record_id AND s.attribute_id = subject.id AND s.active_until IS NULL)
    AND EXISTS (SELECT 1 FROM record_values c JOIN attributes ca ON ca.id = c.attribute_id
      WHERE c.record_id = v.record_id AND c.active_until IS NULL AND ca.slug = 'channel' AND c.text = 'Email')
    AND NOT EXISTS (SELECT 1 FROM record_values s JOIN attributes sa ON sa.id = s.attribute_id
      WHERE s.record_id = v.record_id AND s.active_until IS NULL AND sa.slug = 'draft_status' AND s.text IN ('Sending', 'Sent'))
), closed AS (
  UPDATE record_values SET active_until = now()
  WHERE id IN (SELECT id FROM legacy WHERE parts IS NOT NULL)
  RETURNING id
)
INSERT INTO record_values (record_id, attribute_id, text, source, actor_id, active_from)
SELECT record_id, subject_id, btrim(parts[1]), source, actor_id, active_from FROM legacy JOIN closed USING (id)
UNION ALL
SELECT record_id, attribute_id, parts[2], source, actor_id, active_from FROM legacy JOIN closed USING (id);

UPDATE record_values SET active_until = now()
FROM attributes a JOIN objects o ON o.id = a.object_id
WHERE record_values.attribute_id = a.id AND record_values.active_until IS NULL
  AND o.slug = 'follow_ups' AND a.slug = 'draft_status' AND record_values.text = 'Approved'
  AND EXISTS (SELECT 1 FROM record_values c JOIN attributes ca ON ca.id = c.attribute_id
    WHERE c.record_id = record_values.record_id AND c.active_until IS NULL AND ca.slug = 'channel' AND c.text = 'Email');

INSERT INTO record_values (record_id, attribute_id, text, source)
SELECT r.id, a.id, 'Draft', 'agent' FROM records r
JOIN objects o ON o.id = r.object_id AND o.slug = 'follow_ups'
JOIN attributes a ON a.object_id = o.id AND a.slug = 'draft_status'
WHERE EXISTS (SELECT 1 FROM record_values d JOIN attributes da ON da.id = d.attribute_id
  WHERE d.record_id = r.id AND d.active_until IS NULL AND da.slug = 'draft')
AND NOT EXISTS (SELECT 1 FROM record_values s WHERE s.record_id = r.id AND s.attribute_id = a.id AND s.active_until IS NULL);
