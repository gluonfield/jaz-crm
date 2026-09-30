-- name: UserByAPIKey :one
SELECT users.* FROM api_keys
JOIN users ON users.id = api_keys.user_id
WHERE api_keys.key_hash = $1;

-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, label, hint, key_hash) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ReplaceAPIKey :one
-- ReplaceAPIKey makes the key the user's one key with this label, keeping it
-- when already registered to them and deleting the label's other keys.
WITH replaced AS (
  DELETE FROM api_keys WHERE user_id = sqlc.arg(user_id) AND label = sqlc.arg(label) AND key_hash <> sqlc.arg(key_hash)
)
INSERT INTO api_keys (user_id, label, hint, key_hash)
VALUES (sqlc.arg(user_id), sqlc.arg(label), sqlc.arg(hint), sqlc.arg(key_hash))
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
INSERT INTO workspaces (name) VALUES ($1) RETURNING *;

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = $1;

-- name: UpdateWorkspace :execrows
UPDATE workspaces SET name = $2, description = $3 WHERE id = $1;

-- name: CreateAuthUser :one
INSERT INTO users (workspace_id, name, email, avatar_url, admin)
VALUES ($1, $2, $3, $4, $5)
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
INSERT INTO workspace_invites (workspace_id, email, invited_by) VALUES ($1, lower(@email::text), $2)
RETURNING *;

-- name: ListInvites :many
SELECT * FROM workspace_invites WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: InvitesByEmail :many
SELECT * FROM workspace_invites WHERE email = lower(@email::text);

-- name: DeleteInvite :execrows
DELETE FROM workspace_invites WHERE workspace_id = $1 AND id = $2;
