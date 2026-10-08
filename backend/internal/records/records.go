// Package records owns the CRM data model: objects, their typed attributes,
// and records whose values keep their history and the source that set them.
package records

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
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
	Standard   bool
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
	// Options are a select's allowed values, or a status's stages in order.
	Options   []string
	Archived  bool
	Protected bool
}

type Record struct {
	ID             string
	ConversationID string
	Object         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Fields         []Field
	Related        map[string][]Value
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
	Icon     string
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

func (sc schema) attributes(objectID string, includeArchived bool) []storage.Attribute {
	var out []storage.Attribute
	for _, a := range sc.attrs {
		if a.ObjectID == objectID && (includeArchived || !a.Archived) {
			out = append(out, a)
		}
	}
	return out
}

func (sc schema) attribute(object storage.Object, slug string, includeArchived bool) (storage.Attribute, error) {
	i := slices.IndexFunc(sc.attrs, func(a storage.Attribute) bool { return a.ObjectID == object.ID && a.Slug == slug })
	if i < 0 {
		return storage.Attribute{}, errs.Invalidf("%s has no attribute %q; call list_objects for its attributes", object.Slug, slug)
	}
	if sc.attrs[i].Archived && !includeArchived {
		return storage.Attribute{}, errs.Invalidf("%s.%s is archived; restore it in Settings → Schema to use it", object.Slug, slug)
	}
	return sc.attrs[i], nil
}

func (sc schema) attributeByID(id string) storage.Attribute {
	i := slices.IndexFunc(sc.attrs, func(a storage.Attribute) bool { return a.ID == id })
	return sc.attrs[i]
}

// Objects describes the workspace's objects and their attributes.
func (s *Service) Objects(ctx context.Context, actor auth.Actor, includeArchived bool) ([]Object, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []Object{}
	for _, o := range sc.objects {
		view := Object{Slug: o.Slug, Name: o.Name, Standard: standard(o.Slug)}
		for _, a := range sc.attributes(o.ID, includeArchived) {
			attr := Attribute{Slug: a.Slug, Name: a.Name, Type: a.Type, Multi: a.Multi, Unique: a.IsUnique, Options: a.Options, Archived: a.Archived, Protected: protected(o.Slug, a)}
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
	records, err := s.store.Records(ctx, workspaceID, []string{id})
	if err != nil {
		return Record{}, err
	}
	if len(records) == 0 {
		return Record{}, errs.Invalidf("no record %q", id)
	}
	views, err := s.views(ctx, workspaceID, sc, records, true)
	if err != nil {
		return Record{}, err
	}
	return views[0], nil
}

// entry validates a raw value, resolving a reference to its record.
func (s *Service) entry(ctx context.Context, workspaceID string, sc schema, attr storage.Attribute, raw string) (entry, error) {
	switch attr.Type {
	case Reference:
		id, err := s.resolve(ctx, workspaceID, sc, attr, strings.TrimSpace(raw))
		return entry{ref: &id}, err
	case Member:
		return s.member(ctx, workspaceID, attr, raw)
	}
	e, err := normalize(attr, raw)
	if err == nil && sc.textValue(attr, *e.text).Icon != "" && (strings.HasPrefix(*e.text, "data:") || strings.HasPrefix(*e.text, "image:")) {
		return s.pageImage(ctx, workspaceID, *e.text)
	}
	return e, err
}

// member names a member of the workspace by their email.
func (s *Service) member(ctx context.Context, workspaceID string, attr storage.Attribute, raw string) (entry, error) {
	e, err := normalize(storage.Attribute{Slug: attr.Slug, Type: Email}, raw)
	if err != nil {
		return entry{}, err
	}
	users, err := s.store.Users(ctx, workspaceID)
	if err != nil {
		return entry{}, err
	}
	if !slices.ContainsFunc(users, func(u storage.User) bool { return strings.EqualFold(u.Email, *e.text) }) {
		return entry{}, errs.Invalidf("%s: %s is not a member of this workspace", attr.Slug, *e.text)
	}
	return e, nil
}

// resolve finds the record a reference names: its id, or a unique value of
// the target object such as a company's domain.
func (s *Service) resolve(ctx context.Context, workspaceID string, sc schema, attr storage.Attribute, raw string) (string, error) {
	target := sc.objectByID(*attr.TargetObjectID)
	records, err := s.store.Records(ctx, workspaceID, []string{raw})
	if err != nil {
		return "", err
	}
	if len(records) == 1 && records[0].ObjectID == target.ID {
		return raw, nil
	}
	var attrIDs, keys []string
	for _, a := range sc.attributes(target.ID, false) {
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

// views loads records' current values, naming referenced records; only a
// full view carries documents.
func (s *Service) views(ctx context.Context, workspaceID string, sc schema, records []storage.Record, full bool) ([]Record, error) {
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
	labels, err := s.labels(ctx, workspaceID, sc, refs)
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(records))
	for i, r := range records {
		view := Record{ID: r.ID, Object: sc.objectByID(r.ObjectID).Slug, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Fields: []Field{}}
		for _, attr := range sc.attributes(r.ObjectID, false) {
			if attr.Type == Markdown && !full {
				continue
			}
			field := Field{Attribute: attr.Slug, Multi: attr.Multi}
			for _, v := range held[[2]string{r.ID, attr.ID}] {
				if v.RefRecordID != nil {
					if label, active := labels[*v.RefRecordID]; active {
						field.Values = append(field.Values, label)
					}
				} else {
					field.Values = append(field.Values, sc.textValue(attr, *v.Text))
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

// Label names a record for display elsewhere.
type Label struct {
	Object string
	Name   string
}

// Labels names records of the workspace; ids of other workspaces are left out.
func (s *Service) Labels(ctx context.Context, workspaceID string, ids []string) (map[string]Label, error) {
	out := map[string]Label{}
	if len(ids) == 0 {
		return out, nil
	}
	sc, err := s.schema(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	records, err := s.store.Records(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	labels, err := s.labels(ctx, workspaceID, sc, ids)
	for _, r := range records {
		out[r.ID] = Label{Object: sc.objectByID(r.ObjectID).Slug, Name: labels[r.ID].Text}
	}
	return out, err
}

// labels names records by their title, else an identifying value such as an
// email address, else any text; page icons travel with their labels.
func (s *Service) labels(ctx context.Context, workspaceID string, sc schema, ids []string) (map[string]Value, error) {
	out := map[string]Value{}
	if len(ids) == 0 {
		return out, nil
	}
	records, err := s.store.Records(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		out[record.ID] = Value{RecordID: record.ID}
	}
	values, err := s.store.CurrentValues(ctx, workspaceID, ids)
	rank := func(a storage.Attribute) int {
		switch {
		case a.Slug == titleAttribute:
			return 0
		case a.IsUnique:
			return 1
		}
		return 2
	}
	selected := map[string]storage.Attribute{}
	for _, v := range values {
		if _, active := out[v.RecordID]; !active || v.Text == nil {
			continue
		}
		attr := sc.attributeByID(v.AttributeID)
		label := out[v.RecordID]
		if value := sc.textValue(attr, *v.Text); value.Icon != "" {
			label.Icon = value.Icon
			out[v.RecordID] = label
			continue
		}
		was, found := selected[v.RecordID]
		if !found || rank(attr) < rank(was) || rank(attr) == rank(was) && attr.Slug < was.Slug {
			label.Text = *v.Text
			out[v.RecordID] = label
			selected[v.RecordID] = attr
		}
	}
	return out, err
}

func (sc schema) textValue(attr storage.Attribute, text string) Value {
	value := Value{Text: text}
	if attr.Slug == "icon" && sc.objectByID(attr.ObjectID).Slug == Pages {
		value.Icon = text
	}
	return value
}
