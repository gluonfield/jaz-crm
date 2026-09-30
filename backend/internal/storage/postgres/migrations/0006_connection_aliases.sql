-- +goose Up
-- Aliases are the other addresses a connected mailbox sends as or receives
-- at; they are the workspace's own addresses like the account itself.
ALTER TABLE connections ADD COLUMN aliases text[] NOT NULL DEFAULT '{}';
