package postgres

import (
	"context"
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	authdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/auth"
	intdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/interactions"
	recdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/records"
	"github.com/jackc/pgx/v5"
)

func toObject(r recdb.Object) storage.Object          { return storage.Object(r) }
func toAttribute(r recdb.Attribute) storage.Attribute { return storage.Attribute(r) }
func toRecord(r recdb.Record) storage.Record          { return storage.Record(r) }
func toValue(r recdb.RecordValue) storage.RecordValue { return storage.RecordValue(r) }

func (s *Store) Objects(ctx context.Context, workspaceID string) ([]storage.Object, error) {
	return many(toObject)(s.rec.ListObjects(ctx, workspaceID))
}

func (s *Store) Attributes(ctx context.Context, workspaceID string) ([]storage.Attribute, error) {
	return many(toAttribute)(s.rec.ListAttributes(ctx, workspaceID))
}

func (s *Store) CreateObject(ctx context.Context, workspaceID string, object storage.NewObject) error {
	return s.tx(ctx, func(_ *authdb.Queries, r *recdb.Queries) error {
		return createObjects(ctx, r, workspaceID, []storage.NewObject{object})
	})
}

func (s *Store) CreateAttribute(ctx context.Context, attr storage.AttributeInput) error {
	_, err := s.rec.CreateAttribute(ctx, recdb.CreateAttributeParams(attr))
	return mapError(err)
}

func (s *Store) AddAttributeOption(ctx context.Context, workspaceID, attributeID, value string) (string, error) {
	option, err := s.rec.AddAttributeOption(ctx, recdb.AddAttributeOptionParams{WorkspaceID: workspaceID, AttributeID: attributeID, Value: value})
	return option, mapError(err)
}

func (s *Store) DeleteRecord(ctx context.Context, workspaceID, id string) error {
	return mapError(pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := s.auth.WithTx(tx).LockWorkspace(ctx, workspaceID); err != nil {
			return err
		}
		r := s.rec.WithTx(tx)
		q := s.in.WithTx(tx)
		if _, err := r.LockRecordForDeletion(ctx, recdb.LockRecordForDeletionParams{WorkspaceID: workspaceID, ID: id}); err != nil {
			return err
		}
		handles, err := q.SkipRecordHandles(ctx, intdb.SkipRecordHandlesParams{WorkspaceID: workspaceID, RecordID: &id})
		if err != nil {
			return err
		}
		ids, err := q.InteractionsOfHandles(ctx, handles)
		if err != nil {
			return err
		}
		if err := affected(r.DeleteRecord(ctx, recdb.DeleteRecordParams{WorkspaceID: workspaceID, ID: id})); err != nil {
			return err
		}
		return relink(ctx, q, ids)
	}))
}

func (s *Store) Records(ctx context.Context, workspaceID string, ids []string) ([]storage.Record, error) {
	return many(toRecord)(s.rec.GetRecords(ctx, recdb.GetRecordsParams{WorkspaceID: workspaceID, IDs: ids}))
}

func (s *Store) SearchRecords(ctx context.Context, query storage.RecordQuery) ([]storage.Record, error) {
	return many(toRecord)(s.rec.SearchRecords(ctx, recdb.SearchRecordsParams(query)))
}

func (s *Store) RelatedRecords(ctx context.Context, workspaceID, attributeID string, ids []string, limit int32) ([]storage.RelatedRecord, error) {
	return many(func(r recdb.RelatedRecordsRow) storage.RelatedRecord {
		return storage.RelatedRecord(r)
	})(s.rec.RelatedRecords(ctx, recdb.RelatedRecordsParams{WorkspaceID: workspaceID, AttributeID: attributeID, IDs: ids, Limit: limit}))
}

func (s *Store) History(ctx context.Context, workspaceID, recordID string, limit int32) ([]storage.PastValue, error) {
	return many(func(r recdb.RecordHistoryRow) storage.PastValue {
		return storage.PastValue{RecordValue: toValue(r.RecordValue), ActorName: r.ActorName}
	})(s.rec.RecordHistory(ctx, recdb.RecordHistoryParams{WorkspaceID: workspaceID, RecordID: recordID, Limit: limit}))
}

func (s *Store) CurrentValues(ctx context.Context, workspaceID string, recordIDs []string) ([]storage.RecordValue, error) {
	return many(toValue)(s.rec.CurrentValues(ctx, recdb.CurrentValuesParams{WorkspaceID: workspaceID, RecordIDs: recordIDs}))
}

func (s *Store) RecordsByUniqueKeys(ctx context.Context, workspaceID string, attributeIDs, keys []string) ([]string, error) {
	ids, err := s.rec.RecordsByUniqueKeys(ctx, recdb.RecordsByUniqueKeysParams{WorkspaceID: workspaceID, AttributeIDs: attributeIDs, UniqueKeys: keys})
	return ids, mapError(err)
}

func (s *Store) WriteRecord(ctx context.Context, workspaceID, objectID, id string, mutate storage.RecordMutation) (string, error) {
	err := s.tx(ctx, func(_ *authdb.Queries, r *recdb.Queries) error {
		statuses, err := r.LockObjectStatuses(ctx, recdb.LockObjectStatusesParams{WorkspaceID: workspaceID, ObjectID: objectID})
		if err != nil {
			return err
		}
		var record recdb.Record
		if id == "" {
			record, err = r.CreateRecord(ctx, recdb.CreateRecordParams{WorkspaceID: workspaceID, ObjectID: objectID})
		} else {
			record, err = r.LockRecord(ctx, recdb.LockRecordParams{WorkspaceID: workspaceID, ObjectID: objectID, ID: id})
		}
		if err != nil {
			return err
		}
		id = record.ID
		current, err := r.CurrentValues(ctx, recdb.CurrentValuesParams{WorkspaceID: workspaceID, RecordIDs: []string{id}})
		if err != nil {
			return err
		}
		values := make([]storage.RecordValue, len(current))
		for i, v := range current {
			values[i] = toValue(v)
		}
		changes, err := mutate(values)
		if err != nil {
			return err
		}
		for _, value := range changes.Insert {
			for _, status := range statuses {
				if value.AttributeID == status.ID && (value.Text == nil || !slices.Contains(status.Options, *value.Text)) {
					return storage.ErrConflict
				}
			}
		}
		if err := r.CloseValues(ctx, recdb.CloseValuesParams{RecordID: id, IDs: changes.Close}); err != nil {
			return err
		}
		for _, value := range changes.Insert {
			value.RecordID = id
			if err := r.InsertValue(ctx, recdb.InsertValueParams(value)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}
