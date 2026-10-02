-- +goose Up
-- NULL also schedules previously fetched plain-text bodies for an HTML refresh.
ALTER TABLE parts ADD COLUMN html text;

DROP INDEX parts_unfetched;
CREATE INDEX parts_unfetched ON parts (connection_id)
WHERE provider_id IS NOT NULL AND (content IS NULL OR html IS NULL);
