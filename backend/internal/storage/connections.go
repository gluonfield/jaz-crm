package storage

import (
	"context"
	"time"
)

// Connection is one member's account at a provider, feeding one workspace.
// RefreshToken is encrypted.
type Connection struct {
	ID           string
	WorkspaceID  string
	UserID       string
	Provider     string
	Account      string
	RefreshToken []byte
	Status       string
	CreatedAt    time.Time
}

type NewConnection struct {
	WorkspaceID  string
	UserID       string
	Provider     string
	Account      string
	RefreshToken []byte
}

type SyncCursor struct {
	ConnectionID string
	Stream       string
	Cursor       string
	UpdatedAt    time.Time
}

type ConnectionStore interface {
	// SaveConnection creates the connection or reactivates it with a new token.
	SaveConnection(ctx context.Context, c NewConnection) (Connection, error)
	Connection(ctx context.Context, id string) (Connection, error)
	Connections(ctx context.Context, workspaceID string) ([]Connection, error)
	ActiveConnections(ctx context.Context) ([]Connection, error)
	SetConnectionStatus(ctx context.Context, id, status string) error
	DeleteConnection(ctx context.Context, workspaceID, id string) error
	// Cursor returns ErrNotFound for a stream that has not started.
	Cursor(ctx context.Context, connectionID, stream string) (string, error)
	SetCursor(ctx context.Context, connectionID, stream, cursor string) error
	DeleteCursor(ctx context.Context, connectionID, stream string) error
	Cursors(ctx context.Context, connectionIDs []string) ([]SyncCursor, error)
	// InternalAddresses are the workspace's members' and connections' emails.
	InternalAddresses(ctx context.Context, workspaceID string) ([]string, error)
}
