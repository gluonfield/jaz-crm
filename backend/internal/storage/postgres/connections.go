package postgres

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	conndb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/connections"
)

func toConnection(r conndb.Connection) storage.Connection { return storage.Connection(r) }
func toCursor(r conndb.SyncCursor) storage.SyncCursor     { return storage.SyncCursor(r) }

func (s *Store) SaveConnection(ctx context.Context, c storage.NewConnection) (storage.Connection, error) {
	return one(toConnection)(s.conn.SaveConnection(ctx, conndb.SaveConnectionParams(c)))
}

func (s *Store) Connection(ctx context.Context, id string) (storage.Connection, error) {
	return one(toConnection)(s.conn.GetConnection(ctx, id))
}

func (s *Store) Connections(ctx context.Context, workspaceID string) ([]storage.Connection, error) {
	return many(toConnection)(s.conn.ListConnections(ctx, workspaceID))
}

func (s *Store) ActiveConnections(ctx context.Context) ([]storage.Connection, error) {
	return many(toConnection)(s.conn.ActiveConnections(ctx))
}

func (s *Store) SetConnectionStatus(ctx context.Context, id, status string) error {
	return mapError(s.conn.SetConnectionStatus(ctx, conndb.SetConnectionStatusParams{ID: id, Status: status}))
}

func (s *Store) DeleteConnection(ctx context.Context, workspaceID, id string) error {
	return affected(s.conn.DeleteConnection(ctx, conndb.DeleteConnectionParams{WorkspaceID: workspaceID, ID: id}))
}

func (s *Store) Cursor(ctx context.Context, connectionID, stream string) (string, error) {
	cursor, err := s.conn.GetCursor(ctx, conndb.GetCursorParams{ConnectionID: connectionID, Stream: stream})
	return cursor, mapError(err)
}

func (s *Store) SetCursor(ctx context.Context, connectionID, stream, cursor string) error {
	return mapError(s.conn.SetCursor(ctx, conndb.SetCursorParams{ConnectionID: connectionID, Stream: stream, Cursor: cursor}))
}

func (s *Store) DeleteCursor(ctx context.Context, connectionID, stream string) error {
	return mapError(s.conn.DeleteCursor(ctx, conndb.DeleteCursorParams{ConnectionID: connectionID, Stream: stream}))
}

func (s *Store) Cursors(ctx context.Context, connectionIDs []string) ([]storage.SyncCursor, error) {
	return many(toCursor)(s.conn.ListCursors(ctx, connectionIDs))
}

func (s *Store) InternalAddresses(ctx context.Context, workspaceID string) ([]string, error) {
	addresses, err := s.conn.InternalAddresses(ctx, workspaceID)
	return addresses, mapError(err)
}
