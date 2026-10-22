-- +goose Up
ALTER TABLE saved_filters ADD COLUMN user_id text REFERENCES users(id) ON DELETE CASCADE;
DROP INDEX saved_filters_name;
CREATE UNIQUE INDEX saved_filters_shared_name ON saved_filters (workspace_id, object_id, lower(name)) WHERE user_id IS NULL;
CREATE UNIQUE INDEX saved_filters_private_name ON saved_filters (workspace_id, object_id, user_id, lower(name)) WHERE user_id IS NOT NULL;
