-- +goose Up
ALTER TABLE workspaces
  ADD COLUMN auto_keep_email boolean NOT NULL DEFAULT false,
  ADD COLUMN auto_keep_meetings boolean NOT NULL DEFAULT false,
  ADD COLUMN auto_keep_records boolean NOT NULL DEFAULT false,
  ADD COLUMN auto_keep_ai boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspaces
  DROP COLUMN auto_keep_email,
  DROP COLUMN auto_keep_meetings,
  DROP COLUMN auto_keep_records,
  DROP COLUMN auto_keep_ai;
