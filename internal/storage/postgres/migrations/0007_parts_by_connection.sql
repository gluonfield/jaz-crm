-- +goose Up
-- Sync progress counts each connection's mail.
CREATE INDEX parts_connection ON parts (connection_id, kind);
