-- name: UpsertHandle :one
INSERT INTO handles (workspace_id, kind, value, name, triage, decided_by, reason)
SELECT @workspace_id, @kind, @value, @name,
  coalesce(rule.triage, @triage)::text,
  CASE WHEN rule.domain IS NOT NULL THEN 'user' ELSE sqlc.narg(decided_by)::text END,
  coalesce(rule.reason, @reason)::text
FROM (SELECT 1) AS input
LEFT JOIN domain_rules rule ON rule.workspace_id = @workspace_id AND rule.domain = split_part(@value, '@', 2)
  AND @kind::text = 'email' AND @triage::text <> 'internal'
ON CONFLICT (workspace_id, kind, value) DO UPDATE
SET name = CASE WHEN handles.name = '' THEN EXCLUDED.name ELSE handles.name END
RETURNING *;

-- name: SkipPendingHandle :exec
UPDATE handles SET triage = 'skipped', decided_by = 'rule', reason = $2 WHERE id = $1 AND triage = 'pending';

-- name: SetTriage :execrows
UPDATE handles SET triage = $3, decided_by = $4, reason = $5, person_id = $6
WHERE workspace_id = $1 AND id = $2;

-- name: GetHandles :many
SELECT * FROM handles WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: HandlesByValue :many
SELECT * FROM handles WHERE workspace_id = @workspace_id AND value = ANY(@handle_values::text[]);

-- name: HandlesByDomain :many
SELECT * FROM handles WHERE workspace_id = $1 AND kind = 'email' AND split_part(value, '@', 2) = $2;

-- name: ListHandles :many
SELECT sqlc.embed(handles), count(DISTINCT participants.interaction_id)::int AS interactions, coalesce(max(interactions.started_at), handles.created_at)::timestamptz AS last_seen
FROM handles
LEFT JOIN participants ON participants.handle_id = handles.id
LEFT JOIN interactions ON interactions.id = participants.interaction_id
WHERE handles.workspace_id = @workspace_id AND handles.triage = @triage
  AND (sqlc.narg(query)::text IS NULL OR handles.value ILIKE '%' || sqlc.narg(query)::text || '%' OR handles.name ILIKE '%' || sqlc.narg(query)::text || '%')
GROUP BY handles.id
ORDER BY max(interactions.started_at) DESC NULLS LAST, handles.id
LIMIT @row_limit;

-- name: SetPhotos :exec
-- SetPhotos records the profile pictures of a workspace's email addresses.
UPDATE handles SET photo_url = photos.url
FROM (
  SELECT (@addresses::text[])[i] AS address, (@urls::text[])[i] AS url FROM generate_subscripts(@addresses::text[], 1) AS i
) AS photos
WHERE handles.workspace_id = @workspace_id AND handles.kind = 'email' AND handles.value = photos.address AND handles.photo_url <> photos.url;

-- name: PersonPhotos :many
-- PersonPhotos picks a profile picture for each person from their addresses.
SELECT DISTINCT ON (person_id) person_id::text AS person_id, photo_url FROM handles
WHERE workspace_id = @workspace_id AND person_id = ANY(@person_ids::uuid[]) AND photo_url <> ''
ORDER BY person_id, created_at;

-- name: MarkInternal :exec
-- MarkInternal files the workspace's own addresses and domains as internal,
-- unless a person decided otherwise.
UPDATE handles SET triage = 'internal', decided_by = NULL, reason = ''
WHERE workspace_id = @workspace_id AND kind = 'email'
  AND (triage = 'pending' OR (triage = 'skipped' AND decided_by IN ('rule', 'agent')))
  AND (value = ANY(@addresses::text[]) OR split_part(value, '@', 2) = ANY(@domains::text[]));

