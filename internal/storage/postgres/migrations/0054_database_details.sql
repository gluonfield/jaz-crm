-- +goose Up
ALTER TABLE objects ADD COLUMN icon text NOT NULL DEFAULT '';
ALTER TABLE objects ADD COLUMN description text NOT NULL DEFAULT '';
