// Package records owns the CRM data model: objects, their typed attributes,
// and records whose values keep their history and the source that set them.
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
	"github.com/google/uuid"
)

type Service struct {
	store storage.RecordStore
}

func NewService(store storage.RecordStore) *Service {
	return &Service{store: store}
}

type Object struct {
	Slug       string
	Name       string
	Attributes []Attribute
}

type Attribute struct {
	Slug   string
	Name   string
	Type   string
	Multi  bool
	Unique bool
	// Target is the object a reference points at.
	Target string
	// Options are a select attribute's allowed values.
	Options []string
}

type Record struct {
	ID        string
	Object    string
	CreatedAt time.Time
	Fields    []Field
}

// Field is an attribute's current values; a single-valued one has one.
type Field struct {
	Attribute string
	Multi     bool
	Values    []Value
}

// Value is text, or a referenced record with its name as Text.
type Value struct {
	Text     string
	RecordID string
}

// schema is one workspace's objects and attributes.
type schema struct {
	objects []storage.Object
	attrs   []storage.Attribute
}

func (s *Service) schema(ctx context.Context, workspaceID string) (schema, error) {
	objects, err := s.store.Objects(ctx, workspaceID)
	if err != nil {
		return schema{}, err
	}
	attrs, err := s.store.Attributes(ctx, workspaceID)
	return schema{objects: objects, attrs: attrs}, err
}

func (sc schema) object(slug string) (storage.Object, error) {
	i := slices.IndexFunc(sc.objects, func(o storage.Object) bool { return o.Slug == slug })
	if i < 0 {
		slugs := []string{}
		for _, o := range sc.objects {
			slugs = append(slugs, o.Slug)
		}
		return storage.Object{}, errs.Invalidf("no object %q; objects are %s", slug, strings.Join(slugs, ", "))
	}
	return sc.objects[i], nil
}

func (sc schema) objectByID(id string) storage.Object {
	i := slices.IndexFunc(sc.objects, func(o storage.Object) bool { return o.ID == id })
	return sc.objects[i]
}

func (sc schema) attributes(objectID string) []storage.Attribute {
	var out []storage.Attribute
	for _, a := range sc.attrs {
		if a.ObjectID == objectID {
			out = append(out, a)
		}
	}
	return out
}

func (sc schema) attribute(object storage.Object, slug string) (storage.Attribute, error) {
	i := slices.IndexFunc(sc.attrs, func(a storage.Attribute) bool { return a.ObjectID == object.ID && a.Slug == slug })
	if i < 0 {
		return storage.Attribute{}, errs.Invalidf("%s has no attribute %q; call list_objects for its attributes", object.Slug, slug)
	}
	return sc.attrs[i], nil
}

func (sc schema) attributeByID(id string) storage.Attribute {
	i := slices.IndexFunc(sc.attrs, func(a storage.Attribute) bool { return a.ID == id })
	return sc.attrs[i]
}

// Objects describes the workspace's objects and their attributes.
func (s *Service) Objects(ctx context.Context, actor auth.Actor) ([]Object, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []Object{}
	for _, o := range sc.objects {
		view := Object{Slug: o.Slug, Name: o.Name}
		for _, a := range sc.attributes(o.ID) {
			attr := Attribute{Slug: a.Slug, Name: a.Name, Type: a.Type, Multi: a.Multi, Unique: a.IsUnique, Options: a.Options}
			if a.TargetObjectID != nil {
				attr.Target = sc.objectByID(*a.TargetObjectID).Slug
			}
			view.Attributes = append(view.Attributes, attr)
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (Record, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return Record{}, err
	}
	return s.get(ctx, actor.WorkspaceID, sc, id)
}

func (s *Service) get(ctx context.Context, workspaceID string, sc schema, id string) (Record, error) {
	records, err := s.find(ctx, workspaceID, id)
	if err != nil {
		return Record{}, err
	}
	if len(records) == 0 {
		return Record{}, errs.Invalidf("no record %q", id)
	}
	views, err := s.views(ctx, workspaceID, sc, records)
	if err != nil {
		return Record{}, err
	}
	return views[0], nil
}

// find loads a record by id; anything but a UUID names no record.
func (s *Service) find(ctx context.Context, workspaceID, id string) ([]storage.Record, error) {
	if uuid.Validate(id) != nil {
		return nil, nil
	}
	return s.store.Records(ctx, workspaceID, []string{id})
}

type Search struct {
	Object string
	// Query matches any text value containing it, case-insensitively.
	Query string
	// Where requires attribute values: text case-insensitively, emails,
	// domains and phones by their normalized form, references by record id
	// or the target's unique value.
	Where map[string]string
	Limit int
}

// Search lists an object's records, newest first.
func (s *Service) Search(ctx context.Context, actor auth.Actor, q Search) ([]Record, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	object, err := sc.object(q.Object)
	if err != nil {
		return nil, err
	}
	query := storage.RecordQuery{WorkspaceID: actor.WorkspaceID, ObjectID: object.ID, AttributeIDs: []string{}, Matches: []string{}, Limit: 20}
	if q.Limit > 0 {
		query.Limit = int32(min(q.Limit, 100))
	}
	if text := strings.TrimSpace(q.Query); text != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text)
		query.Query = &escaped
	}
	for slug, raw := range q.Where {
		attr, err := sc.attribute(object, slug)
		if err != nil {
			return nil, err
		}
		e, err := s.entry(ctx, actor.WorkspaceID, sc, attr, raw)
		var unknown errs.Invalid
		if attr.Type == Reference && errors.As(err, &unknown) {
			return []Record{}, nil
		}
		if err != nil {
			return nil, err
		}
		query.AttributeIDs = append(query.AttributeIDs, attr.ID)
		query.Matches = append(query.Matches, e.match())
	}
	records, err := s.store.SearchRecords(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, actor.WorkspaceID, sc, records)
}

