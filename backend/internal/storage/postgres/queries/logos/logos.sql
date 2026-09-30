-- name: StaleLogoDomains :many
-- StaleLogoDomains are a workspace's record domains whose logo was never
-- looked for, or not for a month; a week when none was found.
SELECT DISTINCT record_values.text::text AS domain FROM record_values
JOIN records ON records.id = record_values.record_id
JOIN attributes ON attributes.id = record_values.attribute_id AND attributes.type = 'domain'
LEFT JOIN domain_logos ON domain_logos.domain = record_values.text
WHERE records.workspace_id = @workspace_id AND record_values.active_until IS NULL
  AND (domain_logos.domain IS NULL
    OR domain_logos.checked_at < now() - interval '30 days'
    OR domain_logos.image IS NULL AND domain_logos.checked_at < now() - interval '7 days')
LIMIT @row_limit;

-- name: SaveLogo :exec
INSERT INTO domain_logos (domain, content_type, image) VALUES (@domain, @content_type, @image)
ON CONFLICT (domain) DO UPDATE SET content_type = EXCLUDED.content_type, image = EXCLUDED.image, checked_at = now();

-- name: LogoByToken :one
SELECT content_type, image FROM domain_logos WHERE token = @token AND image IS NOT NULL;

-- name: RecordLogos :many
-- RecordLogos names the logo of each record that has a domain with one.
SELECT DISTINCT ON (records.id) records.id::text AS record_id, domain_logos.token::text AS token FROM records
JOIN record_values ON record_values.record_id = records.id AND record_values.active_until IS NULL
JOIN attributes ON attributes.id = record_values.attribute_id AND attributes.type = 'domain'
JOIN domain_logos ON domain_logos.domain = record_values.text AND domain_logos.image IS NOT NULL
WHERE records.workspace_id = @workspace_id AND records.id = ANY(@record_ids::uuid[])
ORDER BY records.id, record_values.id;