-- name: EngagedHandles :many
-- EngagedHandles are undecided or heuristically skipped handles someone in the
-- workspace wrote to, or met with, in a conversation of at most max_size
-- participants.
SELECT DISTINCT handles.id FROM handles
JOIN participants ON participants.handle_id = handles.id AND participants.role <> 'declined'
JOIN interactions ON interactions.id = participants.interaction_id AND NOT interactions.skipped
WHERE handles.workspace_id = @workspace_id
  AND (handles.triage = 'pending' OR (handles.triage = 'skipped' AND handles.decided_by IN ('rule', 'agent')))
  AND (SELECT count(DISTINCT everyone.handle_id) FROM participants everyone WHERE everyone.interaction_id = interactions.id) <= @max_size::int
  AND EXISTS (
    SELECT 1 FROM participants own
    JOIN handles internal ON internal.id = own.handle_id AND internal.triage = 'internal'
    WHERE own.interaction_id = interactions.id AND ((@auto_keep_email::boolean AND interactions.kind = 'email' AND own.role = 'from')
      OR (@auto_keep_meetings::boolean AND interactions.kind = 'meeting' AND interactions.ended_at <= now() AND own.role IN ('organizer', 'attendee')))
  );

-- name: HandlesOnRecords :many
-- HandlesOnRecords pairs undecided handles, and kept ones that lost their
-- person, with the record that holds their address.
SELECT DISTINCT ON (handles.id) handles.id, record_values.record_id FROM handles
JOIN record_values ON record_values.unique_key = handles.value AND record_values.active_until IS NULL
JOIN attributes ON attributes.id = record_values.attribute_id AND attributes.type IN ('email', 'phone')
JOIN records ON records.id = record_values.record_id AND records.workspace_id = handles.workspace_id
WHERE handles.workspace_id = @workspace_id
  AND (handles.triage IN ('pending', 'kept') AND handles.person_id IS NULL
    OR handles.triage = 'skipped' AND handles.decided_by IN ('rule', 'agent'));

-- name: SkipRecordHandles :many
WITH domains AS (
    SELECT DISTINCT record_values.text AS domain FROM record_values
    JOIN attributes ON attributes.id = record_values.attribute_id AND attributes.type = 'domain'
    JOIN records ON records.id = record_values.record_id AND records.workspace_id = @workspace_id
    JOIN objects ON objects.id = records.object_id AND objects.slug = 'companies'
    WHERE records.id = @record_id AND record_values.active_until IS NULL
), blocked AS (
    INSERT INTO domain_rules (workspace_id, domain, triage, reason)
    SELECT @workspace_id, domain, 'skipped', 'company deleted' FROM domains
    ON CONFLICT ON CONSTRAINT domain_rules_pkey DO UPDATE SET triage = 'skipped', reason = EXCLUDED.reason
)
UPDATE handles SET triage = 'skipped', decided_by = 'user', reason = 'record deleted'
WHERE handles.workspace_id = @workspace_id AND handles.triage <> 'internal'
    AND (handles.person_id = @record_id OR (handles.kind = 'email' AND split_part(handles.value, '@', 2) IN (SELECT domain FROM domains)))
RETURNING id;

-- name: KeptWithoutPerson :many
SELECT * FROM handles WHERE workspace_id = $1 AND triage = 'kept' AND person_id IS NULL AND decided_by = 'user';

-- name: UnassessedHandles :many
-- UnassessedHandles are pending handles no agent has judged, with the titles
-- of their latest conversations.
SELECT handles.id, handles.value, handles.name,
  array(SELECT interactions.title FROM participants JOIN interactions ON interactions.id = participants.interaction_id
    WHERE participants.handle_id = handles.id ORDER BY interactions.started_at DESC LIMIT 3)::text[] AS titles
FROM handles
WHERE handles.workspace_id = $1 AND handles.triage = 'pending' AND handles.decided_by IS NULL
ORDER BY handles.created_at
LIMIT $2;

-- name: EmailInteractionByMessageIDs :one
SELECT interactions.id FROM parts
JOIN interactions ON interactions.id = parts.interaction_id
WHERE interactions.workspace_id = @workspace_id AND interactions.kind = 'email' AND parts.external_id = ANY(@message_ids::text[])
LIMIT 1;

-- name: UpsertEmailThread :one
INSERT INTO interactions (workspace_id, kind, source, external_id, connection_id, user_id, title, started_at, ended_at)
VALUES (@workspace_id, 'email', 'gmail', @external_id, @connection_id, @user_id, @title, @at, @at)
ON CONFLICT (workspace_id, source, external_id) DO UPDATE
SET started_at = LEAST(interactions.started_at, EXCLUDED.started_at),
    ended_at = GREATEST(interactions.ended_at, EXCLUDED.ended_at),
    title = CASE WHEN interactions.title = '' THEN EXCLUDED.title ELSE interactions.title END
