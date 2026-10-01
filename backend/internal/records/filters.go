package records

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/lithammer/shortuuid/v4"
)

func (s *Service) SavedFilters(ctx context.Context, actor auth.Actor, slug string) ([]storage.SavedFilter, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	object, err := sc.object(slug)
	if err != nil {
		return nil, err
	}
	return s.store.SavedFilters(ctx, actor.WorkspaceID, object.ID)
}

func (s *Service) SaveFilter(ctx context.Context, actor auth.Actor, slug string, filter storage.SavedFilter) (storage.SavedFilter, error) {
	filter.Name = strings.TrimSpace(filter.Name)
	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Name == "" || utf8.RuneCountInString(filter.Name) > 100 {
		return filter, errs.Invalidf("filter name must have 1–100 characters")
	}
	if filter.ID != "" {
		if _, err := shortuuid.DefaultEncoder.Decode(filter.ID); err != nil {
			return filter, errs.Invalidf("invalid saved filter id")
		}
	}
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return filter, err
	}
	object, err := sc.object(slug)
	if err != nil {
		return filter, err
	}
	if _, err := s.filterQuery(ctx, actor, sc, object, filter.Filters); err != nil {
		return filter, err
	}
	filter.ObjectID = object.ID
	if filter.Filters == nil {
		filter.Filters = []Filter{}
	}
	saved, err := s.store.SaveFilter(ctx, actor.WorkspaceID, filter)
	if errors.Is(err, storage.ErrConflict) {
		return filter, errs.Invalidf("a filter with this name already exists")
	}
	if errors.Is(err, storage.ErrNotFound) {
		return filter, errs.Invalidf("saved filter not found")
	}
	return saved, err
}

func (s *Service) DeleteFilter(ctx context.Context, actor auth.Actor, id string) error {
	err := s.store.DeleteFilter(ctx, actor.WorkspaceID, id)
	if errors.Is(err, storage.ErrNotFound) {
		return errs.Invalidf("saved filter not found")
	}
	return err
}
