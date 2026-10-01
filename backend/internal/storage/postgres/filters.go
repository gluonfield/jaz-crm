package postgres

import (
	"context"
	"encoding/json"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	recdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/records"
	"github.com/lithammer/shortuuid/v4"
)

func savedFilter(row recdb.SavedFilter) (storage.SavedFilter, error) {
	filter := storage.SavedFilter{ID: row.ID, ObjectID: row.ObjectID, Name: row.Name, Query: row.Query}
	err := json.Unmarshal(row.Filters, &filter.Filters)
	return filter, err
}

func (s *Store) SavedFilters(ctx context.Context, workspaceID, objectID string) ([]storage.SavedFilter, error) {
	rows, err := s.rec.ListSavedFilters(ctx, recdb.ListSavedFiltersParams{WorkspaceID: workspaceID, ObjectID: objectID})
	if err != nil {
		return nil, err
	}
	filters := []storage.SavedFilter{}
	for _, row := range rows {
		filter, err := savedFilter(row)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

func (s *Store) SaveFilter(ctx context.Context, workspaceID string, filter storage.SavedFilter) (storage.SavedFilter, error) {
	data, err := json.Marshal(filter.Filters)
	if err != nil {
		return filter, err
	}
	var row recdb.SavedFilter
	if filter.ID == "" {
		row, err = s.rec.CreateSavedFilter(ctx, recdb.CreateSavedFilterParams{ID: shortuuid.New(), WorkspaceID: workspaceID, ObjectID: filter.ObjectID, Name: filter.Name, Query: filter.Query, Filters: data})
	} else {
		row, err = s.rec.UpdateSavedFilter(ctx, recdb.UpdateSavedFilterParams{ID: filter.ID, WorkspaceID: workspaceID, ObjectID: filter.ObjectID, Name: filter.Name, Query: filter.Query, Filters: data})
	}
	if err != nil {
		return filter, mapError(err)
	}
	return savedFilter(row)
}

func (s *Store) DeleteFilter(ctx context.Context, workspaceID, id string) error {
	return affected(s.rec.DeleteFilter(ctx, recdb.DeleteFilterParams{WorkspaceID: workspaceID, ID: id}))
}
