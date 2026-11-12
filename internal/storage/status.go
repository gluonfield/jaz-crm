package storage

import (
	"context"
	"time"
)

type OperationStatus struct {
	ConnectionID *string
	Operation    string
	Reason       string
	ObservedAt   time.Time
	OccurredAt   *time.Time
	RetryAt      *time.Time
}

type OperationStore interface {
	ObserveOperation(ctx context.Context, status OperationStatus) error
	OperationErrors(ctx context.Context, workspaceID, userID string) ([]OperationStatus, error)
}
