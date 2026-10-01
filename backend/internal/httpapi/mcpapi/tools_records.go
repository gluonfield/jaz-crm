package mcpapi

import (
	"context"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerRecords(r *registry, crm *records.Service, conversations *interactions.Service, pics pictures) {
	add(r, &mcp.Tool{Name: "edit_pipeline_stage", Title: "Edit pipeline stage",
		Description: "Rename, move or delete a status stage. Move places it before another stage, or last when before is omitted. Delete requires a replacement when records use the stage, and moves them there. At least one stage remains; history is preserved."},
		func(ctx context.Context, actor auth.Actor, in stageInput) (empty, error) {
			return empty{}, crm.EditStage(ctx, actor, in.Object, in.Attribute, records.StageEdit{Action: in.Action, Stage: in.Stage, Name: in.Name, Before: in.Before, Replacement: in.Replacement})
		})
	add(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "add_attribute_option", Title: "Add attribute option",
		Description: "Add a reusable choice to a select attribute or append a pipeline stage to a status attribute. Returns the existing choice when its spelling differs only in case. Assign choices through upsert_record."},
		func(ctx context.Context, actor auth.Actor, in optionInput) (optionOutput, error) {
			value, err := crm.AddOption(ctx, actor, in.Object, in.Attribute, in.Value)
			return optionOutput{Value: value}, err
		})
	add(r, &mcp.Tool{Name: "list_objects", Title: "List objects", Annotations: readOnly,
		Description: "List the objects records belong to, such as people and companies, with their attributes."},
		func(ctx context.Context, actor auth.Actor, _ empty) (objectsOutput, error) {
			objects, err := crm.Objects(ctx, actor)
			return objectsOutput{Objects: objectViews(objects)}, err
		})
	add(r, &mcp.Tool{Name: "search_records", Title: "Search records", Annotations: readOnly,
		Description: "List an object's records, newest first, filtered by text and attribute values, with how often and when last each was in touch. Returns a filtered resource_uri that renders the matching CRM list or deal pipeline, and opens it automatically. Records, and the records they reference, carry a picture: a person's profile picture when Google has one, else the website logo of a record with a domain.",
		Meta:        mcp.Meta{"ui": map[string]any{"resourceUri": appURI}}},
		func(ctx context.Context, actor auth.Actor, in searchInput) (recordsOutput, error) {
			found, err := crm.Search(ctx, actor, records.Search{Object: in.Object, Query: in.Query, Where: in.Where, Limit: in.Limit})
			if err != nil {
				return recordsOutput{}, err
			}
			ids := make([]string, len(found))
			for i, record := range found {
				ids[i] = record.ID
			}
			activity, err := conversations.Activities(ctx, actor, ids)
			if err != nil {
				return recordsOutput{}, err
			}
			photos, err := pics.of(ctx, actor, found)
			out := recordsOutput{Records: []recordView{}, ResourceURI: recordSearchURI(in)}
			for _, record := range found {
				out.Records = append(out.Records, recordWith(record, activity, photos))
			}
			return out, err
		})
	add(r, &mcp.Tool{Name: "get_record", Title: "Get record", Annotations: readOnly,
		Description: "Get a person, company, deal or custom record by record_id with its current values and communication activity; upcoming meetings do not count. Returns a resource_uri that automatically renders a compact record card. Clicking the card opens the full CRM record.",
		Meta:        mcp.Meta{"ui": map[string]any{"resourceUri": appURI}, "ui/resourceUri": appURI}},
		func(ctx context.Context, actor auth.Actor, in recordInput) (recordOutput, error) {
			record, err := crm.Get(ctx, actor, in.RecordID)
			if err != nil {
				return recordOutput{}, err
			}
			activity, err := conversations.Activities(ctx, actor, []string{record.ID})
			if err != nil {
				return recordOutput{}, err
			}
			photos, err := pics.of(ctx, actor, []records.Record{record})
			return recordOutput{recordView: recordWith(record, activity, photos), ResourceURI: "ui://jaz-crm/r/" + record.ID}, err
		})
	add(r, &mcp.Tool{Name: "record_history", Title: "Record history", Annotations: readOnly,
		Description: "A record's recent changes, newest first: the value each attribute got, from which source (user, agent or sync), who made it and when; removed marks a value taken away with nothing in its place."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (historyOutput, error) {
			changes, err := crm.History(ctx, actor, in.RecordID)
			out := historyOutput{Changes: []changeView{}}
			for _, c := range changes {
				out.Changes = append(out.Changes, changeView{
					Attribute: c.Attribute, Value: c.Value.Text, RecordID: c.Value.RecordID, Removed: c.Removed, Source: string(c.Source), Actor: c.Actor, At: c.At,
				})
			}
			return out, err
		})
	add(r, &mcp.Tool{Name: "upsert_record", Title: "Upsert record",
		Description: "Create or update a record. Without record_id it updates the record holding a given email, domain or phone number, else creates one."},
		func(ctx context.Context, actor auth.Actor, in upsertInput) (upsertOutput, error) {
			set, err := lists(in.Values)
			if err != nil {
				return upsertOutput{}, err
			}
			record, skips, err := crm.Upsert(ctx, actor, records.SourceOf(actor), records.Write{Object: in.Object, RecordID: in.RecordID, Set: set, Remove: in.Remove})
			if err != nil {
				return upsertOutput{}, err
			}
			out := upsertOutput{Record: recordOf(record, nil), Skipped: []skipView{}}
			for _, s := range skips {
				out.Skipped = append(out.Skipped, skipView{Attribute: s.Attribute, Value: s.Value, SetBy: string(s.Source)})
			}
			return out, nil
		})
	add(r, &mcp.Tool{Name: "delete_record", Title: "Delete record",
		Description: "Delete a record with its values and links to interactions. A deleted person's addresses are skipped in triage, so sync does not add them back."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (empty, error) {
			return empty{}, conversations.Delete(ctx, actor, in.RecordID)
		})
	add(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "create_object", Title: "Create object",
		Description: "Create a record type, such as quotes or suppliers. It starts with a name attribute."},
		func(ctx context.Context, actor auth.Actor, in objectInput) (objectView, error) {
			object, err := crm.CreateObject(ctx, actor, in.Slug, in.Name)
			return objectOf(object), err
		})
	add(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "create_attribute", Title: "Create attribute",
		Description: "Add an attribute to an object. Types: text, number, date, checkbox, url, select (with options), status (with its stages in order as options; a new record starts in the first), member (a workspace member by email; a new record names its creator), email, domain, phone, reference (with target)."},
		func(ctx context.Context, actor auth.Actor, in attributeInput) (objectView, error) {
			object, err := crm.CreateAttribute(ctx, actor, in.Object, records.Attribute{
				Slug: in.Slug, Name: in.Name, Type: in.Type, Multi: in.Multi, Unique: in.Unique, Target: in.Target, Options: in.Options,
			})
			return objectOf(object), err
		})
}

