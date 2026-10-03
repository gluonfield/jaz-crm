-- name: CreateObject :one
INSERT INTO objects (workspace_id, slug, name) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateAttribute :one
INSERT INTO attributes (object_id, slug, name, type, multi, is_unique, target_object_id, options)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: StartRecords :exec
-- StartRecords gives every record of an object a value of a new attribute.
INSERT INTO record_values (record_id, attribute_id, text, source, actor_id)
SELECT records.id, @attribute_id, @text, @source, @actor_id FROM records WHERE records.object_id = @object_id;

-- name: AddAttributeOption :one
WITH updated AS (
  UPDATE attributes SET options = CASE
    WHEN EXISTS (SELECT 1 FROM unnest(options) AS option WHERE lower(option) = lower(@value::text)) THEN options
    ELSE array_append(options, @value::text)
  END
  FROM objects
  WHERE attributes.object_id = objects.id AND objects.workspace_id = @workspace_id
    AND attributes.id = @attribute_id AND attributes.type IN ('select', 'status')
  RETURNING attributes.options
)
SELECT option::text FROM updated, unnest(options) AS option WHERE lower(option) = lower(@value::text);

-- name: RenameObject :execrows
UPDATE objects SET name = @name WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteObject :execrows
DELETE FROM objects WHERE workspace_id = @workspace_id AND id = @id;

-- name: RenameAttribute :execrows
UPDATE attributes SET name = @name FROM objects
WHERE attributes.object_id = objects.id AND objects.workspace_id = @workspace_id AND attributes.id = @id;

-- name: DeleteAttribute :execrows
DELETE FROM attributes USING objects
WHERE attributes.object_id = objects.id AND objects.workspace_id = @workspace_id AND attributes.id = @id;

-- name: DropFilterConditions :exec
UPDATE saved_filters SET filters = (
  SELECT coalesce(jsonb_agg(condition ORDER BY position), '[]'::jsonb) FROM jsonb_array_elements(filters) WITH ORDINALITY AS conditions(condition, position)
  WHERE NOT EXISTS (
    SELECT 1 FROM attributes
    WHERE attributes.id = ANY(@attribute_ids::uuid[]) AND attributes.object_id = saved_filters.object_id AND attributes.slug = condition->>'attribute'
  )
)
WHERE saved_filters.workspace_id = @workspace_id AND saved_filters.object_id IN (SELECT object_id FROM attributes WHERE id = ANY(@attribute_ids::uuid[]));

-- name: ListObjects :many
SELECT * FROM objects WHERE workspace_id = $1 ORDER BY created_at, slug;

-- name: LockStatus :one
SELECT attributes.* FROM attributes JOIN objects ON objects.id = attributes.object_id
WHERE objects.workspace_id = @workspace_id AND attributes.id = @id AND attributes.type = 'status'
FOR UPDATE OF attributes;

-- name: LockObjectStatuses :many
SELECT attributes.* FROM attributes JOIN objects ON objects.id = attributes.object_id
WHERE objects.workspace_id = @workspace_id AND objects.id = @object_id AND attributes.type = 'status'
ORDER BY attributes.id FOR SHARE OF attributes;

-- name: StageInUse :one
SELECT EXISTS (SELECT 1 FROM record_values WHERE attribute_id = @attribute_id AND text = @text AND active_until IS NULL);

-- name: UpdateStatusOptions :exec
UPDATE attributes SET options = @options WHERE id = @id;

-- name: ReplaceStageValues :exec
WITH closed AS (
  UPDATE record_values SET active_until = clock_timestamp()
  WHERE record_values.attribute_id = @attribute_id AND record_values.text = @from_stage AND record_values.active_until IS NULL
  RETURNING record_values.record_id, record_values.attribute_id, record_values.source, record_values.active_until
)
INSERT INTO record_values (record_id, attribute_id, text, source, actor_id, active_from)
SELECT closed.record_id, closed.attribute_id, @to_stage, closed.source, @actor_id, closed.active_until FROM closed;

-- name: ListAttributes :many
SELECT attributes.* FROM attributes
JOIN objects ON objects.id = attributes.object_id
WHERE objects.workspace_id = $1
ORDER BY attributes.object_id, attributes.created_at, attributes.slug;

-- name: LockRecordForDeletion :one
SELECT * FROM records WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: DeleteRecord :execrows
DELETE FROM records WHERE workspace_id = $1 AND id = $2;

-- name: CreateRecord :one
INSERT INTO records (workspace_id, object_id) VALUES ($1, $2) RETURNING *;

-- name: LockRecord :one
SELECT * FROM records WHERE workspace_id = $1 AND object_id = $2 AND id = $3 FOR UPDATE;

