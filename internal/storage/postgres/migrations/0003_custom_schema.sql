-- +goose Up
ALTER TABLE workspaces ADD COLUMN description text NOT NULL DEFAULT '';

ALTER TABLE attributes DROP CONSTRAINT attributes_type_check;
ALTER TABLE attributes ADD CONSTRAINT attributes_type_check
  CHECK (type IN ('text', 'number', 'date', 'checkbox', 'url', 'select', 'email', 'domain', 'phone', 'reference'));
ALTER TABLE attributes ADD COLUMN options text[] NOT NULL DEFAULT '{}';
