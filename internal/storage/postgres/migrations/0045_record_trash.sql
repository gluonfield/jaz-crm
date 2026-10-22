-- +goose Up
ALTER TABLE records ADD COLUMN deleted_at timestamptz;
ALTER TABLE records RENAME TO retained_records;

-- Keep ordinary reads and writes on active records. Foreign keys retain
-- the original rows, so restoring also restores their references and links.
CREATE VIEW records AS
SELECT id, workspace_id, object_id, created_at, updated_at
FROM retained_records WHERE deleted_at IS NULL;

CREATE VIEW active_links AS
SELECT links.* FROM links JOIN records ON records.id = links.record_id;

CREATE VIEW record_domains AS
SELECT retained_records.id AS record_id, retained_records.workspace_id, record_values.text AS domain
FROM retained_records JOIN objects ON objects.id = retained_records.object_id AND objects.slug = 'companies'
JOIN record_values ON record_values.record_id = retained_records.id AND record_values.active_until IS NULL
JOIN attributes ON attributes.id = record_values.attribute_id AND attributes.type = 'domain';
