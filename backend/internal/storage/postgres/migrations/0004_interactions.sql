-- +goose Up
-- A connection is one member's Google account feeding one workspace.
CREATE TABLE connections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
  provider text NOT NULL CHECK (provider IN ('google')),
  account text NOT NULL,
  refresh_token bytea NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, provider, account)
);

-- A cursor is where one of a connection's streams resumes, such as a Gmail
-- history id or a Calendar sync token.
CREATE TABLE sync_cursors (
  connection_id uuid NOT NULL REFERENCES connections ON DELETE CASCADE,
  stream text NOT NULL,
  cursor text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (connection_id, stream)
);

-- A handle is an address seen in the workspace's mail and meetings, and the
-- triage verdict on whether its person belongs in the CRM.
CREATE TABLE handles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('email', 'phone')),
  value text NOT NULL,
  name text NOT NULL DEFAULT '',
  person_id uuid REFERENCES records ON DELETE SET NULL,
  triage text NOT NULL CHECK (triage IN ('pending', 'kept', 'skipped', 'internal')),
  decided_by text CHECK (decided_by IN ('rule', 'agent', 'engagement', 'user')),
  reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, kind, value)
);

CREATE INDEX handles_triage ON handles (workspace_id, triage, created_at DESC);

-- An interaction is an email thread, a meeting, a call or a note.
CREATE TABLE interactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('email', 'meeting', 'call', 'note')),
  source text NOT NULL CHECK (source IN ('gmail', 'calendar', 'manual', 'webhook')),
  external_id text NOT NULL,
  connection_id uuid REFERENCES connections ON DELETE SET NULL,
  user_id uuid REFERENCES users ON DELETE SET NULL,
  title text NOT NULL DEFAULT '',
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  meet_code text NOT NULL DEFAULT '',
  transcript_checked_at timestamptz,
  skipped boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, source, external_id)
);

CREATE INDEX interactions_started ON interactions (workspace_id, started_at DESC);

CREATE TABLE participants (
  interaction_id uuid NOT NULL REFERENCES interactions ON DELETE CASCADE,
  handle_id uuid NOT NULL REFERENCES handles ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('from', 'to', 'cc', 'organizer', 'attendee', 'declined')),
  PRIMARY KEY (interaction_id, handle_id, role)
);

CREATE INDEX participants_handle ON participants (handle_id);

-- A part is one message, transcript entry, description or note. Content a
-- provider holds (provider_id) is fetched only while the interaction is
-- linked to a record, and cleared when it no longer is.
CREATE TABLE parts (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  interaction_id uuid NOT NULL REFERENCES interactions ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('message', 'description', 'transcript', 'note')),
  external_id text NOT NULL,
  connection_id uuid REFERENCES connections ON DELETE SET NULL,
  provider_id text,
  author_handle_id uuid REFERENCES handles ON DELETE SET NULL,
  author_name text NOT NULL DEFAULT '',
  at timestamptz NOT NULL,
  content text,
  search tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(content, ''))) STORED,
  UNIQUE (interaction_id, external_id)
);

CREATE INDEX parts_external ON parts (external_id);
CREATE INDEX parts_search ON parts USING gin (search);
CREATE INDEX parts_unfetched ON parts (connection_id) WHERE content IS NULL AND provider_id IS NOT NULL;

-- A link ties an interaction to a record. Sync links are derived from kept
-- participants and rebuilt when triage changes; agent and user links stay.
CREATE TABLE links (
  interaction_id uuid NOT NULL REFERENCES interactions ON DELETE CASCADE,
  record_id uuid NOT NULL REFERENCES records ON DELETE CASCADE,
  source text NOT NULL CHECK (source IN ('sync', 'agent', 'user')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (interaction_id, record_id)
);

CREATE INDEX links_record ON links (record_id);

-- A domain rule is a person's keep or skip decision for every address at a
-- domain, including ones not seen yet.
CREATE TABLE domain_rules (
  workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
  domain text NOT NULL,
  triage text NOT NULL CHECK (triage IN ('kept', 'skipped')),
  reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, domain)
);
