-- +goose Up
-- Teammates may send replies from a connected mailbox unless its owner turns
-- that off.
ALTER TABLE connections ADD COLUMN teammates_send boolean NOT NULL DEFAULT true;
