-- +goose Up
-- Every statement that changes a workspace's data notifies channel changes
-- with the workspace's ID at commit, whichever process wrote it. Updates
-- that rewrite rows unchanged, as a sync pass does, notify nothing.
-- notify_change(column) reads the workspace from a column of the changed
-- rows; notify_change(column, parent[, key]) from the parent row whose key,
-- id by default, the column holds.
-- A logo re-check moves its checked time and so notifies too, which a
-- domain's re-check does weekly at most.
-- +goose StatementBegin
CREATE FUNCTION notify_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  changed text := CASE TG_OP
    WHEN 'INSERT' THEN 'new_rows'
    WHEN 'DELETE' THEN 'old_rows'
    ELSE '(SELECT * FROM new_rows EXCEPT SELECT * FROM old_rows)' END;
  workspace text;
BEGIN
  FOR workspace IN EXECUTE CASE TG_NARGS
    WHEN 1 THEN format('SELECT DISTINCT changed.%I FROM %s changed', TG_ARGV[0], changed)
    ELSE format('SELECT DISTINCT parent.workspace_id FROM %s changed JOIN %I parent ON parent.%I = changed.%I', changed, TG_ARGV[1], coalesce(TG_ARGV[2], 'id'), TG_ARGV[0]) END
  LOOP
    PERFORM pg_notify('changes', workspace);
  END LOOP;
  RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
DECLARE
  watched record;
  args text;
BEGIN
  FOR watched IN SELECT * FROM (VALUES
    ('workspaces', ARRAY['id']),
    ('users', ARRAY['workspace_id']),
    ('workspace_invites', ARRAY['workspace_id']),
    ('workspace_knowledge_pages', ARRAY['workspace_id']),
    ('objects', ARRAY['workspace_id']),
    ('attributes', ARRAY['object_id', 'objects']),
    ('saved_filters', ARRAY['workspace_id']),
    ('retained_records', ARRAY['workspace_id']),
    ('record_values', ARRAY['record_id', 'retained_records']),
    ('domain_logos', ARRAY['domain', 'record_domains', 'domain']),
    ('interactions', ARRAY['workspace_id']),
    ('parts', ARRAY['interaction_id', 'interactions']),
    ('participants', ARRAY['interaction_id', 'interactions']),
    ('links', ARRAY['interaction_id', 'interactions']),
    ('handles', ARRAY['workspace_id']),
    ('domain_rules', ARRAY['workspace_id']),
    ('connections', ARRAY['workspace_id']),
    ('gmail_drafts', ARRAY['connection_id', 'connections'])
  ) AS tables(name, args)
  LOOP
    args := (SELECT string_agg(quote_literal(arg), ', ') FROM unnest(watched.args) AS arg);
    EXECUTE format('CREATE TRIGGER %I AFTER INSERT ON %I REFERENCING NEW TABLE AS new_rows FOR EACH STATEMENT EXECUTE FUNCTION notify_change(%s)', watched.name || '_inserted', watched.name, args);
    EXECUTE format('CREATE TRIGGER %I AFTER UPDATE ON %I REFERENCING OLD TABLE AS old_rows NEW TABLE AS new_rows FOR EACH STATEMENT EXECUTE FUNCTION notify_change(%s)', watched.name || '_updated', watched.name, args);
    EXECUTE format('CREATE TRIGGER %I AFTER DELETE ON %I REFERENCING OLD TABLE AS old_rows FOR EACH STATEMENT EXECUTE FUNCTION notify_change(%s)', watched.name || '_deleted', watched.name, args);
  END LOOP;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION notify_change() CASCADE;
