-- name: SaveConnection :one
INSERT INTO connections (workspace_id, user_id, provider, account, refresh_token)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, provider, account) DO UPDATE
SET user_id = EXCLUDED.user_id, refresh_token = EXCLUDED.refresh_token, status = 'active'
RETURNING *;

-- name: GetConnection :one
SELECT * FROM connections WHERE id = $1;

-- name: ListConnections :many
SELECT * FROM connections WHERE workspace_id = $1 ORDER BY created_at;

-- name: ActiveConnections :many
SELECT * FROM connections WHERE status = 'active' ORDER BY created_at;

-- name: SetConnectionStatus :exec
UPDATE connections SET status = $2 WHERE id = $1;

-- name: DeleteConnection :execrows
DELETE FROM connections WHERE workspace_id = $1 AND id = $2;

-- name: GetCursor :one
SELECT cursor FROM sync_cursors WHERE connection_id = $1 AND stream = $2;

-- name: SetCursor :exec
INSERT INTO sync_cursors (connection_id, stream, cursor) VALUES ($1, $2, $3)
ON CONFLICT (connection_id, stream) DO UPDATE SET cursor = EXCLUDED.cursor, updated_at = now();

-- name: DeleteCursor :exec
DELETE FROM sync_cursors WHERE connection_id = $1 AND stream = $2;

-- name: ListCursors :many
SELECT * FROM sync_cursors WHERE connection_id = ANY(@connection_ids::uuid[]);

-- name: InternalAddresses :many
-- InternalAddresses are the workspace's own addresses: its members' emails and
-- its connected accounts.
SELECT lower(users.email)::text AS address FROM users WHERE users.workspace_id = @workspace_id
UNION
SELECT lower(connections.account)::text FROM connections WHERE connections.workspace_id = @workspace_id;
