-- name: LockGmailDraft :exec
SELECT pg_advisory_xact_lock(hashtextextended('gmail-draft:' || sqlc.arg(connection_id)::text || ':' || sqlc.arg(draft_id)::text, 0));

-- name: GmailDrafts :many
SELECT * FROM gmail_drafts WHERE connection_id = $1
  AND (state = 'draft' OR state = 'missing' AND state_changed_at > now() - interval '24 hours')
ORDER BY draft_id;

-- name: GmailDraftByID :one
SELECT * FROM gmail_drafts WHERE connection_id = $1 AND draft_id = $2;

-- name: GmailDraft :one
SELECT d.* FROM gmail_drafts d
JOIN connections c ON c.id = d.connection_id
WHERE c.workspace_id = $1 AND d.follow_up_id = $2;

-- name: InteractionDrafts :many
SELECT l.interaction_id, sqlc.embed(d)
FROM links l JOIN gmail_drafts d ON d.follow_up_id = l.record_id
WHERE l.interaction_id = ANY(@ids::text[]) AND d.state <> 'sent'
ORDER BY l.interaction_id, d.at, d.draft_id;

-- name: UpsertGmailDraft :exec
INSERT INTO gmail_drafts (
  connection_id, draft_id, message_id, rfc_message_id, thread_id,
  subject, body, html, sender, recipients, cc, bcc, state, sent_message_id, at, attachments
) VALUES (
  @connection_id, @draft_id, @message_id, @rfc_message_id, @thread_id,
  @subject, @body, @html, @sender, coalesce(@recipients::text[], '{}'),
  coalesce(@cc::text[], '{}'), coalesce(@bcc::text[], '{}'), @state, @sent_message_id,
  @at, coalesce(@attachments::text[], '{}')
)
ON CONFLICT (connection_id, draft_id) DO UPDATE SET
  message_id = EXCLUDED.message_id, rfc_message_id = EXCLUDED.rfc_message_id,
  thread_id = EXCLUDED.thread_id, subject = EXCLUDED.subject, body = EXCLUDED.body,
  html = EXCLUDED.html, sender = EXCLUDED.sender, recipients = EXCLUDED.recipients,
  cc = EXCLUDED.cc, bcc = EXCLUDED.bcc, state = EXCLUDED.state,
  state_changed_at = CASE WHEN gmail_drafts.state <> EXCLUDED.state THEN now() ELSE gmail_drafts.state_changed_at END,
  sent_message_id = EXCLUDED.sent_message_id, at = EXCLUDED.at, attachments = EXCLUDED.attachments
WHERE gmail_drafts.state <> 'sent' OR EXCLUDED.state = 'sent';

-- name: BindGmailDraft :execrows
UPDATE gmail_drafts d SET follow_up_id = @follow_up_id
FROM connections c, records r JOIN objects o ON o.id = r.object_id
WHERE d.connection_id = @connection_id AND d.draft_id = @draft_id
  AND c.id = d.connection_id AND r.id = @follow_up_id AND r.workspace_id = c.workspace_id
  AND o.slug = 'follow_ups' AND (d.follow_up_id IS NULL OR d.follow_up_id = @follow_up_id);

-- name: MigrateDraftMessage :many
UPDATE parts p SET kind = 'draft' FROM interactions i
WHERE i.id = p.interaction_id AND p.kind = 'message'
  AND p.connection_id = @connection_id AND p.provider_id = @provider_id
RETURNING p.interaction_id;

-- name: RepairDraftInteractions :exec
UPDATE interactions i SET
  started_at = coalesce((SELECT min(p.at) FROM parts p WHERE p.interaction_id = i.id AND p.kind = 'message'), i.started_at),
  ended_at = (SELECT max(p.at) FROM parts p WHERE p.interaction_id = i.id AND p.kind = 'message'),
  followed_up_at = NULL, drafting_state = '', drafting_reason = '', drafting_started_at = NULL
WHERE i.id = ANY(@ids::text[]);

-- name: LinkKnownSentGmailDrafts :exec
INSERT INTO links (interaction_id, record_id, source)
SELECT p.interaction_id, d.follow_up_id, 'agent'
FROM parts p JOIN gmail_drafts d ON d.connection_id = p.connection_id AND d.sent_message_id = p.provider_id
WHERE d.connection_id = @connection_id AND d.sent_message_id = @provider_id AND p.kind = 'message'
  AND d.state = 'sent' AND d.follow_up_id IS NOT NULL
ON CONFLICT (interaction_id, record_id) DO NOTHING;

-- name: LinkSentGmailDrafts :exec
INSERT INTO links (interaction_id, record_id, source)
SELECT i.id, d.follow_up_id, 'agent'
FROM gmail_drafts d JOIN connections c ON c.id = d.connection_id
JOIN interactions i ON i.workspace_id = c.workspace_id AND i.id = @interaction_id
WHERE d.connection_id = @connection_id AND d.sent_message_id = @provider_id
  AND d.state = 'sent' AND d.follow_up_id IS NOT NULL
ON CONFLICT (interaction_id, record_id) DO NOTHING;
