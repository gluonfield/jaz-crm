-- +goose Up
ALTER TABLE workspaces ADD COLUMN company_page_id uuid REFERENCES records ON DELETE SET NULL;

-- Preserve an unambiguous existing root once; future reads follow its identity.
WITH candidates AS (
  SELECT records.workspace_id, records.id FROM records
  JOIN objects ON objects.id = records.object_id AND objects.slug = 'pages'
  JOIN attributes ON attributes.object_id = objects.id AND attributes.slug = 'name'
  JOIN record_values ON record_values.record_id = records.id AND record_values.attribute_id = attributes.id
    AND record_values.active_until IS NULL AND lower(record_values.text) = 'company'
  WHERE NOT EXISTS (
    SELECT 1 FROM record_values parents JOIN attributes ON attributes.id = parents.attribute_id
    WHERE parents.record_id = records.id AND attributes.slug = 'parent' AND parents.active_until IS NULL
  )
), roots AS (
  SELECT workspace_id, (array_agg(id))[1] AS id FROM candidates GROUP BY workspace_id HAVING count(*) = 1
)
UPDATE workspaces SET company_page_id = roots.id FROM roots WHERE workspaces.id = roots.workspace_id;
