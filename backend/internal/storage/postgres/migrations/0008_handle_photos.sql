-- +goose Up
-- A handle's photo is the profile picture Google shows for the address.
ALTER TABLE handles ADD COLUMN photo_url text NOT NULL DEFAULT '';
