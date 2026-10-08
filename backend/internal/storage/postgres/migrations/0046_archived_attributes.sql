-- +goose Up
ALTER TABLE attributes ADD COLUMN archived boolean NOT NULL DEFAULT false;