// entry validates a raw value, resolving a reference to its record.
func (s *Service) entry(ctx context.Context, workspaceID string, sc schema, attr storage.Attribute, raw string) (entry, error) {
	if attr.Type != Reference {
		return normalize(attr, raw)
	}
	id, err := s.resolve(ctx, workspaceID, sc, attr, strings.TrimSpace(raw))
	return entry{ref: &id}, err
}

// resolve finds the record a reference names: its id, or a unique value of
// the target object such as a company's domain.
func (s *Service) resolve(ctx context.Context, workspaceID string, sc schema, attr storage.Attribute, raw string) (string, error) {
	target := sc.objectByID(*attr.TargetObjectID)
	records, err := s.find(ctx, workspaceID, raw)
	if err != nil {
		return "", err
	}
	if len(records) == 1 && records[0].ObjectID == target.ID {
		return raw, nil
	}
	var attrIDs, keys []string
	for _, a := range sc.attributes(target.ID) {
		if !a.IsUnique {
			continue
		}
		if e, err := normalize(a, raw); err == nil {
			attrIDs = append(attrIDs, a.ID)
			keys = append(keys, *e.key)
		}
	}
	owners, err := s.owners(ctx, workspaceID, attrIDs, keys)
	if err != nil {
		return "", err
	}
	if len(owners) != 1 {
		return "", errs.Invalidf("%s: %q names no single %s record; give its id or a unique value such as a domain", attr.Slug, raw, target.Slug)
	}
	return owners[0], nil
}

// owners lists the records holding the (attribute, unique key) pairs.
func (s *Service) owners(ctx context.Context, workspaceID string, attrIDs, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	return s.store.RecordsByUniqueKeys(ctx, workspaceID, attrIDs, keys)
}

// views loads records' current values, naming referenced records.
func (s *Service) views(ctx context.Context, workspaceID string, sc schema, records []storage.Record) ([]Record, error) {
	ids := make([]string, len(records))
	for i, r := range records {
		ids[i] = r.ID
	}
	values, err := s.store.CurrentValues(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	var refs []string
	held := map[[2]string][]storage.RecordValue{}
	for _, v := range values {
		key := [2]string{v.RecordID, v.AttributeID}
		held[key] = append(held[key], v)
		if v.RefRecordID != nil {
			refs = append(refs, *v.RefRecordID)
		}
	}
	names, err := s.names(ctx, workspaceID, sc, refs)
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(records))
	for i, r := range records {
		view := Record{ID: r.ID, Object: sc.objectByID(r.ObjectID).Slug, CreatedAt: r.CreatedAt, Fields: []Field{}}
		for _, attr := range sc.attributes(r.ObjectID) {
			field := Field{Attribute: attr.Slug, Multi: attr.Multi}
			for _, v := range held[[2]string{r.ID, attr.ID}] {
				if v.RefRecordID != nil {
					field.Values = append(field.Values, Value{RecordID: *v.RefRecordID, Text: names[*v.RefRecordID]})
				} else {
					field.Values = append(field.Values, Value{Text: *v.Text})
				}
			}
			if len(field.Values) > 0 {
				view.Fields = append(view.Fields, field)
			}
		}
		out[i] = view
	}
	return out, nil
}

// names maps records to their title attribute.
func (s *Service) names(ctx context.Context, workspaceID string, sc schema, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	values, err := s.store.CurrentValues(ctx, workspaceID, ids)
	for _, v := range values {
		if v.Text != nil && sc.attributeByID(v.AttributeID).Slug == titleAttribute {
			out[v.RecordID] = *v.Text
		}
	}
	return out, err
}