RETURNING id;

-- name: ExtendEmailThread :exec
UPDATE interactions
SET started_at = LEAST(started_at, @at), ended_at = GREATEST(ended_at, @at),
    title = CASE WHEN title = '' THEN @title ELSE title END
WHERE id = @id;

-- name: UpsertInteraction :one
INSERT INTO interactions (workspace_id, kind, source, external_id, connection_id, user_id, title, started_at, ended_at, meet_code, skipped)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (workspace_id, source, external_id) DO UPDATE
SET title = EXCLUDED.title, started_at = EXCLUDED.started_at, ended_at = EXCLUDED.ended_at,
    meet_code = EXCLUDED.meet_code, skipped = interactions.skipped OR EXCLUDED.skipped
RETURNING *;

-- name: ClearParticipants :exec
DELETE FROM participants WHERE interaction_id = $1;

-- name: AddParticipant :exec
INSERT INTO participants (interaction_id, handle_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: UpsertPart :exec
INSERT INTO parts (interaction_id, kind, external_id, connection_id, provider_id, author_handle_id, author_name, at, content)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (interaction_id, external_id) DO UPDATE
SET content = COALESCE(EXCLUDED.content, parts.content), author_name = EXCLUDED.author_name, at = EXCLUDED.at;

-- name: InteractionsOfHandles :many
SELECT DISTINCT interaction_id FROM participants WHERE handle_id = ANY(@handle_ids::uuid[]);

-- name: DeleteSyncLinks :exec
DELETE FROM links WHERE source = 'sync' AND interaction_id = ANY(@ids::uuid[]);

-- name: InsertSyncLinks :exec
-- InsertSyncLinks links interactions to the people behind their kept
-- participants and to those people's companies.
WITH people AS (
  SELECT DISTINCT participants.interaction_id, handles.person_id AS record_id FROM participants
  JOIN handles ON handles.id = participants.handle_id AND handles.triage = 'kept' AND handles.person_id IS NOT NULL
  JOIN interactions ON interactions.id = participants.interaction_id AND NOT interactions.skipped
  WHERE participants.interaction_id = ANY(@ids::uuid[])
), companies AS (
  SELECT DISTINCT people.interaction_id, record_values.ref_record_id AS record_id FROM people
  JOIN record_values ON record_values.record_id = people.record_id AND record_values.active_until IS NULL AND record_values.ref_record_id IS NOT NULL
  JOIN attributes ON attributes.id = record_values.attribute_id AND attributes.slug = 'company'
)
INSERT INTO links (interaction_id, record_id, source)
SELECT interaction_id, record_id, 'sync' FROM people
UNION
SELECT interaction_id, record_id, 'sync' FROM companies
ON CONFLICT (interaction_id, record_id) DO NOTHING;

-- name: ClearUnlinkedContent :exec
-- ClearUnlinkedContent forgets provider content of interactions no record links.
UPDATE parts SET content = NULL
WHERE provider_id IS NOT NULL AND content IS NOT NULL AND interaction_id = ANY(@ids::uuid[])
  AND NOT EXISTS (SELECT 1 FROM links WHERE links.interaction_id = parts.interaction_id);

-- name: AddLink :exec
INSERT INTO links (interaction_id, record_id, source) VALUES ($1, $2, $3)
ON CONFLICT (interaction_id, record_id) DO UPDATE SET source = EXCLUDED.source;

-- name: DeleteLink :execrows
DELETE FROM links WHERE interaction_id = $1 AND record_id = $2;

-- name: SkipInteraction :execrows
UPDATE interactions SET skipped = true WHERE workspace_id = $1 AND id = $2;

-- name: UnfetchedParts :many
SELECT parts.id, parts.provider_id::text AS provider_id FROM parts
JOIN interactions ON interactions.id = parts.interaction_id AND NOT interactions.skipped
WHERE parts.connection_id = $1 AND parts.content IS NULL AND parts.provider_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM links WHERE links.interaction_id = interactions.id)
ORDER BY parts.at DESC
LIMIT $2;

-- name: SetPartContent :exec
UPDATE parts SET content = $2 WHERE id = $1;

