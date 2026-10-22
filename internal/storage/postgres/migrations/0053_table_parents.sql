-- +goose Up
ALTER TABLE objects ADD COLUMN parent_id text REFERENCES retained_records(id) ON DELETE SET NULL;
