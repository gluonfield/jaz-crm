-- +goose Up
CREATE TABLE active_filters (
  object_id uuid PRIMARY KEY REFERENCES objects(id) ON DELETE CASCADE,
  query text NOT NULL DEFAULT '',
  filters jsonb NOT NULL DEFAULT '[]',
  saved_id text REFERENCES saved_filters(id) ON DELETE SET NULL
);
