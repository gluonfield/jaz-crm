-- +goose Up
-- Other addresses a member sends from, such as a university or personal mailbox.
ALTER TABLE users ADD COLUMN addresses text[] NOT NULL DEFAULT '{}';
