-- +goose Up
-- followed_up_at is the newest message the follow-up agent has read.
ALTER TABLE interactions ADD COLUMN followed_up_at timestamptz;
