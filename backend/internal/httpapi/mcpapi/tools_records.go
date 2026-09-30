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

func registerRecords(r *registry, crm *records.Service, conversations *interactions.Service) {
	add(r, &mcp.Tool{Name: "list_objects", Title: "List objects", Annotations: readOnly,
		Description: "List the objects records belong to, such as people and companies, with their attributes."},
		func(ctx context.Context, actor auth.Actor, _ empty) (objectsOutput, error) {
			objects, err := crm.Objects(ctx, actor)
			return objectsOutput{Objects: objectViews(objects)}, err
		})
	add(r, &mcp.Tool{Name: "search_records", Title: "Search records", Annotations: readOnly,
		Description: "List an object's records, newest first, filtered by text and attribute values."},
		func(ctx context.Context, actor auth.Actor, in searchInput) (recordsOutput, error) {
			found, err := crm.Search(ctx, actor, records.Search{Object: in.Object, Query: in.Query, Where: in.Where, Limit: in.Limit})
			out := recordsOutput{Records: []recordView{}}
			for _, record := range found {
				out.Records = append(out.Records, recordOf(record))
			}
			return out, err
		})
	add(r, &mcp.Tool{Name: "get_record", Title: "Get record", Annotations: readOnly,
		Description: "Get one record with its current values and how often, and when last, it was in touch."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (recordView, error) {
			record, err := crm.Get(ctx, actor, in.RecordID)
			if err != nil {
				return recordView{}, err
			}
			activity, err := conversations.Activity(ctx, actor, record.ID)
			view := recordOf(record)
			view.Activity = &activityView{Interactions: activity.Interactions, LastAt: activity.LastAt}
			return view, err
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
			out := upsertOutput{Record: recordOf(record), Skipped: []skipView{}}
			for _, s := range skips {
				out.Skipped = append(out.Skipped, skipView{Attribute: s.Attribute, Value: s.Value, SetBy: string(s.Source)})
			}
			return out, nil
		})
	add(r, &mcp.Tool{Name: "delete_record", Title: "Delete record",
		Description: "Delete a record with its values and links to interactions."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (empty, error) {
			return empty{}, crm.Delete(ctx, actor, in.RecordID)
		})
	add(r, &mcp.Tool{Name: "create_object", Title: "Create object",
		Description: "Create a record type, such as deals or suppliers. It starts with a name attribute."},
		func(ctx context.Context, actor auth.Actor, in objectInput) (objectView, error) {
			object, err := crm.CreateObject(ctx, actor, in.Slug, in.Name)
			return objectOf(object), err
		})
	add(r, &mcp.Tool{Name: "create_attribute", Title: "Create attribute",
		Description: "Add an attribute to an object. Types: text, number, date, checkbox, url, select (with options), email, domain, phone, reference (with target)."},
		func(ctx context.Context, actor auth.Actor, in attributeInput) (objectView, error) {
			object, err := crm.CreateAttribute(ctx, actor, in.Object, records.Attribute{
				Slug: in.Slug, Name: in.Name, Type: in.Type, Multi: in.Multi, Unique: in.Unique, Target: in.Target, Options: in.Options,
			})
			return objectOf(object), err
		})
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
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type recordView struct {
	ID        string         `json:"id"`
	Object    string         `json:"object"`
	CreatedAt time.Time      `json:"created_at"`
	Values    map[string]any `json:"values"`
	Activity  *activityView  `json:"activity,omitempty"`
}

type activityView struct {
	Interactions int        `json:"interactions"`
	LastAt       *time.Time `json:"last_at,omitempty"`
}

// recordOf shows a single-valued attribute as its value and a multi-valued
// one as a list; references appear as the record's id and name.
func recordOf(r records.Record) recordView {
	values := map[string]any{}
	for _, f := range r.Fields {
		var list []any
		for _, v := range f.Values {
			if v.RecordID != "" {
				list = append(list, refView{ID: v.RecordID, Name: v.Text})
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
	return recordView{ID: r.ID, Object: r.Object, CreatedAt: r.CreatedAt, Values: values}
}

type searchInput struct {
	Object string            `json:"object" jsonschema:"object slug, such as people or companies"`
	Query  string            `json:"query,omitempty" jsonschema:"text that any value contains, case-insensitively"`
	Where  map[string]string `json:"where,omitempty" jsonschema:"attribute slug to a value the record must hold; a reference takes a record id or a unique value such as a domain"`
	Limit  int               `json:"limit,omitempty" jsonschema:"at most 100, default 20"`
}

type recordsOutput struct {
	Records []recordView `json:"records"`
}

type recordInput struct {
	RecordID string `json:"record_id"`
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
	Slug string `json:"slug" jsonschema:"lowercase identifier, such as deals"`
	Name string `json:"name" jsonschema:"display name, such as Deals"`
}

type attributeInput struct {
	Object  string   `json:"object"`
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Multi   bool     `json:"multi,omitempty"`
	Unique  bool     `json:"unique,omitempty" jsonschema:"values identify a record, like an email"`
	Target  string   `json:"target,omitempty" jsonschema:"object slug a reference points at"`
	Options []string `json:"options,omitempty" jsonschema:"allowed values of a select, such as pipeline stages"`
}