-- name: GetRecords :many
SELECT * FROM records WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: CurrentValues :many
SELECT record_values.* FROM record_values
JOIN records ON records.id = record_values.record_id
WHERE records.workspace_id = @workspace_id AND record_values.record_id = ANY(@record_ids::uuid[])
  AND record_values.active_until IS NULL
ORDER BY record_values.id;

-- name: RecordHistory :many
-- RecordHistory lists values a record has had, newest first, with the name
-- of the member who set each.
SELECT sqlc.embed(record_values), coalesce(users.name, '')::text AS actor_name FROM record_values
JOIN records ON records.id = record_values.record_id
LEFT JOIN users ON users.id = record_values.actor_id
WHERE records.workspace_id = @workspace_id AND record_values.record_id = @record_id
ORDER BY record_values.active_from DESC, record_values.id DESC
LIMIT @row_limit;

-- name: RecordsByUniqueKeys :many
-- RecordsByUniqueKeys finds the records holding any of the (attribute,
-- unique key) pairs.
SELECT DISTINCT record_values.record_id FROM record_values
JOIN records ON records.id = record_values.record_id
WHERE records.workspace_id = @workspace_id AND record_values.active_until IS NULL
  AND (record_values.attribute_id, record_values.unique_key) IN (
    SELECT (@attribute_ids::uuid[])[i], (@unique_keys::text[])[i] FROM generate_subscripts(@attribute_ids::uuid[], 1) AS i
  );

