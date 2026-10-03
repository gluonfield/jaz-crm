package records

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
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

// SourceOf is who writes for an actor: a person in their browser, or an agent.
func SourceOf(actor auth.Actor) Source {
	if actor.Agent {
		return SourceAgent
	}
	return SourceUser
}

func (s Source) rank() int {
	return slices.Index([]Source{SourceSync, SourceAgent, SourceUser}, s)
}

// Write sets and removes values on one record: the record with RecordID,
// else the record holding a unique value being set, else a new one. Set adds
// to multi-valued attributes and replaces single-valued ones; Remove drops
// the listed values, or every value when the list is empty. Expect names the
// value single-valued attributes must still hold, empty for none, so an
// editor cannot overwrite a change it has not seen.
type Write struct {
	Object   string
	RecordID string
	Set      map[string][]string
	Remove   map[string][]string
	Expect   map[string]string
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
	// force replaces values whatever their source's rank.
	force bool
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
	// Documents are written whole by whoever edits them, and pages belong to
	// people and agents alike.
	for _, c := range [][]change{set, remove} {
		for i := range c {
			c[i].force = c[i].attr.Type == Markdown || object.Slug == Pages || object.Slug == "people" && c[i].attr.Slug == ContextAttribute
		}
	}
	expect := map[string]string{}
	for slug, value := range w.Expect {
		attr, err := sc.attribute(object, slug)
		if err != nil {
			return Record{}, nil, err
		}
		expect[attr.ID] = strings.TrimSpace(value)
	}
	id, err := s.target(ctx, actor.WorkspaceID, object, w.RecordID, set)
	if err != nil {
		return Record{}, nil, err
	}
	for _, c := range set {
		if id != "" && c.attr.Type == Reference && !c.attr.Multi && *c.attr.TargetObjectID == object.ID {
			if err := s.acyclic(ctx, actor.WorkspaceID, c.attr, id, *c.entries[0].ref); err != nil {
				return Record{}, nil, err
			}
		}
	}
	if id == "" && !slices.ContainsFunc(set, func(c change) bool { return len(c.entries) > 0 }) {
		return Record{}, nil, errs.Invalidf("a new %s record needs at least one value", object.Slug)
	}
	if id == "" {
		set, err = s.defaults(ctx, actor, sc.attributes(object.ID), set)
		if err != nil {
			return Record{}, nil, err
		}
	}
	var skips []Skip
	id, err = s.store.WriteRecord(ctx, actor.WorkspaceID, object.ID, id, func(current []storage.RecordValue) (storage.ValueChanges, error) {
		for attrID, value := range expect {
			held := ""
			if i := slices.IndexFunc(current, func(v storage.RecordValue) bool { return v.AttributeID == attrID }); i >= 0 {
				held = valueIdentity(current[i])
			}
			if held != value {
				return storage.ValueChanges{}, errs.Invalidf("%s changed since it was read", sc.attributeByID(attrID).Slug)
			}
		}
		set, remove, blocked, err := guardFollowUp(sc, object, current, slices.Clone(set), slices.Clone(remove), source)
		if err != nil {
			return storage.ValueChanges{}, err
		}
		var changes storage.ValueChanges
		changes, skips = plan(current, set, remove, source, actor.UserID)
		skips = append(skips, blocked...)
		return changes, nil
	})
	if errors.Is(err, storage.ErrNotFound) {
		return Record{}, nil, errs.Invalidf("no %s record %q", object.Slug, w.RecordID)
	}
	if err != nil {
		return Record{}, nil, err
	}
	record, err := s.get(ctx, actor.WorkspaceID, sc, id)
	return record, skips, err
}

// acyclic refuses a parent that is the record itself or one of its
// descendants, through a single-valued reference from an object to itself
// such as a page's parent.
func (s *Service) acyclic(ctx context.Context, workspaceID string, attr storage.Attribute, id, parent string) error {
	for at, depth := parent, 0; at != ""; depth++ {
		if at == id || depth > 100 {
			return errs.Invalidf("%s: a record cannot sit inside itself", attr.Slug)
		}
		values, err := s.store.CurrentValues(ctx, workspaceID, []string{at})
		if err != nil {
			return err
		}
		at = ""
		for _, v := range values {
			if v.AttributeID == attr.ID {
				at = *v.RefRecordID
			}
		}
	}
	return nil
}

// defaults fills what a new record is not given: each status starts in its
// first stage, and each member attribute names whoever creates the record.
func (s *Service) defaults(ctx context.Context, actor auth.Actor, attributes []storage.Attribute, set []change) ([]change, error) {
	var creator *string
	for _, attr := range attributes {
		if slices.ContainsFunc(set, func(c change) bool { return c.attr.ID == attr.ID }) {
			continue
		}
		switch {
		case attr.Type == Status:
			set = append(set, change{attr: attr, entries: []entry{{text: &attr.Options[0]}}})
		case attr.Type == Member && actor.UserID != "":
			if creator == nil {
				users, err := s.store.Users(ctx, actor.WorkspaceID)
				if err != nil {
					return nil, err
				}
				i := slices.IndexFunc(users, func(u storage.User) bool { return u.ID == actor.UserID })
				if i < 0 {
					continue
				}
				email := strings.ToLower(users[i].Email)
				creator = &email
			}
			set = append(set, change{attr: attr, entries: []entry{{text: creator}}})
		}
	}
	return set, nil
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
			return nil, errs.Invalidf("%s holds one value; remove it to clear it", slug)
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
				return "", errs.Invalidf("a unique value being set already belongs to %s record %s", object.Slug, owner)
			}
		}
		return id, nil
	}
	if len(owners) > 1 {
		return "", errs.Invalidf("the unique values match several %s records: %s", object.Slug, strings.Join(owners, ", "))
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
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	insert := func(attr storage.Attribute, e entry) {
		out.Insert = append(out.Insert, storage.NewRecordValue{
			AttributeID: attr.ID, Text: e.text, RefRecordID: e.ref, UniqueKey: e.key, Source: string(source), ActorID: actor,
		})
	}
	// release closes a value unless a higher-ranked source holds it.
	release := func(c change, v storage.RecordValue) bool {
		if !c.force && Source(v.Source).rank() > source.rank() {
			skips = append(skips, Skip{Attribute: c.attr.Slug, Value: valueIdentity(v), Source: Source(v.Source)})
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
			case c.attr.Type == Markdown && len(existing) == 1 && revising(existing[0], source, actorID):
				out.Revise = append(out.Revise, storage.ValueRevision{ID: existing[0].ID, Text: *e.text})
			case c.attr.Multi || len(existing) == 0 || release(c, existing[0]):
				insert(c.attr, e)
			}
		}
	}
	for _, c := range remove {
		for _, v := range held[c.attr.ID] {
			if len(c.entries) == 0 || slices.ContainsFunc(c.entries, func(e entry) bool { return e.identity() == valueIdentity(v) }) {
				release(c, v)
			}
		}
	}
	return out, skips
}

// revisionWindow is how long one writer's edits to a document make one
// version of it.
const revisionWindow = 10 * time.Minute

// revising reports whether a document's current version is one the member
// writing is still editing.
func revising(v storage.RecordValue, source Source, actorID string) bool {
	return Source(v.Source) == source && v.ActorID != nil && *v.ActorID == actorID && time.Since(v.ActiveFrom) < revisionWindow
}
