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
	registerFilters(r, crm)
	registerDates(r, crm)
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
		Description: "List the objects records belong to with their attributes: the standard people, companies, deals, follow-ups and pages, and the workspace's own tables, whose records are pages too. Pages nest under a parent page."},
		func(ctx context.Context, actor auth.Actor, _ empty) (objectsOutput, error) {
			objects, err := crm.Objects(ctx, actor)
			return objectsOutput{Objects: objectViews(objects)}, err
		})
	add(r, &mcp.Tool{Name: "search_records", Title: "Search records", Annotations: readOnly,
		Description: "List an object's records, newest first or by a date, filtered by text and attribute values, with how often and when last each was in touch and the start of its latest message. Returns a filtered resource_uri that renders the matching CRM list or deal pipeline, and opens it automatically. Records, and the records they reference, carry a picture: a person's profile picture when Google has one, else the website logo of a record with a domain. Listed records leave out their markdown content, which query still matches and get_record returns.",
		Meta:        mcp.Meta{"ui": map[string]any{"resourceUri": appURI}}},
		func(ctx context.Context, actor auth.Actor, in searchInput) (recordsOutput, error) {
			found, total, err := crm.Search(ctx, actor, records.Search{Object: in.Object, Query: in.Query, Where: in.Where, Filters: in.Filters, Sort: in.Sort, Offset: in.Offset, Limit: in.Limit, Include: in.Include})
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
			out := recordsOutput{Records: []recordView{}, Total: total, ResourceURI: recordSearchURI(in)}
			for _, record := range found {
				out.Records = append(out.Records, recordWith(record, activity, photos))
			}
			return out, err
		})
	add(r, &mcp.Tool{Name: "get_record", Title: "Get record", Annotations: readOnly,
		Description: "Get a person, company, deal, page or table record by record_id with its current values, including a page's markdown content, communication activity, and what references it, such as its follow-ups or a page's sub-pages, as compact references in related keyed by object.attribute, up to 20 each; upcoming meetings do not count. Returns a resource_uri that automatically renders a compact record card. Clicking the card opens the full CRM record.",
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
			record, err = crm.Referenced(ctx, actor, record, 20)
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
		Description: "Create or update a record. Without record_id it updates the record holding a given email, domain or phone number, else creates one. A person's context is the relationship's TLDR as bullet points of one short line each, who they are and how we know them, then dated events, newest first; write it whole, merged from the current context and what you learned, since every write replaces it. For email follow-ups, put the email subject in subject and only the body in draft; use channel Email and known addresses in to and cc. With no linked email conversation it starts a new email; a linked email supplies the reply subject when subject is empty. A follow-up's draft_status moves from Draft to Approved, which only a person sets in the CRM, then to Sending, which claims an approved draft for whoever sends it and succeeds once, then to Sent; editing the draft withdraws its approval. content is the markdown body of a page and of every record of the workspace's own tables: headings, lists, task lists, tables, quotes, code, links, bold and italic; write it whole. Mention a person, company or page with a link to its record, such as [Acme](/r/<record_id>). A page nests under another through parent; without one it is top level. To edit a document someone may be changing, pass expect with the content you read."},
		func(ctx context.Context, actor auth.Actor, in upsertInput) (upsertOutput, error) {
			set, err := lists(in.Values)
			if err != nil {
				return upsertOutput{}, err
			}
			expect := map[string][]string{}
			for slug, value := range in.Expect {
				expect[slug] = []string{value}
			}
			record, skips, err := crm.Upsert(ctx, actor, records.SourceOf(actor), records.Write{Object: in.Object, RecordID: in.RecordID, Set: set, Remove: in.Remove, Expect: expect})
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
		Description: "Delete a record with its values and links to interactions. A deleted person's addresses and a deleted company's domains are skipped in triage, so sync does not add them back."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (empty, error) {
			return empty{}, crm.Delete(ctx, actor, in.RecordID)
		})
	add(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "create_object", Title: "Create table",
		Description: "Create a table: a record type, such as painpoints or suppliers, whose records are pages. It starts with a name and markdown content; add columns with create_attribute."},
		func(ctx context.Context, actor auth.Actor, in objectInput) (objectView, error) {
			object, err := crm.CreateObject(ctx, actor, in.Slug, in.Name)
			return objectOf(object), err
		})
	add(r, &mcp.Tool{Name: "edit_object", Title: "Edit table",
		Description: "Rename or delete a table the workspace created. Delete removes its records and every reference attribute pointing at it."},
		func(ctx context.Context, actor auth.Actor, in objectEdit) (empty, error) {
			return empty{}, crm.EditObject(ctx, actor, in.Object, in.Action, in.Name)
		})
	add(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "create_attribute", Title: "Create attribute",
		Description: "Add an attribute to an object, such as a table's column. Types: text, number, date, datetime (date with optional time), checkbox, url, select (with options), status (with its stages in order as options; a new record starts in the first), member (a workspace member by email; a new record names its creator), email, domain, phone, reference (with target, such as people, companies or pages)."},
		func(ctx context.Context, actor auth.Actor, in attributeInput) (objectView, error) {
			object, err := crm.CreateAttribute(ctx, actor, in.Object, records.Attribute{
				Slug: in.Slug, Name: in.Name, Type: in.Type, Multi: in.Multi, Unique: in.Unique, Target: in.Target, Options: in.Options,
			})
			return objectOf(object), err
		})
	add(r, &mcp.Tool{Name: "edit_attribute", Title: "Edit attribute",
		Description: "Rename an attribute, or delete one the workspace added with its values and its conditions in saved filters. Built-in attributes, such as name and content, stay."},
		func(ctx context.Context, actor auth.Actor, in attributeEdit) (empty, error) {
			return empty{}, crm.EditAttribute(ctx, actor, in.Object, in.Attribute, in.Action, in.Name)
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
	Standard   bool            `json:"standard,omitempty"`
	Attributes []attributeView `json:"attributes"`
}

type objectsOutput struct {
	Objects []objectView `json:"objects"`
}

func objectOf(o records.Object) objectView {
	view := objectView{Slug: o.Slug, Name: o.Name, Standard: o.Standard, Attributes: []attributeView{}}
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
	Related   map[string][]refView   `json:"related,omitempty"`
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
	related := map[string][]refView{}
	for relation, records := range r.Related {
		list := make([]refView, len(records))
		for i, record := range records {
			list[i] = refView{ID: record.RecordID, Name: record.Text, Photo: photos[record.RecordID]}
		}
		related[relation] = list
	}
	return recordView{ID: r.ID, Object: r.Object, CreatedAt: r.CreatedAt, Values: values, Related: related, Photo: photos[r.ID]}
}

type searchInput struct {
	Object  string             `json:"object" jsonschema:"object slug, such as people or companies"`
	Query   string             `json:"query,omitempty" jsonschema:"text that any value contains, case-insensitively"`
	Where   map[string]string  `json:"where,omitempty" jsonschema:"attribute slug to a value the record must hold; a reference takes a record id or a unique value such as a domain"`
	Filters []records.Filter   `json:"filters,omitempty" jsonschema:"multiple attribute conditions; every condition must match, including repeated attributes"`
	Sort    string             `json:"sort,omitempty" jsonschema:"a date or text attribute to order by, earliest or first alphabetically and empty last, such as action_date or name; omit for newest first"`
	Offset  int                `json:"offset,omitempty" jsonschema:"matching records to skip, for the next page"`
	Limit   int                `json:"limit,omitempty" jsonschema:"at most 500, default 20"`
	Include []records.Relation `json:"include,omitempty" jsonschema:"at most 8 reverse relationships to include as compact references, keyed by object.attribute in each record's related field"`
}

type recordsOutput struct {
	Records []recordView `json:"records"`
	// Total counts every matching record, beyond this page.
	Total       int    `json:"total"`
	ResourceURI string `json:"resource_uri"`
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
	Expect   map[string]string   `json:"expect,omitempty" jsonschema:"attribute slug to the value it must still hold, empty for none; the write fails if someone changed it"`
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

type objectEdit struct {
	Object string `json:"object"`
	Action string `json:"action" jsonschema:"rename or delete"`
	Name   string `json:"name,omitempty"`
}

type attributeEdit struct {
	Object    string `json:"object"`
	Attribute string `json:"attribute"`
	Action    string `json:"action" jsonschema:"rename or delete"`
	Name      string `json:"name,omitempty"`
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
