-- +goose Up
-- A domain's logo is the icon its website publishes, kept for every
-- workspace alike since it is public; no image means none was found. The
-- random token names it in URLs, so logos cannot be probed by domain.
CREATE TABLE domain_logos (
  domain text PRIMARY KEY,
  token uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  content_type text NOT NULL DEFAULT '',
  image bytea,
  checked_at timestamptz NOT NULL DEFAULT now()
);
