-- name: CreateObject :one
INSERT INTO objects (workspace_id, slug, name) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateAttribute :one
INSERT INTO attributes (object_id, slug, name, type, multi, is_unique, target_object_id, options)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

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
SELECT records.* FROM records
WHERE records.workspace_id = @workspace_id AND records.object_id = @object_id
  AND (sqlc.narg(query)::text IS NULL OR EXISTS (
    SELECT 1 FROM record_values
    WHERE record_values.record_id = records.id AND record_values.active_until IS NULL
      AND record_values.text ILIKE '%' || sqlc.narg(query)::text || '%'
  ))
  AND NOT EXISTS (
    SELECT 1 FROM generate_subscripts(@attribute_ids::uuid[], 1) AS i
    WHERE EXISTS (
      SELECT 1 FROM record_values
      WHERE record_values.record_id = records.id AND record_values.active_until IS NULL
        AND record_values.attribute_id = (@attribute_ids::uuid[])[i]
        AND CASE (@operators::text[])[i]
          WHEN 'is_empty' THEN true
          WHEN 'is_not_empty' THEN true
          WHEN 'contains' THEN record_values.text ILIKE '%' || (@matches::text[])[i] || '%'
          WHEN 'not_contains' THEN record_values.text ILIKE '%' || (@matches::text[])[i] || '%'
          WHEN 'before' THEN record_values.text < (@matches::text[])[i]
          WHEN 'on_or_before' THEN record_values.text <= (@matches::text[])[i]
          WHEN 'after' THEN record_values.text > (@matches::text[])[i]
          WHEN 'on_or_after' THEN record_values.text >= (@matches::text[])[i]
          ELSE lower(record_values.text) = (@matches::text[])[i] OR record_values.unique_key = (@matches::text[])[i]
            OR record_values.ref_record_id::text = (@matches::text[])[i]
        END
    ) = ((@operators::text[])[i] IN ('is_not', 'not_contains', 'is_empty'))
  )
ORDER BY (
    SELECT min(record_values.text) FROM record_values
    WHERE record_values.record_id = records.id AND record_values.active_until IS NULL
      AND record_values.attribute_id = sqlc.narg(sort_attribute_id)::uuid
  ) NULLS LAST, records.created_at DESC, records.id
LIMIT @row_limit;

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
