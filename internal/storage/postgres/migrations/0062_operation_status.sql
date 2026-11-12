-- +goose Up
CREATE TABLE operation_status (
  connection_id text REFERENCES connections ON DELETE CASCADE,
  operation text NOT NULL,
  reason text NOT NULL,
  observed_at timestamptz NOT NULL,
  occurred_at timestamptz,
  retry_at timestamptz,
  UNIQUE NULLS NOT DISTINCT (connection_id, operation),
  CHECK (reason = '' OR occurred_at IS NOT NULL)
);

-- +goose StatementBegin
CREATE FUNCTION notify_operation_status() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  connection text;
BEGIN
  IF TG_OP = 'INSERT' AND NEW.reason = '' THEN
    RETURN NULL;
  END IF;
  IF TG_OP = 'UPDATE' AND (NEW.reason, NEW.occurred_at, NEW.retry_at) IS NOT DISTINCT FROM (OLD.reason, OLD.occurred_at, OLD.retry_at) THEN
    RETURN NULL;
  END IF;
  connection := CASE WHEN TG_OP = 'DELETE' THEN OLD.connection_id ELSE NEW.connection_id END;
  IF connection IS NULL THEN
    PERFORM pg_notify('changes', id) FROM workspaces;
  ELSE
    PERFORM pg_notify('changes', workspace_id) FROM connections WHERE id = connection;
  END IF;
  RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER operation_status_changed AFTER INSERT OR UPDATE OR DELETE ON operation_status
FOR EACH ROW EXECUTE FUNCTION notify_operation_status();