-- name: SearchRecords :many
WITH scoped AS (
  SELECT records.*, workspaces.timezone, conversation.id AS conversation_id,
    (sqlc.narg(query)::text IS NULL OR EXISTS (
      SELECT 1 FROM record_values
      WHERE record_values.record_id = records.id AND record_values.active_until IS NULL
        AND record_values.text ILIKE '%' || sqlc.narg(query)::text || '%'
    )) AS matches_text
  FROM records
  JOIN workspaces ON workspaces.id = records.workspace_id
  LEFT JOIN LATERAL (
    SELECT min(interactions.id::text)::uuid AS id FROM links
    JOIN interactions ON interactions.id = links.interaction_id
    WHERE (@group_by_conversation::boolean OR sqlc.narg(conversation_id)::uuid IS NOT NULL) AND links.record_id = records.id
      AND interactions.workspace_id = @workspace_id AND NOT interactions.skipped
      AND interactions.kind IN ('email', 'message') AND interactions.started_at <= now()
    -- Actions spanning conversations stay separate instead of guessing a reply target.
    HAVING count(*) = 1
  ) conversation ON true
  WHERE records.workspace_id = @workspace_id AND records.object_id = @object_id
    AND (sqlc.narg(conversation_id)::uuid IS NULL OR conversation.id = sqlc.narg(conversation_id)::uuid)
), matched AS (
  SELECT scoped.*, CASE WHEN @group_by_conversation::boolean
    THEN bool_or(matches_text) OVER (PARTITION BY coalesce(conversation_id, id))
    ELSE matches_text END AS matches_query
  FROM scoped
), ranked AS (
  SELECT matched.id, matched.conversation_id,
    CASE WHEN @group_by_conversation::boolean THEN row_number() OVER (
      PARTITION BY coalesce(matched.conversation_id, matched.id)
      ORDER BY CASE state.status WHEN 'Done' THEN 1 WHEN 'Dismissed' THEN 2 ELSE 0 END,
        CASE WHEN state.status IS NULL OR state.status = 'Open' THEN CASE state.waiting_on WHEN 'Them' THEN 1 ELSE 0 END ELSE 0 END,
        CASE WHEN state.status IS NULL OR state.status = 'Open' THEN record_instant(state.action_date, matched.timezone) END NULLS LAST,
        matched.created_at DESC, matched.id
    ) ELSE 1 END AS position
  FROM matched
  LEFT JOIN LATERAL (
    SELECT max(record_values.text) FILTER (WHERE attributes.slug = 'status') AS status,
      max(record_values.text) FILTER (WHERE attributes.slug = 'waiting_on') AS waiting_on,
      max(record_values.text) FILTER (WHERE attributes.slug = 'action_date') AS action_date
    FROM record_values JOIN attributes ON attributes.id = record_values.attribute_id
    WHERE @group_by_conversation::boolean AND record_values.record_id = matched.id
      AND record_values.active_until IS NULL AND attributes.slug IN ('status', 'waiting_on', 'action_date')
  ) state ON true
  WHERE matched.matches_query
  AND NOT EXISTS (
    SELECT 1 FROM generate_subscripts(@attribute_ids::uuid[], 1) AS i
    WHERE EXISTS (
      SELECT 1 FROM record_values JOIN attributes ON attributes.id = record_values.attribute_id
      WHERE record_values.record_id = matched.id AND record_values.active_until IS NULL
        AND record_values.attribute_id = (@attribute_ids::uuid[])[i]
        AND CASE (@operators::text[])[i]
          WHEN 'is_empty' THEN true
          WHEN 'is_not_empty' THEN true
          WHEN 'contains' THEN record_values.text ILIKE '%' || (@matches::text[])[i] || '%'
          WHEN 'not_contains' THEN record_values.text ILIKE '%' || (@matches::text[])[i] || '%'
          WHEN 'before' THEN CASE WHEN attributes.type = 'datetime'
            THEN record_instant(record_values.text, matched.timezone) < record_instant((@matches::text[])[i], matched.timezone)
            ELSE record_values.text < (@matches::text[])[i] END
          WHEN 'on_or_before' THEN CASE WHEN attributes.type = 'datetime'
            THEN record_instant(record_values.text, matched.timezone) <= record_instant((@matches::text[])[i], matched.timezone)
            ELSE record_values.text <= (@matches::text[])[i] END
          WHEN 'after' THEN CASE WHEN attributes.type = 'datetime'
            THEN record_instant(record_values.text, matched.timezone) > record_instant((@matches::text[])[i], matched.timezone)
            ELSE record_values.text > (@matches::text[])[i] END
          WHEN 'on_or_after' THEN CASE WHEN attributes.type = 'datetime'
            THEN record_instant(record_values.text, matched.timezone) >= record_instant((@matches::text[])[i], matched.timezone)
            ELSE record_values.text >= (@matches::text[])[i] END
          ELSE CASE WHEN attributes.type = 'datetime' AND (@matches::text[])[i] = 'today' THEN
            CASE WHEN length(record_values.text) = 10 THEN record_values.text::date
              ELSE (record_values.text::timestamptz AT TIME ZONE matched.timezone)::date END = (CURRENT_TIMESTAMP AT TIME ZONE matched.timezone)::date
          WHEN attributes.type = 'datetime' AND (@matches::text[])[i] = 'now' THEN
            record_instant(record_values.text, matched.timezone) = CURRENT_TIMESTAMP
          ELSE lower(record_values.text) = (@matches::text[])[i] OR record_values.unique_key = (@matches::text[])[i]
            OR record_values.ref_record_id::text = (@matches::text[])[i] END
        END
    ) = ((@operators::text[])[i] IN ('is_not', 'not_contains', 'is_empty'))
  )
)
SELECT sqlc.embed(records), coalesce(ranked.conversation_id::text, '')::text AS conversation_id, count(*) OVER () AS total FROM ranked
JOIN records ON records.id = ranked.id
JOIN workspaces ON workspaces.id = records.workspace_id
WHERE ranked.position = 1
ORDER BY (
    SELECT min(CASE WHEN attributes.type = 'datetime'
      THEN to_char(record_instant(record_values.text, workspaces.timezone) AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US')
      ELSE record_values.text END) FROM record_values JOIN attributes ON attributes.id = record_values.attribute_id
    WHERE record_values.record_id = records.id AND record_values.active_until IS NULL
      AND record_values.attribute_id = sqlc.narg(sort_attribute_id)::uuid
  ) NULLS LAST, records.created_at DESC, records.id
LIMIT @row_limit OFFSET @row_offset;

-- name: RelatedRecords :many
SELECT parents.id AS parent_id, children.id FROM records parents
JOIN LATERAL (
  SELECT records.id, records.created_at FROM record_values
  JOIN records ON records.id = record_values.record_id
  WHERE record_values.ref_record_id = parents.id AND record_values.attribute_id = @attribute_id
    AND record_values.active_until IS NULL AND records.workspace_id = @workspace_id
  ORDER BY records.created_at DESC, records.id
  LIMIT @row_limit
) children ON true
WHERE parents.workspace_id = @workspace_id AND parents.id = ANY(@ids::uuid[])
ORDER BY parents.id, children.created_at DESC, children.id;

-- name: CloseValues :exec
UPDATE record_values SET active_until = now()
WHERE record_id = @record_id AND id = ANY(@ids::bigint[]) AND active_until IS NULL;

-- name: ReviseValue :exec
UPDATE record_values SET text = @text WHERE record_id = @record_id AND id = @id AND active_until IS NULL;

-- name: InsertValue :exec
INSERT INTO record_values (record_id, attribute_id, text, ref_record_id, unique_key, source, actor_id)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListSavedFilters :many
SELECT * FROM saved_filters WHERE workspace_id = $1 AND object_id = $2 ORDER BY lower(name), id;

-- name: CreateSavedFilter :one
INSERT INTO saved_filters (id, workspace_id, object_id, name, query, filters)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateSavedFilter :one
UPDATE saved_filters SET name = @name, query = @query, filters = @filters
WHERE workspace_id = @workspace_id AND object_id = @object_id AND id = @id
RETURNING *;

-- name: DeleteFilter :execrows
DELETE FROM saved_filters WHERE workspace_id = $1 AND id = $2;
