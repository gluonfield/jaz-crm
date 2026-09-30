package postgres

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	authdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/auth"
	recdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/records"
)

func (s *Store) EditStatus(ctx context.Context, workspaceID, attributeID string, mutate storage.StatusMutation) error {
	return s.tx(ctx, func(_ *authdb.Queries, r *recdb.Queries) error {
		attr, err := r.LockStatus(ctx, recdb.LockStatusParams{WorkspaceID: workspaceID, ID: attributeID})
		if err != nil {
			return err
		}
		change, err := mutate(attr.Options)
		if err != nil {
			return err
		}
		if change.From != "" && change.To == "" {
			used, err := r.StageInUse(ctx, recdb.StageInUseParams{AttributeID: attributeID, Text: &change.From})
			if err != nil {
				return err
			}
			if used {
				return storage.ErrConflict
			}
		}
		if change.From != "" && change.To != "" {
			if err := r.ReplaceStageValues(ctx, recdb.ReplaceStageValuesParams{AttributeID: attributeID, FromStage: &change.From, ToStage: &change.To, ActorID: change.ActorID}); err != nil {
				return err
			}
		}
		return r.UpdateStatusOptions(ctx, recdb.UpdateStatusOptionsParams{ID: attributeID, Options: change.Options})
	})
}
