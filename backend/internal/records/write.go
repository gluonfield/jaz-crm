package records

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// Source is who wrote a value. A write replaces or removes only values from
// its own source or a lower-ranked one, so sync never overwrites an agent and
// neither overwrites a person.
type Source string

const (
	SourceSync  Source = "sync"
	SourceAgent Source = "agent"
	SourceUser  Source = "user"
)

func (s Source) rank() int {
	return slices.Index([]Source{SourceSync, SourceAgent, SourceUser}, s)
}

// Write sets and removes values on one record: the record with RecordID,
// else the record holding a unique value being set, else a new one. Set adds
// to multi-valued attributes and replaces single-valued ones; Remove drops
// the listed values, or every value when the list is empty.
type Write struct {
	Object   string
	RecordID string
	Set      map[string][]string
	Remove   map[string][]string
}

// Skip is a change left out because a higher-ranked source holds the value.
type Skip struct {
	Attribute string
	Value     string
	Source    Source
}

type change struct {
	attr    storage.Attribute
	entries []entry
}

func (s *Service) Upsert(ctx context.Context, actor auth.Actor, source Source, w Write) (Record, []Skip, error) {
	record, skips, err := s.upsert(ctx, actor, source, w)
	if errors.Is(err, storage.ErrConflict) {
		// A concurrent write took a unique value first; match it this time.
		record, skips, err = s.upsert(ctx, actor, source, w)
	}
	return record, skips, err
}

func (s *Service) upsert(ctx context.Context, actor auth.Actor, source Source, w Write) (Record, []Skip, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return Record{}, nil, err
	}
	object, err := sc.object(w.Object)
	if err != nil {
		return Record{}, nil, err
	}
	set, err := s.changes(ctx, actor.WorkspaceID, sc, object, w.Set, true)
	if err != nil {
		return Record{}, nil, err
	}
	remove, err := s.changes(ctx, actor.WorkspaceID, sc, object, w.Remove, false)
	if err != nil {
		return Record{}, nil, err
	}
	id, err := s.target(ctx, actor.WorkspaceID, object, w.RecordID, set)
	if err != nil {
		return Record{}, nil, err
	}
	if id == "" && !slices.ContainsFunc(set, func(c change) bool { return len(c.entries) > 0 }) {
		return Record{}, nil, invalid("a new %s record needs at least one value", object.Slug)
	}
	var skips []Skip
	id, err = s.store.WriteRecord(ctx, actor.WorkspaceID, object.ID, id, func(current []storage.RecordValue) (storage.ValueChanges, error) {
		var changes storage.ValueChanges
		changes, skips = plan(current, set, remove, source, actor.UserID)
		return changes, nil
	})
	if errors.Is(err, storage.ErrNotFound) {
		return Record{}, nil, invalid("no %s record %q", object.Slug, w.RecordID)
	}
	if err != nil {
		return Record{}, nil, err
	}
	record, err := s.get(ctx, actor.WorkspaceID, sc, id)
	return record, skips, err
}

// changes validates raw values by attribute, in a stable order.
func (s *Service) changes(ctx context.Context, workspaceID string, sc schema, object storage.Object, raw map[string][]string, setting bool) ([]change, error) {
	slugs := make([]string, 0, len(raw))
	for slug := range raw {
		slugs = append(slugs, slug)
	}
	slices.Sort(slugs)
	var out []change
	for _, slug := range slugs {
		attr, err := sc.attribute(object, slug)
		if err != nil {
			return nil, err
		}
		if setting && !attr.Multi && len(raw[slug]) != 1 {
			return nil, invalid("%s holds one value; remove it to clear it", slug)
		}
		c := change{attr: attr}
		for _, value := range raw[slug] {
			e, err := s.entry(ctx, workspaceID, sc, attr, value)
			if err != nil {
				return nil, err
			}
			if !slices.ContainsFunc(c.entries, func(other entry) bool { return other.identity() == e.identity() }) {
				c.entries = append(c.entries, e)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// target picks the record to write: the one named, else the one holding a
// unique value being set, else none for a new record.
func (s *Service) target(ctx context.Context, workspaceID string, object storage.Object, id string, set []change) (string, error) {
	var attrIDs, keys []string
	for _, c := range set {
		for _, e := range c.entries {
			if e.key != nil {
				attrIDs = append(attrIDs, c.attr.ID)
				keys = append(keys, *e.key)
			}
		}
	}
	owners, err := s.owners(ctx, workspaceID, attrIDs, keys)
	if err != nil {
		return "", err
	}
	if id != "" {
		for _, owner := range owners {
			if owner != id {
				return "", invalid("a unique value being set already belongs to %s record %s", object.Slug, owner)
			}
		}
		return id, nil
	}
	if len(owners) > 1 {
		return "", invalid("the unique values match several %s records: %s", object.Slug, strings.Join(owners, ", "))
	}
	if len(owners) == 1 {
		return owners[0], nil
	}
	return "", nil
}

// plan turns a write into value changes against the record's current values.
func plan(current []storage.RecordValue, set, remove []change, source Source, actorID string) (storage.ValueChanges, []Skip) {
	var out storage.ValueChanges
	var skips []Skip
	held := map[string][]storage.RecordValue{}
	for _, v := range current {
		held[v.AttributeID] = append(held[v.AttributeID], v)
	}
	insert := func(attr storage.Attribute, e entry) {
		out.Insert = append(out.Insert, storage.NewRecordValue{
			AttributeID: attr.ID, Text: e.text, RefRecordID: e.ref, UniqueKey: e.key, Source: string(source), ActorID: &actorID,
		})
	}
	// release closes a value unless a higher-ranked source holds it.
	release := func(attr storage.Attribute, v storage.RecordValue) bool {
		if Source(v.Source).rank() > source.rank() {
			skips = append(skips, Skip{Attribute: attr.Slug, Value: valueIdentity(v), Source: Source(v.Source)})
			return false
		}
		out.Close = append(out.Close, v.ID)
		return true
	}
	for _, c := range set {
		existing := held[c.attr.ID]
		for _, e := range c.entries {
			i := slices.IndexFunc(existing, func(v storage.RecordValue) bool { return valueIdentity(v) == e.identity() })
			switch {
			case i >= 0 && Source(existing[i].Source).rank() >= source.rank():
			case i >= 0:
				out.Close = append(out.Close, existing[i].ID)
				insert(c.attr, e)
			case c.attr.Multi || len(existing) == 0 || release(c.attr, existing[0]):
				insert(c.attr, e)
			}
		}
	}
	for _, c := range remove {
		for _, v := range held[c.attr.ID] {
			if len(c.entries) == 0 || slices.ContainsFunc(c.entries, func(e entry) bool { return e.identity() == valueIdentity(v) }) {
				release(c.attr, v)
			}
		}
	}
	return out, skips
}