type stageInput struct {
	Object      string `json:"object"`
	Attribute   string `json:"attribute"`
	Action      string `json:"action" jsonschema:"rename, move or delete"`
	Stage       string `json:"stage"`
	Name        string `json:"name,omitempty"`
	Before      string `json:"before,omitempty"`
	Replacement string `json:"replacement,omitempty"`
}

type optionInput struct {
	Object    string `json:"object"`
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
}

type optionOutput struct {
	Value string `json:"value"`
}

type attributeView struct {
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Multi   bool     `json:"multi,omitempty"`
	Unique  bool     `json:"unique,omitempty"`
	Target  string   `json:"target,omitempty"`
	Options []string `json:"options,omitempty"`
}

type objectView struct {
	Slug       string          `json:"slug"`
	Name       string          `json:"name"`
	Attributes []attributeView `json:"attributes"`
}

type objectsOutput struct {
	Objects []objectView `json:"objects"`
}

func objectOf(o records.Object) objectView {
	view := objectView{Slug: o.Slug, Name: o.Name, Attributes: []attributeView{}}
	for _, a := range o.Attributes {
		view.Attributes = append(view.Attributes, attributeView(a))
	}
	return view
}

func objectViews(objects []records.Object) []objectView {
	out := []objectView{}
	for _, o := range objects {
		out = append(out, objectOf(o))
	}
	return out
}

type refView struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Photo string `json:"photo,omitempty"`
}

