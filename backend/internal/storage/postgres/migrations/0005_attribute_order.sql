-- +goose Up
-- Attributes list in the order they were created, including those a new
-- workspace creates in one transaction.
ALTER TABLE attributes ALTER COLUMN created_at SET DEFAULT clock_timestamp();
