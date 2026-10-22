package records

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gluonfield/jaz-crm/backend/crm/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/lithammer/shortuuid/v4"
)

func (s *Service) SavedFilters(ctx context.Context, actor Actor, slug string) ([]SavedFilter, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	object, err := sc.object(slug)
	if err != nil {
		return nil, err
	}
	filters, err := s.store.SavedFilters(ctx, actor.WorkspaceID, object.ID, actor.MemberID)
	if err != nil {
		return nil, err
	}
	filters = slices.DeleteFunc(filters, func(filter storage.SavedFilter) bool {
		for _, condition := range filter.Filters {
			if _, err := sc.attribute(object, condition.Attribute, false); err != nil {
				return true
			}
		}
		return false
	})
	order := map[string]int{}
	for _, standard := range StandardObjects {
		if standard.Slug == slug {
			for i, filter := range standard.Filters {
				order[strings.ToLower(filter.Name)] = i - len(standard.Filters)
			}
		}
	}
	slices.SortStableFunc(filters, func(a, b storage.SavedFilter) int {
		return cmp.Compare(order[strings.ToLower(a.Name)], order[strings.ToLower(b.Name)])
	})
	if slug == FollowUps {
		users, err := s.store.Users(ctx, actor.WorkspaceID)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(users, func(user storage.User) bool { return user.ID == actor.MemberID })
		if i < 0 {
			return nil, errs.Invalidf("Follow Ups views belong to a member of the workspace")
		}
		open := Filter{Attribute: "status", Operator: "is", Value: "Open"}
		filters = append([]storage.SavedFilter{
			{ID: "everyone", Name: "Everyone", Builtin: true, Filters: []Filter{open}},
			{ID: "assigned_to_me", Name: "Assigned to me", Builtin: true, Filters: []Filter{open, {Attribute: "owner", Operator: "is", Value: users[i].Email}}},
		}, filters...)
	}
	return filters, nil
}

func (s *Service) SaveFilter(ctx context.Context, actor Actor, slug string, filter SavedFilter) (SavedFilter, error) {
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
	if slug == FollowUps {
		if strings.EqualFold(filter.Name, "Everyone") || strings.EqualFold(filter.Name, "Assigned to me") {
			return filter, errs.Invalidf("this name belongs to a built-in Follow Ups view")
		}
		filter.UserID = &actor.MemberID
	}
	filter.ObjectID = object.ID
	if filter.Filters == nil {
		filter.Filters = []Filter{}
	}
	saved, err := s.store.SaveFilter(ctx, actor.WorkspaceID, actor.MemberID, filter)
	if errors.Is(err, storage.ErrConflict) {
		return filter, errs.Invalidf("a filter with this name already exists")
	}
	if errors.Is(err, storage.ErrNotFound) {
		return filter, errs.Invalidf("saved filter not found")
	}
	return saved, err
}

func (s *Service) DeleteFilter(ctx context.Context, actor Actor, id string) error {
	err := s.store.DeleteFilter(ctx, actor.WorkspaceID, id, actor.MemberID)
	if errors.Is(err, storage.ErrNotFound) {
		return errs.Invalidf("saved filter not found")
	}
	return err
}
