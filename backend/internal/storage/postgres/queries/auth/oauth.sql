-- name: CreateOAuthClient :one
INSERT INTO oauth_clients (id, name, redirect_uris) VALUES ($1, $2, $3) RETURNING *;

-- name: GetOAuthClient :one
SELECT * FROM oauth_clients WHERE id = $1;

-- name: CreateOAuthCode :exec
INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, scope, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ConsumeOAuthCode :one
DELETE FROM oauth_codes WHERE code_hash = $1 RETURNING *;

-- name: CreateOAuthGrant :one
INSERT INTO oauth_grants (client_id, user_id, scope) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateOAuthToken :exec
INSERT INTO oauth_tokens (token_hash, grant_id, kind, expires_at) VALUES ($1, $2, $3, $4);

-- name: GetOAuthToken :one
SELECT sqlc.embed(oauth_tokens), sqlc.embed(oauth_grants) FROM oauth_tokens
JOIN oauth_grants ON oauth_grants.id = oauth_tokens.grant_id
WHERE oauth_tokens.token_hash = $1;

-- name: UserByAccessToken :one
SELECT users.* FROM oauth_tokens
JOIN oauth_grants ON oauth_grants.id = oauth_tokens.grant_id
JOIN users ON users.id = oauth_grants.user_id
WHERE oauth_tokens.token_hash = $1 AND oauth_tokens.kind = 'access'
  AND oauth_tokens.revoked_at IS NULL AND oauth_tokens.expires_at > now()
  AND oauth_grants.revoked_at IS NULL;

-- name: RevokeOAuthToken :execrows
UPDATE oauth_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeTokenFamily :exec
UPDATE oauth_tokens SET revoked_at = now()
WHERE oauth_tokens.revoked_at IS NULL AND (oauth_tokens.token_hash = @hash::bytea OR oauth_tokens.grant_id = (
  SELECT refresh.grant_id FROM oauth_tokens refresh WHERE refresh.token_hash = @hash::bytea AND refresh.kind = 'refresh'
));

-- name: RevokeGrantTokens :exec
UPDATE oauth_tokens SET revoked_at = now() WHERE grant_id = $1 AND revoked_at IS NULL;

-- name: RevokeOAuthGrant :execrows
UPDATE oauth_grants SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: ListOAuthGrants :many
SELECT oauth_grants.id, oauth_clients.name AS client_name, oauth_grants.created_at,
  max(oauth_tokens.created_at)::timestamptz AS last_used_at
FROM oauth_grants
JOIN oauth_clients ON oauth_clients.id = oauth_grants.client_id
JOIN oauth_tokens ON oauth_tokens.grant_id = oauth_grants.id
WHERE oauth_grants.user_id = $1 AND oauth_grants.revoked_at IS NULL
GROUP BY oauth_grants.id, oauth_clients.name
ORDER BY last_used_at DESC;

-- name: RevokeGrantByID :exec
UPDATE oauth_grants SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

