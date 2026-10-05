-- name: UserByAPIKey :one
SELECT users.* FROM api_keys
JOIN users ON users.id = api_keys.user_id
WHERE api_keys.key_hash = $1;

-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, label, hint, key_hash, id) VALUES ($1, $2, $3, $4, sqlc.arg(id)) RETURNING *;

-- name: ReplaceAPIKey :one
-- ReplaceAPIKey makes the key the user's one key with this label, keeping it
-- when already registered to them and deleting the label's other keys.
WITH replaced AS (
  DELETE FROM api_keys WHERE user_id = sqlc.arg(user_id) AND label = sqlc.arg(label) AND key_hash <> sqlc.arg(key_hash)
)
INSERT INTO api_keys (user_id, label, hint, key_hash, id)
VALUES (sqlc.arg(user_id), sqlc.arg(label), sqlc.arg(hint), sqlc.arg(key_hash), sqlc.arg(id))
ON CONFLICT (key_hash) DO UPDATE SET label = EXCLUDED.label
WHERE api_keys.user_id = EXCLUDED.user_id
RETURNING *;

-- name: ListAPIKeys :many
SELECT * FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC;

-- name: DeleteAPIKey :execrows
DELETE FROM api_keys WHERE user_id = $1 AND id = $2;

-- name: UsersByEmail :many
SELECT * FROM users WHERE lower(email) = lower($1) ORDER BY created_at;

-- name: ShareIdentity :many
-- ShareIdentity links the identity to every user another identity signs in
-- as, returning the users it newly reaches.
WITH linked AS (
  INSERT INTO identities (issuer, subject, user_id)
  SELECT sqlc.arg(issuer), sqlc.arg(subject), identities.user_id FROM identities
  WHERE identities.issuer = sqlc.arg(from_issuer) AND identities.subject = sqlc.arg(from_subject)
  ON CONFLICT DO NOTHING
  RETURNING user_id
)
SELECT users.* FROM users JOIN linked ON linked.user_id = users.id ORDER BY users.created_at;

-- name: CreateWorkspace :one
INSERT INTO workspaces (name, id) VALUES ($1, sqlc.arg(id)) RETURNING *;

-- name: GetWorkspace :one
SELECT id, name, created_at, description, auto_keep_email, auto_keep_meetings,
  auto_keep_records, auto_keep_ai, drafting_web_access,
  ARRAY(SELECT page_id FROM workspace_knowledge_pages WHERE workspace_id = workspaces.id ORDER BY position)::text[] AS company_page_ids, timezone
FROM workspaces WHERE id = $1;

-- name: LockWorkspace :one
SELECT id FROM workspaces WHERE id = $1 FOR NO KEY UPDATE;

-- name: SetUserAddresses :execrows
UPDATE users SET addresses = @addresses::text[] WHERE id = @id AND workspace_id = @workspace_id;

-- name: UpdateWorkspace :execrows
-- Keep the legacy root readable while older workers finish a rolling deployment.
UPDATE workspaces SET name = COALESCE(sqlc.narg(name), name), description = COALESCE(sqlc.narg(description), description),
  company_page_id = CASE WHEN @company_page_ids::text[] IS NULL THEN company_page_id ELSE (@company_page_ids::text[])[1] END,
  drafting_web_access = COALESCE(sqlc.narg(drafting_web_access), drafting_web_access),
  timezone = COALESCE(sqlc.narg(timezone), timezone)
WHERE workspaces.id = @id AND NOT EXISTS (
  SELECT 1 FROM unnest(@company_page_ids::text[]) AS selected(id)
  LEFT JOIN records ON records.id = selected.id AND records.workspace_id = @id
  LEFT JOIN objects ON objects.id = records.object_id AND objects.slug = 'pages'
  WHERE objects.id IS NULL
);

-- name: ClearKnowledgePages :exec
DELETE FROM workspace_knowledge_pages WHERE workspace_id = $1;

-- name: SetKnowledgePages :exec
INSERT INTO workspace_knowledge_pages (workspace_id, page_id, position)
SELECT @workspace_id, page_id, position FROM unnest(@company_page_ids::text[]) WITH ORDINALITY AS selected(page_id, position);

-- name: DeleteWorkspace :execrows
DELETE FROM workspaces WHERE id = $1 AND name = $2;

-- name: CreateAuthUser :one
INSERT INTO users (workspace_id, name, email, avatar_url, admin, id)
VALUES ($1, $2, $3, $4, $5, sqlc.arg(id))
RETURNING *;

-- name: ListUsers :many
SELECT * FROM users WHERE workspace_id = $1 ORDER BY created_at;

-- name: LinkIdentity :exec
INSERT INTO identities (issuer, subject, user_id) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3);

-- name: UserBySession :one
SELECT users.* FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = $1 AND sessions.expires_at > now();

-- name: UpdateSessionUser :execrows
-- UpdateSessionUser moves a session to another of its person's users.
UPDATE sessions SET user_id = $2 WHERE token_hash = $1;

-- name: UserIdentities :many
SELECT * FROM identities WHERE user_id = $1 ORDER BY created_at;

-- name: Memberships :many
-- Memberships lists the workspaces of a user's person: the users that share
-- an identity with it, and itself.
SELECT users.id AS user_id, workspaces.id AS workspace_id, workspaces.name FROM users
JOIN workspaces ON workspaces.id = users.workspace_id
WHERE users.id = @user_id OR users.id IN (
  SELECT other.user_id FROM identities mine
  JOIN identities other ON other.issuer = mine.issuer AND other.subject = mine.subject
  WHERE mine.user_id = @user_id
)
ORDER BY workspaces.created_at;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: UsersByIdentity :many
SELECT users.* FROM identities
JOIN users ON users.id = identities.user_id
WHERE identities.issuer = $1 AND identities.subject = $2
ORDER BY users.created_at;

-- name: CreateInvite :one
INSERT INTO workspace_invites (workspace_id, email, invited_by, id) VALUES ($1, lower(@email::text), $2, sqlc.arg(id))
RETURNING *;

-- name: ListInvites :many
SELECT * FROM workspace_invites WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: InvitesByEmail :many
SELECT * FROM workspace_invites WHERE email = lower(@email::text);

-- name: DeleteInvite :execrows
DELETE FROM workspace_invites WHERE workspace_id = $1 AND id = $2;

-- name: GetTriageSettings :one
SELECT auto_keep_email, auto_keep_meetings, auto_keep_records, auto_keep_ai FROM workspaces WHERE id = $1;

-- name: UpdateTriageSettings :execrows
UPDATE workspaces SET auto_keep_email = $2, auto_keep_meetings = $3, auto_keep_records = $4, auto_keep_ai = $5 WHERE id = $1;
