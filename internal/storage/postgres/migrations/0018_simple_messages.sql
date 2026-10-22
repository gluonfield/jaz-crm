-- +goose Up
ALTER TABLE interactions DROP CONSTRAINT interactions_kind_check;
ALTER TABLE interactions ADD CONSTRAINT interactions_kind_check CHECK (kind IN ('email', 'message', 'meeting', 'call', 'note'));
ALTER TABLE interactions ADD COLUMN channel text NOT NULL DEFAULT '';
ALTER TABLE interactions ADD COLUMN provenance text NOT NULL DEFAULT '';
ALTER TABLE interactions ADD COLUMN date_only boolean NOT NULL DEFAULT false;
ALTER TABLE parts ADD COLUMN recipients text[] NOT NULL DEFAULT '{}';
ALTER TABLE parts ADD COLUMN direction text NOT NULL DEFAULT '' CHECK (direction IN ('', 'sent', 'received'));
ALTER TABLE parts ADD COLUMN date_only boolean NOT NULL DEFAULT false;
ALTER TABLE parts ADD COLUMN partial boolean NOT NULL DEFAULT false;

-- Recover the fields from the legacy LinkedIn preview import format.
CREATE TEMP TABLE linkedin_imports ON COMMIT DROP AS
SELECT i.id, p.id AS part_id,
  split_part(split_part(p.content, E'Sender: ', 2), E'\n', 1) AS sender,
  split_part(split_part(p.content, E'Recipient: ', 2), E'\n', 1) AS recipient,
  substring(p.content FROM 'Message date displayed by LinkedIn: ([0-9]{4}-[0-9]{2}-[0-9]{2})') AS day,
  split_part(split_part(p.content, E'Verified partial message (verbatim preview, ending at capture cutoff):\n', 2), E'\n\nSource: ', 1) AS body,
  (SELECT string_agg(original.content, E'\n\n' ORDER BY original.id) FROM parts original WHERE original.interaction_id = i.id) AS provenance
FROM interactions i JOIN parts p ON p.interaction_id = i.id
WHERE i.kind = 'note' AND i.source = 'manual' AND i.title LIKE 'LinkedIn message excerpt%'
  AND p.kind = 'transcript' AND p.content LIKE E'Channel: LinkedIn\nSender: %'
  AND p.content LIKE E'%Verified partial message (verbatim preview, ending at capture cutoff):\n%'
  AND p.content LIKE '%This is a truncated incoming-message preview%'
  AND p.content ~ 'Message date displayed by LinkedIn: [0-9]{4}-[0-9]{2}-[0-9]{2};';
UPDATE interactions i SET kind = 'message', channel = 'linkedin', title = 'LinkedIn message',
  started_at = (legacy.day || 'T00:00:00Z')::timestamptz, date_only = true, provenance = legacy.provenance
FROM linkedin_imports legacy WHERE i.id = legacy.id AND legacy.body <> '' AND legacy.sender <> '' AND legacy.recipient <> '';
UPDATE parts p SET kind = 'message', content = legacy.body, author_name = legacy.sender,
  recipients = ARRAY[legacy.recipient], direction = 'received', date_only = true, partial = true,
  at = (legacy.day || 'T00:00:00Z')::timestamptz
FROM linkedin_imports legacy JOIN interactions i ON i.id = legacy.id AND i.kind = 'message'
WHERE p.id = legacy.part_id;
DELETE FROM parts p USING linkedin_imports legacy, interactions i
WHERE p.interaction_id = legacy.id AND i.id = legacy.id AND i.kind = 'message' AND p.id <> legacy.part_id;

UPDATE interactions i SET provenance = split_part(p.content, E'\nSources: ', 2)
FROM parts p WHERE p.interaction_id = i.id AND i.kind = 'note' AND i.source = 'manual'
  AND i.title IN ('Robotics manufacturing expertise', 'Manufacturing network') AND p.content LIKE E'%\nSources: %';
UPDATE parts p SET content = split_part(p.content, E'\nSources: ', 1), author_name = 'Imported'
FROM interactions i WHERE p.interaction_id = i.id AND i.kind = 'note' AND i.provenance <> ''
  AND i.title IN ('Robotics manufacturing expertise', 'Manufacturing network');

-- Legacy notes become one body; speaker text is preserved in that body.
WITH bodies AS (
  SELECT i.id, string_agg(CASE WHEN p.kind = 'transcript' AND p.author_name <> '' THEN p.author_name || ': ' || p.content ELSE p.content END, E'\n\n' ORDER BY p.at, p.id) AS content
  FROM interactions i JOIN parts p ON p.interaction_id = i.id
  WHERE i.kind = 'note' AND p.content IS NOT NULL
  GROUP BY i.id
)
UPDATE parts p SET content = bodies.content, kind = 'note'
FROM bodies
WHERE p.interaction_id = bodies.id AND p.id = (SELECT min(first.id) FROM parts first WHERE first.interaction_id = bodies.id);
DELETE FROM parts p USING interactions i
WHERE i.id = p.interaction_id AND i.kind = 'note'
  AND p.id <> (SELECT min(first.id) FROM parts first WHERE first.interaction_id = i.id);
UPDATE parts p SET author_name = coalesce(nullif(u.name, ''), u.email)
FROM interactions i JOIN users u ON u.id = i.user_id
WHERE p.interaction_id = i.id AND p.kind = 'note' AND p.author_name = '';