-- name: GetInteractions :many
SELECT * FROM interactions WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: Timeline :many
-- Timeline lists a record's interactions that started by now, newest first,
-- or the upcoming ones, soonest first.
SELECT interactions.* FROM interactions
JOIN links ON links.interaction_id = interactions.id AND links.record_id = @record_id
WHERE interactions.workspace_id = @workspace_id AND NOT interactions.skipped
  AND (cardinality(@kinds::text[]) = 0 OR interactions.kind = ANY(@kinds::text[]))
  AND (sqlc.narg(before)::timestamptz IS NULL OR interactions.started_at < sqlc.narg(before)::timestamptz)
  AND (interactions.started_at > now()) = @upcoming::bool
ORDER BY CASE WHEN @upcoming::bool THEN interactions.started_at END, interactions.started_at DESC, interactions.id
LIMIT @row_limit;

-- name: SearchInteractions :many
SELECT interactions.* FROM interactions
WHERE interactions.workspace_id = @workspace_id AND NOT interactions.skipped
  AND EXISTS (SELECT 1 FROM links WHERE links.interaction_id = interactions.id)
  AND (@query::text = '' OR interactions.title ILIKE '%' || @query::text || '%' OR EXISTS (
    SELECT 1 FROM parts WHERE parts.interaction_id = interactions.id AND parts.search @@ websearch_to_tsquery('simple', @query::text)
  ))
ORDER BY interactions.started_at DESC, interactions.id
LIMIT @row_limit;

-- name: InteractionParticipants :many
SELECT participants.interaction_id, participants.role, sqlc.embed(handles) FROM participants
JOIN handles ON handles.id = participants.handle_id
WHERE participants.interaction_id = ANY(@ids::uuid[])
ORDER BY participants.interaction_id, handles.value;

-- name: InteractionParts :many
SELECT id, interaction_id, kind, external_id, author_handle_id, author_name, at, content FROM parts
WHERE interaction_id = ANY(@ids::uuid[]) ORDER BY interaction_id, at, id;

-- name: InteractionLinks :many
SELECT links.interaction_id, links.record_id, links.source FROM links
WHERE links.interaction_id = ANY(@ids::uuid[])
ORDER BY links.interaction_id, links.created_at;

-- name: RecordActivity :many
-- RecordActivity counts each record's interactions that started by now, with
-- when the first started and when the latest was last active.
SELECT links.record_id, count(*)::int AS interactions, min(interactions.started_at)::timestamptz AS first_at,
  max(least(coalesce(interactions.ended_at, interactions.started_at), now()))::timestamptz AS last_at
FROM links
JOIN interactions ON interactions.id = links.interaction_id AND NOT interactions.skipped
WHERE interactions.workspace_id = @workspace_id AND links.record_id = ANY(@record_ids::uuid[]) AND interactions.started_at <= now()
GROUP BY links.record_id;

-- name: DeleteLinks :exec
DELETE FROM links WHERE interaction_id = ANY(@ids::uuid[]);

-- name: SetDomainRule :exec
INSERT INTO domain_rules (workspace_id, domain, triage, reason) VALUES ($1, $2, $3, $4)
ON CONFLICT (workspace_id, domain) DO UPDATE SET triage = EXCLUDED.triage, reason = EXCLUDED.reason;

-- name: DomainRules :many
SELECT * FROM domain_rules WHERE workspace_id = $1 ORDER BY domain;

-- name: DeleteDomainRule :execrows
DELETE FROM domain_rules WHERE workspace_id = $1 AND domain = $2;

-- name: InteractionByExternalID :one
SELECT id FROM interactions WHERE workspace_id = $1 AND source = $2 AND external_id = $3;

-- name: DueMeetings :many
-- DueMeetings are a connection's linked Meet meetings that ended in the last
-- 30 days and whose transcript was never checked.
SELECT * FROM interactions
WHERE connection_id = $1 AND kind = 'meeting' AND meet_code <> '' AND NOT skipped AND transcript_checked_at IS NULL
  AND ended_at BETWEEN now() - interval '30 days' AND now() - interval '10 minutes'
  AND EXISTS (SELECT 1 FROM links WHERE links.interaction_id = interactions.id)
ORDER BY ended_at DESC
LIMIT 20;

-- name: MarkTranscriptChecked :exec
UPDATE interactions SET transcript_checked_at = now() WHERE id = $1;
