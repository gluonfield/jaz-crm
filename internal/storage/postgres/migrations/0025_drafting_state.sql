-- +goose Up
ALTER TABLE interactions
  ADD COLUMN drafting_state text NOT NULL DEFAULT '' CHECK (drafting_state IN ('', 'drafting', 'completed', 'failed', 'skipped')),
  ADD COLUMN drafting_reason text NOT NULL DEFAULT '',
  ADD COLUMN drafting_started_at timestamptz;
