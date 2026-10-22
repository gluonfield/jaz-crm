package records

import (
	"context"
	"errors"

	"github.com/gluonfield/jaz-crm/backend/crm/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func (s *Service) Trash(ctx context.Context, actor Actor) ([]storage.TrashedRecord, error) {
	return s.store.TrashRecords(ctx, actor.WorkspaceID)
}

func (s *Service) Restore(ctx context.Context, actor Actor, id string) error {
	err := s.store.RestoreRecord(ctx, actor.WorkspaceID, id)
	if errors.Is(err, storage.ErrNotFound) {
		return errs.Invalidf("record not found in Trash")
	}
	return err
}
