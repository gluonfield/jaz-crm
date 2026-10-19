-- +goose Up
ALTER TABLE objects ADD COLUMN columns text[] NOT NULL DEFAULT '{}';
