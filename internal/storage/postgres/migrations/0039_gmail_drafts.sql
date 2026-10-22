-- +goose Up
ALTER TABLE parts DROP CONSTRAINT parts_kind_check;
ALTER TABLE parts ADD CONSTRAINT parts_kind_check CHECK (kind IN ('message', 'draft', 'description', 'transcript', 'note'));

CREATE TABLE gmail_drafts (
  connection_id uuid NOT NULL REFERENCES connections ON DELETE CASCADE,
  draft_id text NOT NULL,
  message_id text NOT NULL,
  rfc_message_id text NOT NULL DEFAULT '',
  thread_id text NOT NULL DEFAULT '',
  follow_up_id uuid UNIQUE REFERENCES records ON DELETE SET NULL,
  subject text NOT NULL DEFAULT '',
  body text NOT NULL DEFAULT '',
  html text NOT NULL DEFAULT '',
  sender text NOT NULL DEFAULT '',
  recipients text[] NOT NULL DEFAULT '{}',
  cc text[] NOT NULL DEFAULT '{}',
  bcc text[] NOT NULL DEFAULT '{}',
  state text NOT NULL CHECK (state IN ('draft', 'missing', 'sent')),
  state_changed_at timestamptz NOT NULL DEFAULT now(),
  sent_message_id text NOT NULL DEFAULT '',
  at timestamptz NOT NULL,
  attachments text[] NOT NULL DEFAULT '{}',
  PRIMARY KEY (connection_id, draft_id)
);
