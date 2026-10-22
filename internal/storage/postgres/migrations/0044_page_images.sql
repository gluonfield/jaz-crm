-- +goose Up
CREATE TABLE page_images (
  id text PRIMARY KEY,
  record_id text NOT NULL REFERENCES records(id) ON DELETE CASCADE,
  png bytea NOT NULL
);
CREATE INDEX page_images_record ON page_images (record_id);

-- +goose Down
DROP TABLE page_images;
