package interactions

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func (s *Service) write(ctx context.Context, workspaceID string, fn func(*Service) error) error {
	return s.store.Atomically(ctx, func(store storage.InteractionStore) error {
		if err := store.LockWorkspace(ctx, workspaceID); err != nil {
			return err
		}
		tx := *s
		tx.store = store
		tx.records = records.NewService(store)
		return fn(&tx)
	})
}
