-- name: ObserveOperation :exec
INSERT INTO operation_status (connection_id, operation, reason, observed_at, occurred_at, retry_at)
VALUES (sqlc.narg(connection_id), @operation, @reason, @observed_at, sqlc.narg(occurred_at), sqlc.narg(retry_at))
ON CONFLICT (connection_id, operation) DO UPDATE SET
  reason = EXCLUDED.reason, observed_at = EXCLUDED.observed_at,
  occurred_at = EXCLUDED.occurred_at, retry_at = EXCLUDED.retry_at
WHERE EXCLUDED.observed_at > operation_status.observed_at;

-- name: OperationErrors :many
SELECT s.* FROM operation_status s LEFT JOIN connections c ON c.id = s.connection_id
WHERE s.reason <> '' AND (s.connection_id IS NULL OR c.workspace_id = @workspace_id AND c.user_id = @user_id)
ORDER BY s.occurred_at DESC, s.operation, s.connection_id;
