package postgres

import (
	"context"
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	authdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/auth"
	intdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/interactions"
	recdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/records"
	"github.com/jackc/pgx/v5"
	"github.com/lithammer/shortuuid/v4"
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
	return s.tx(ctx, func(_ *authdb.Queries, r *recdb.Queries) error {
		created, err := r.CreateAttribute(ctx, recdb.CreateAttributeParams{ID: shortuuid.New(),
			ObjectID: attr.ObjectID, Slug: attr.Slug, Name: attr.Name, Type: attr.Type, Multi: attr.Multi, IsUnique: attr.IsUnique, TargetObjectID: attr.TargetObjectID, Options: attr.Options,
		})
		if err != nil || attr.Start == nil {
			return err
		}
		return r.StartRecords(ctx, recdb.StartRecordsParams{AttributeID: created.ID, Text: attr.Start.Text, Source: attr.Start.Source, ActorID: attr.Start.ActorID, ObjectID: attr.ObjectID})
	})
}

func (s *Store) RenameObject(ctx context.Context, workspaceID, id, name string) error {
	return affected(s.rec.RenameObject(ctx, recdb.RenameObjectParams{Name: name, WorkspaceID: workspaceID, ID: id}))
}

func (s *Store) DeleteObject(ctx context.Context, workspaceID, id string) error {
	return mapError(pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		q := s.in.WithTx(tx)
		r := s.rec.WithTx(tx)
		ids, err := q.InteractionsOfObject(ctx, intdb.InteractionsOfObjectParams{WorkspaceID: workspaceID, ObjectID: id})
		if err != nil {
			return err
		}
		attrs, err := r.ListAttributes(ctx, workspaceID)
		if err != nil {
			return err
		}
		var references []string
		for _, attr := range attrs {
			if attr.TargetObjectID != nil && *attr.TargetObjectID == id && attr.ObjectID != id {
				references = append(references, attr.ID)
			}
		}
		if err := r.DropFilterConditions(ctx, recdb.DropFilterConditionsParams{AttributeIDs: references, WorkspaceID: workspaceID}); err != nil {
			return err
		}
		if err := affected(r.DeleteObject(ctx, recdb.DeleteObjectParams{WorkspaceID: workspaceID, ID: id})); err != nil {
			return err
		}
		return q.ClearUnlinkedContent(ctx, ids)
	}))
}

func (s *Store) RenameAttribute(ctx context.Context, workspaceID, id, name string) error {
	return affected(s.rec.RenameAttribute(ctx, recdb.RenameAttributeParams{Name: name, WorkspaceID: workspaceID, ID: id}))
}

func (s *Store) DeleteAttribute(ctx context.Context, workspaceID string, attr storage.Attribute) error {
	return s.tx(ctx, func(_ *authdb.Queries, r *recdb.Queries) error {
		if err := r.DropFilterConditions(ctx, recdb.DropFilterConditionsParams{AttributeIDs: []string{attr.ID}, WorkspaceID: workspaceID}); err != nil {
			return err
		}
		return affected(r.DeleteAttribute(ctx, recdb.DeleteAttributeParams{WorkspaceID: workspaceID, ID: attr.ID}))
	})
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

func (s *Store) SearchRecords(ctx context.Context, query storage.RecordQuery) ([]storage.SearchRecord, int, error) {
	params := recdb.SearchRecordsParams{
		WorkspaceID: query.WorkspaceID, ObjectID: query.ObjectID, Query: query.Query,
		AttributeIDs: query.AttributeIDs, Operators: query.Operators, Matches: query.Matches,
		SortAttributeID: query.SortAttributeID, Offset: query.Offset, Limit: query.Limit,
		SortUpdatedAt:       query.SortUpdatedAt,
		GroupByConversation: query.GroupByConversation, ConversationID: query.ConversationID,
	}
	rows, err := s.rec.SearchRecords(ctx, params)
	if err != nil {
		return nil, 0, mapError(err)
	}
	out := make([]storage.SearchRecord, len(rows))
	total := 0
	for i, row := range rows {
		out[i] = storage.SearchRecord{Record: toRecord(row.Record), ConversationID: row.ConversationID}
		total = int(row.Total)
	}
	if len(rows) == 0 && query.Offset > 0 {
		params.Offset, params.Limit = 0, 1
		first, err := s.rec.SearchRecords(ctx, params)
		if err != nil {
			return nil, 0, mapError(err)
		}
		if len(first) > 0 {
			total = int(first[0].Total)
		}
	}
	return out, total, nil
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

func (s *Store) PageImage(ctx context.Context, id string) (storage.PageImage, error) {
	return one(func(r recdb.PageImageRow) storage.PageImage {
		return storage.PageImage{ID: r.ID, WorkspaceID: r.WorkspaceID, PNG: r.PNG}
	})(s.rec.PageImage(ctx, id))
}

func (s *Store) WriteRecord(ctx context.Context, workspaceID, objectID, id string, mutate storage.RecordMutation) (string, error) {
	err := s.tx(ctx, func(_ *authdb.Queries, r *recdb.Queries) error {
		statuses, err := r.LockObjectStatuses(ctx, recdb.LockObjectStatusesParams{WorkspaceID: workspaceID, ObjectID: objectID})
		if err != nil {
			return err
		}
		var record recdb.Record
		if id == "" {
			record, err = r.CreateRecord(ctx, recdb.CreateRecordParams{ID: shortuuid.New(), WorkspaceID: workspaceID, ObjectID: objectID})
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
		for _, revision := range changes.Revise {
			if err := r.ReviseValue(ctx, recdb.ReviseValueParams{Text: &revision.Text, RecordID: id, ID: revision.ID}); err != nil {
				return err
			}
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