type recordView struct {
	ID        string                 `json:"id"`
	Object    string                 `json:"object"`
	CreatedAt time.Time              `json:"created_at"`
	Values    map[string]any         `json:"values"`
	Activity  *interactions.Activity `json:"activity,omitempty"`
	// Photo is a person's profile picture from one of their addresses.
	Photo string `json:"photo,omitempty"`
}

// recordWith shows a record with its activity and picture.
func recordWith(r records.Record, activity map[string]interactions.Activity, photos map[string]string) recordView {
	view := recordOf(r, photos)
	a := activity[r.ID]
	view.Activity = &a
	return view
}

// recordOf shows a single-valued attribute as its value and a multi-valued
// one as a list; references appear as the record's id, name and picture.
func recordOf(r records.Record, photos map[string]string) recordView {
	values := map[string]any{}
	for _, f := range r.Fields {
		var list []any
		for _, v := range f.Values {
			if v.RecordID != "" {
				list = append(list, refView{ID: v.RecordID, Name: v.Text, Photo: photos[v.RecordID]})
			} else {
				list = append(list, v.Text)
			}
		}
		if f.Multi {
			values[f.Attribute] = list
		} else {
			values[f.Attribute] = list[0]
		}
	}
	return recordView{ID: r.ID, Object: r.Object, CreatedAt: r.CreatedAt, Values: values, Photo: photos[r.ID]}
}

type searchInput struct {
	Object string            `json:"object" jsonschema:"object slug, such as people or companies"`
	Query  string            `json:"query,omitempty" jsonschema:"text that any value contains, case-insensitively"`
	Where  map[string]string `json:"where,omitempty" jsonschema:"attribute slug to a value the record must hold; a reference takes a record id or a unique value such as a domain"`
	Limit  int               `json:"limit,omitempty" jsonschema:"at most 100, default 20"`
}

type recordsOutput struct {
	Records     []recordView `json:"records"`
	ResourceURI string       `json:"resource_uri"`
}

type recordInput struct {
	RecordID string `json:"record_id"`
}

type recordOutput struct {
	recordView
	ResourceURI string `json:"resource_uri"`
}

type upsertInput struct {
	Object   string              `json:"object" jsonschema:"object slug, such as people or companies"`
	RecordID string              `json:"record_id,omitempty" jsonschema:"the record to update; omit to match by unique values or create"`
	Values   map[string]any      `json:"values,omitempty" jsonschema:"attribute slug to a value or list of values; multi-valued attributes gain them, others are replaced; a reference takes a record id or a unique value such as a domain"`
	Remove   map[string][]string `json:"remove,omitempty" jsonschema:"attribute slug to values to remove; an empty list removes every value"`
}

type skipView struct {
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
	SetBy     string `json:"set_by"`
}

type upsertOutput struct {
	Record  recordView `json:"record"`
	Skipped []skipView `json:"skipped"`
}

// lists reads values that are strings or lists of strings.
func lists(values map[string]any) (map[string][]string, error) {
	out := map[string][]string{}
	for slug, raw := range values {
		switch v := raw.(type) {
		case string:
			out[slug] = []string{v}
		case []any:
			out[slug] = []string{}
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return nil, errs.Invalidf("values must be strings or lists of strings")
				}
				out[slug] = append(out[slug], s)
			}
		default:
			return nil, errs.Invalidf("values must be strings or lists of strings")
		}
	}
	return out, nil
}

type objectInput struct {
	Slug string `json:"slug" jsonschema:"lowercase identifier, such as quotes"`
	Name string `json:"name" jsonschema:"display name, such as Quotes"`
}

type attributeInput struct {
	Object  string   `json:"object"`
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Multi   bool     `json:"multi,omitempty"`
	Unique  bool     `json:"unique,omitempty" jsonschema:"values identify a record, like an email"`
	Target  string   `json:"target,omitempty" jsonschema:"object slug a reference points at"`
	Options []string `json:"options,omitempty" jsonschema:"allowed values of a select, or a status's stages in order"`
}

type changeView struct {
	Attribute string    `json:"attribute"`
	Value     string    `json:"value"`
	RecordID  string    `json:"record_id,omitempty"`
	Removed   bool      `json:"removed,omitempty"`
	Source    string    `json:"source,omitempty"`
	Actor     string    `json:"actor,omitempty"`
	At        time.Time `json:"at"`
}

type historyOutput struct {
	Changes []changeView `json:"changes"`
}
