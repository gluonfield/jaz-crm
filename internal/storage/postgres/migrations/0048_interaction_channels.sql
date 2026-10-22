-- +goose Up
-- An email thread is a message conversation on the email channel. A
-- conversation's identity is its channel and external id; url opens the original.
ALTER TABLE interactions DROP CONSTRAINT interactions_kind_check;
UPDATE interactions SET kind = 'message', channel = 'email' WHERE kind = 'email';
ALTER TABLE interactions ADD CONSTRAINT interactions_kind_check CHECK (kind IN ('message', 'meeting', 'call', 'note'));
ALTER TABLE interactions ADD CONSTRAINT interactions_channel_check CHECK ((kind = 'message') = (channel <> ''));
ALTER TABLE interactions ADD COLUMN url text NOT NULL DEFAULT '';

UPDATE interactions SET url = 'https://mail.google.com/mail/u/' || connections.account || '/#all/' || interactions.external_id
FROM connections WHERE connections.id = interactions.connection_id AND interactions.source = 'gmail';

-- Calendar events carry their own link, which a full sync brings.
DELETE FROM sync_cursors WHERE stream = 'calendar';

-- A logged conversation's original is the first link its provenance named;
-- a note keeps its citations in its text.
UPDATE interactions SET url = rtrim(substring(provenance FROM 'https?://[^\s<>"'']+'), '.,;:)/')
WHERE connection_id IS NULL AND kind <> 'note' AND provenance ~ 'https?://';
UPDATE parts SET content = parts.content || E'\n\n' || interactions.provenance
FROM interactions
WHERE parts.interaction_id = interactions.id AND parts.kind = 'note' AND interactions.kind = 'note' AND interactions.provenance <> '';

ALTER TABLE interactions DROP CONSTRAINT interactions_workspace_id_source_external_id_key;
ALTER TABLE interactions DROP COLUMN source, DROP COLUMN provenance;
ALTER TABLE interactions ADD CONSTRAINT interactions_workspace_id_channel_external_id_key UNIQUE (workspace_id, channel, external_id);

-- A handle is an address on a channel.
ALTER TABLE handles DROP CONSTRAINT handles_kind_check;
ALTER TABLE handles ADD CONSTRAINT handles_kind_check CHECK (kind IN ('email', 'phone', 'linkedin', 'x', 'telegram'));

-- Follow-ups reply on any conversation channel; a workspace's own options stay.
UPDATE attributes SET options = ARRAY['Email', 'LinkedIn', 'WhatsApp', 'X', 'Telegram', 'SMS']
  || array(SELECT o FROM unnest(attributes.options) AS o WHERE lower(o) <> ALL (ARRAY['email', 'linkedin', 'whatsapp', 'x', 'telegram', 'sms']))
FROM objects
WHERE objects.id = attributes.object_id AND objects.slug = 'follow_ups' AND attributes.slug = 'channel';
