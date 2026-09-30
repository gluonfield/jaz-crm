-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE objects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
  slug text NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, slug)
);

CREATE TABLE attributes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  object_id uuid NOT NULL REFERENCES objects ON DELETE CASCADE,
  slug text NOT NULL,
  name text NOT NULL,
  type text NOT NULL CHECK (type IN ('text', 'email', 'domain', 'phone', 'reference')),
  multi boolean NOT NULL DEFAULT false,
  is_unique boolean NOT NULL DEFAULT false,
  target_object_id uuid REFERENCES objects ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (object_id, slug),
  CHECK ((type = 'reference') = (target_object_id IS NOT NULL))
);

CREATE TABLE records (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
  object_id uuid NOT NULL REFERENCES objects ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX records_object ON records (object_id, created_at DESC);

-- Values are append-only: a change closes the current row and inserts its
-- successor, so every attribute keeps its history. unique_key holds the
-- normalized value of a unique attribute, such as a lowercased email.
CREATE TABLE record_values (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  record_id uuid NOT NULL REFERENCES records ON DELETE CASCADE,
  attribute_id uuid NOT NULL REFERENCES attributes ON DELETE CASCADE,
  text text,
  ref_record_id uuid REFERENCES records ON DELETE CASCADE,
  unique_key text,
  source text NOT NULL CHECK (source IN ('user', 'agent', 'sync')),
  actor_id uuid REFERENCES users ON DELETE SET NULL,
  active_from timestamptz NOT NULL DEFAULT now(),
  active_until timestamptz,
  CHECK (num_nonnulls(text, ref_record_id) = 1)
);

CREATE INDEX record_values_current ON record_values (record_id, attribute_id) WHERE active_until IS NULL;
CREATE UNIQUE INDEX record_values_unique ON record_values (attribute_id, unique_key) WHERE active_until IS NULL;
CREATE INDEX record_values_ref ON record_values (ref_record_id) WHERE active_until IS NULL;
CREATE INDEX record_values_text ON record_values USING gin (text gin_trgm_ops) WHERE active_until IS NULL;
