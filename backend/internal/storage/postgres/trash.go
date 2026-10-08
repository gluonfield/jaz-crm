package postgres

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	recdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/records"
	"github.com/jackc/pgx/v5"
)

func (s *Store) TrashRecords(ctx context.Context, workspaceID string) ([]storage.TrashedRecord, error) {
	return many(func(r recdb.TrashRecordsRow) storage.TrashedRecord {
		return storage.TrashedRecord(r)
	})(s.rec.TrashRecords(ctx, workspaceID))
}

func (s *Store) RestoreContactRecords(ctx context.Context, workspaceID, address, domain string) error {
	return mapError(pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := s.auth.WithTx(tx).LockWorkspace(ctx, workspaceID); err != nil {
			return err
		}
		ids, err := s.rec.WithTx(tx).TrashedContactRecords(ctx, recdb.TrashedContactRecordsParams{WorkspaceID: workspaceID, Address: &address, Domain: &domain})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.restoreRecord(ctx, tx, workspaceID, id); err != nil {
				return err
			}
		}
		return nil
	}))
}

func (s *Store) RestoreRecord(ctx context.Context, workspaceID, id string) error {
	return mapError(pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := s.auth.WithTx(tx).LockWorkspace(ctx, workspaceID); err != nil {
			return err
		}
		return s.restoreRecord(ctx, tx, workspaceID, id)
	}))
}

func (s *Store) restoreRecord(ctx context.Context, tx pgx.Tx, workspaceID, id string) error {
	r := s.rec.WithTx(tx)
	if err := affected(r.RestoreRecord(ctx, recdb.RestoreRecordParams{WorkspaceID: workspaceID, ID: id})); err != nil {
		return err
	}
	if err := r.RestoreRecordDomains(ctx, recdb.RestoreRecordDomainsParams{WorkspaceID: workspaceID, RecordID: id}); err != nil {
		return err
	}
	handles, err := r.RestoreRecordHandles(ctx, recdb.RestoreRecordHandlesParams{WorkspaceID: workspaceID, RecordID: &id})
	if err != nil {
		return err
	}
	q := s.in.WithTx(tx)
	ids, err := q.InteractionsOfHandles(ctx, handles)
	if err != nil {
		return err
	}
	return relink(ctx, q, ids)
}
