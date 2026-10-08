package records

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

var types = []string{Text, Number, Date, DateTime, Checkbox, URL, Select, Status, Member, Email, Domain, Phone, Reference}

func validName(slug, name string) error {
	if !slugPattern.MatchString(slug) {
		return errs.Invalidf("slug %q must be lowercase letters, digits and underscores, starting with a letter", slug)
	}
	if name == "" || len([]rune(name)) > 80 {
		return errs.Invalidf("a name is 1 to 80 characters")
	}
	return nil
}

// CreateObject adds a table: a record type with a name and markdown content.
func (s *Service) CreateObject(ctx context.Context, actor auth.Actor, slug, name string) (Object, error) {
	name = strings.TrimSpace(name)
	if err := validName(slug, name); err != nil {
		return Object{}, err
	}
	err := s.store.CreateObject(ctx, actor.WorkspaceID, storage.NewObject{Slug: slug, Name: name, Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: ContentAttribute, Name: "Content", Type: Markdown},
	}})
	if errors.Is(err, storage.ErrConflict) {
		return Object{}, errs.Invalidf("object %q already exists", slug)
	}
	if err != nil {
		return Object{}, err
	}
	return s.object(ctx, actor, slug)
}

// CreateAttribute adds an attribute to an object.
func (s *Service) CreateAttribute(ctx context.Context, actor auth.Actor, object string, a Attribute) (Object, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return Object{}, err
	}
	o, err := sc.object(object)
	if err != nil {
		return Object{}, err
	}
	a.Name = strings.TrimSpace(a.Name)
	if err := validName(a.Slug, a.Name); err != nil {
		return Object{}, err
	}
	input := storage.AttributeInput{ObjectID: o.ID, Slug: a.Slug, Name: a.Name, Type: a.Type, Multi: a.Multi, IsUnique: a.Unique, Options: []string{}}
	switch {
	case !slices.Contains(types, a.Type):
		return Object{}, errs.Invalidf("type %q is not one of %s", a.Type, strings.Join(types, ", "))
	case a.Unique && !slices.Contains(uniqueTypes, a.Type):
		return Object{}, errs.Invalidf("only %s attributes can be unique", strings.Join(uniqueTypes, ", "))
	case a.Multi && (a.Type == Checkbox || a.Type == Status):
		return Object{}, errs.Invalidf("a %s holds one value", a.Type)
	case a.Type != Select && a.Type != Status && len(a.Options) > 0:
		return Object{}, errs.Invalidf("only select and status attributes list options")
	case a.Type == Status && len(a.Options) == 0:
		return Object{}, errs.Invalidf("a status lists its stages")
	case (a.Type == Reference) != (a.Target != ""):
		return Object{}, errs.Invalidf("reference attributes, and only they, name a target object")
	}
	for _, option := range a.Options {
		option = strings.TrimSpace(option)
		if option == "" || slices.ContainsFunc(input.Options, func(o string) bool { return strings.EqualFold(o, option) }) {
			return Object{}, errs.Invalidf("options must be distinct and non-empty")
		}
		input.Options = append(input.Options, option)
	}
	if a.Target != "" {
		target, err := sc.object(a.Target)
		if err != nil {
			return Object{}, err
		}
		input.TargetObjectID = &target.ID
	}
	if a.Type == Status {
		// Existing records start in the first stage, as new ones do.
		input.Start = &storage.NewRecordValue{Text: &input.Options[0], Source: string(SourceOf(actor))}
		if actor.UserID != "" {
			input.Start.ActorID = &actor.UserID
		}
	}
	err = s.store.CreateAttribute(ctx, input)
	if errors.Is(err, storage.ErrConflict) {
		return Object{}, errs.Invalidf("%s already has an attribute %q", object, a.Slug)
	}
	if err != nil {
		return Object{}, err
	}
	return s.object(ctx, actor, object)
}

func (s *Service) AddOption(ctx context.Context, actor auth.Actor, object, attribute, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 80 {
		return "", errs.Invalidf("an option is 1 to 80 characters")
	}
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return "", err
	}
	o, err := sc.object(object)
	if err != nil {
		return "", err
	}
	a, err := sc.attribute(o, attribute)
	if err != nil {
		return "", err
	}
	if a.Type != Select && a.Type != Status {
		return "", errs.Invalidf("%s is not a select or status attribute", attribute)
	}
	return s.store.AddAttributeOption(ctx, actor.WorkspaceID, a.ID, value)
}

func (s *Service) object(ctx context.Context, actor auth.Actor, slug string) (Object, error) {
	objects, err := s.Objects(ctx, actor)
	if err != nil {
		return Object{}, err
	}
	i := slices.IndexFunc(objects, func(o Object) bool { return o.Slug == slug })
	return objects[i], nil
}

// Delete moves a record to Trash, retaining its values and references.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, id string) error {
	records, err := s.store.Records(ctx, actor.WorkspaceID, []string{id})
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return errs.Invalidf("no record %q", id)
	}
	return s.store.DeleteRecord(ctx, actor.WorkspaceID, id)
}

// EditObject renames or deletes one of the workspace's own tables.
func (s *Service) EditObject(ctx context.Context, actor auth.Actor, object, action, name string) error {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return err
	}
	o, err := sc.object(object)
	if err != nil {
		return err
	}
	if standard(o.Slug) {
		return errs.Invalidf("%s is built in; only the workspace's own tables are renamed or deleted", o.Slug)
	}
	switch action {
	case "rename":
		name = strings.TrimSpace(name)
		if err := validName(o.Slug, name); err != nil {
			return err
		}
		return s.store.RenameObject(ctx, actor.WorkspaceID, o.ID, name)
	case "delete":
		return s.store.DeleteObject(ctx, actor.WorkspaceID, o.ID)
	}
	return errs.Invalidf("action is rename or delete")
}

// EditAttribute renames an attribute, or deletes one the workspace added.
func (s *Service) EditAttribute(ctx context.Context, actor auth.Actor, object, attribute, action, name string) error {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return err
	}
	o, err := sc.object(object)
	if err != nil {
		return err
	}
	a, err := sc.attribute(o, attribute)
	if err != nil {
		return err
	}
	switch action {
	case "rename":
		name = strings.TrimSpace(name)
		if err := validName(a.Slug, name); err != nil {
			return err
		}
		return s.store.RenameAttribute(ctx, actor.WorkspaceID, a.ID, name)
	case "delete":
		if a.Slug == titleAttribute || a.Type == Markdown || standard(o.Slug, a.Slug) {
			return errs.Invalidf("%s.%s is built in and stays", o.Slug, a.Slug)
		}
		return s.store.DeleteAttribute(ctx, actor.WorkspaceID, a)
	}
	return errs.Invalidf("action is rename or delete")
}
