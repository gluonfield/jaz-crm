-- +goose Up
ALTER TABLE records ADD COLUMN updated_at timestamptz;

UPDATE records SET updated_at = greatest(records.created_at, changes.at)
FROM (
  SELECT records.id, max(greatest(record_values.active_from, record_values.active_until)) AS at
  FROM records LEFT JOIN record_values ON record_values.record_id = records.id
  GROUP BY records.id
) changes
WHERE records.id = changes.id;

ALTER TABLE records ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE records ALTER COLUMN updated_at SET NOT NULL;
CREATE INDEX records_updated ON records (object_id, updated_at DESC, id);

-- +goose StatementBegin
CREATE FUNCTION record_value_updated() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND NEW IS NOT DISTINCT FROM OLD THEN
    RETURN NULL;
  END IF;
  UPDATE records SET updated_at = clock_timestamp()
  WHERE id = CASE WHEN TG_OP = 'DELETE' THEN OLD.record_id ELSE NEW.record_id END;
  RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER record_value_updated
AFTER INSERT OR UPDATE OR DELETE ON record_values
FOR EACH ROW EXECUTE FUNCTION record_value_updated();

-- +goose Down
DROP TRIGGER record_value_updated ON record_values;
DROP FUNCTION record_value_updated();
DROP INDEX records_updated;
ALTER TABLE records DROP COLUMN updated_at;
