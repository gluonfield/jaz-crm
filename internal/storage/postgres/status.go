package postgres

import (
	"context"

	"github.com/gluonfield/jaz-crm/internal/storage"
	statusdb "github.com/gluonfield/jaz-crm/internal/storage/postgres/generated/status"
)

func (s *Store) ObserveOperation(ctx context.Context, status storage.OperationStatus) error {
	return mapError(s.status.ObserveOperation(ctx, statusdb.ObserveOperationParams(status)))
}

func (s *Store) OperationErrors(ctx context.Context, workspaceID, userID string) ([]storage.OperationStatus, error) {
	return many(func(row statusdb.OperationStatus) storage.OperationStatus {
		return storage.OperationStatus(row)
	})(s.status.OperationErrors(ctx, statusdb.OperationErrorsParams{WorkspaceID: workspaceID, UserID: userID}))
}
