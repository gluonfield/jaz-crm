-- name: CreateObject :one
INSERT INTO objects (workspace_id, slug, name) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateAttribute :one
INSERT INTO attributes (object_id, slug, name, type, multi, is_unique, target_object_id, options)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListObjects :many
SELECT * FROM objects WHERE workspace_id = $1 ORDER BY created_at, slug;

-- name: ListAttributes :many
SELECT attributes.* FROM attributes
JOIN objects ON objects.id = attributes.object_id
WHERE objects.workspace_id = $1
ORDER BY attributes.object_id, attributes.created_at, attributes.slug;

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
-- SearchRecords returns an object's records, newest first, whose current
-- values contain the query and satisfy every (attribute, match) filter; a
-- match equals a value's lowercased text, unique key or referenced record id.
SELECT records.* FROM records
WHERE records.workspace_id = @workspace_id AND records.object_id = @object_id
  AND (sqlc.narg(query)::text IS NULL OR EXISTS (
    SELECT 1 FROM record_values
    WHERE record_values.record_id = records.id AND record_values.active_until IS NULL
      AND record_values.text ILIKE '%' || sqlc.narg(query)::text || '%'
  ))
  AND NOT EXISTS (
    SELECT 1 FROM generate_subscripts(@attribute_ids::uuid[], 1) AS i
    WHERE NOT EXISTS (
      SELECT 1 FROM record_values
      WHERE record_values.record_id = records.id AND record_values.active_until IS NULL
        AND record_values.attribute_id = (@attribute_ids::uuid[])[i]
        AND (lower(record_values.text) = (@matches::text[])[i] OR record_values.unique_key = (@matches::text[])[i]
          OR record_values.ref_record_id::text = (@matches::text[])[i])
    )
  )
ORDER BY records.created_at DESC, records.id
LIMIT @row_limit;

-- name: CloseValues :exec
UPDATE record_values SET active_until = now()
WHERE record_id = @record_id AND id = ANY(@ids::bigint[]) AND active_until IS NULL;

-- name: InsertValue :exec
INSERT INTO record_values (record_id, attribute_id, text, ref_record_id, unique_key, source, actor_id)
VALUES ($1, $2, $3, $4, $5, $6, $7);
