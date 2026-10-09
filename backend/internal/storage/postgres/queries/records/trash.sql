-- name: TrashRecords :many
SELECT retained_records.id, objects.slug AS object, coalesce(nullif(details.name, ''), details.identity, 'Untitled')::text AS name,
  coalesce(details.icon, '')::text AS icon, retained_records.deleted_at::timestamptz AS deleted_at
FROM retained_records
JOIN objects ON objects.id = retained_records.object_id
LEFT JOIN LATERAL (
  SELECT max(record_values.text) FILTER (WHERE attributes.slug = 'name') AS name,
    min(record_values.text) FILTER (WHERE attributes.is_unique) AS identity,
    max(record_values.text) FILTER (WHERE objects.slug = 'pages' AND attributes.slug = 'icon') AS icon
  FROM record_values JOIN attributes ON attributes.id = record_values.attribute_id
  WHERE record_values.record_id = retained_records.id AND record_values.active_until IS NULL
) details ON true
WHERE retained_records.workspace_id = @workspace_id AND retained_records.deleted_at IS NOT NULL
ORDER BY retained_records.deleted_at DESC, retained_records.id;

-- name: RestoreRecord :execrows
UPDATE retained_records SET deleted_at = NULL
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NOT NULL;

-- name: RestoreRecordDomains :exec
UPDATE domain_rules SET triage = 'kept', reason = 'company restored'
WHERE domain_rules.workspace_id = @workspace_id AND triage = 'skipped' AND reason = 'company deleted'
  AND domain_rules.domain IN (SELECT domain FROM record_domains WHERE record_id = @record_id);

-- name: RestoreRecordHandles :many
UPDATE handles SET triage = 'kept', decided_by = 'user', reason = 'record restored'
WHERE handles.workspace_id = @workspace_id AND triage = 'skipped' AND decided_by = 'user' AND reason IN ('record deleted', 'company deleted')
  AND (person_id = @record_id OR kind = 'email' AND split_part(value, '@', 2) IN (SELECT domain FROM record_domains WHERE record_id = @record_id))
  AND (person_id IS NULL OR EXISTS (SELECT 1 FROM records WHERE records.id = handles.person_id))
  AND NOT EXISTS (SELECT 1 FROM domain_rules WHERE domain_rules.workspace_id = handles.workspace_id
    AND domain_rules.domain = split_part(handles.value, '@', 2) AND domain_rules.triage = 'skipped')
RETURNING id;

-- name: TrashedContactRecords :many
SELECT DISTINCT retained_records.id FROM retained_records
JOIN objects ON objects.id = retained_records.object_id
JOIN record_values ON record_values.record_id = retained_records.id AND record_values.active_until IS NULL
JOIN attributes ON attributes.id = record_values.attribute_id
WHERE retained_records.workspace_id = @workspace_id AND retained_records.deleted_at IS NOT NULL
  AND (objects.slug = 'people' AND attributes.type IN ('email', 'phone', 'url') AND record_values.unique_key = @address
    OR objects.slug = 'companies' AND attributes.type = 'domain' AND record_values.unique_key = @domain)
ORDER BY retained_records.id;
