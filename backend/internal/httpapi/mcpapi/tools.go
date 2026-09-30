package mcpapi

import (
	"context"
	"errors"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func register(server *mcp.Server, t tools) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, &mcp.Tool{Name: "list_objects", Title: "List objects", Annotations: readOnly,
		Description: "List the objects records belong to, such as people and companies, with their attributes."}, t.listObjects)
	mcp.AddTool(server, &mcp.Tool{Name: "search_records", Title: "Search records", Annotations: readOnly,
		Description: "List an object's records, newest first, filtered by text and attribute values."}, t.searchRecords)
	mcp.AddTool(server, &mcp.Tool{Name: "get_record", Title: "Get record", Annotations: readOnly,
		Description: "Get one record with its current values."}, t.getRecord)
	mcp.AddTool(server, &mcp.Tool{Name: "upsert_record", Title: "Upsert record",
		Description: "Create or update a record. Without record_id it updates the record holding a given email, domain or phone number, else creates one."}, t.upsertRecord)
	mcp.AddTool(server, &mcp.Tool{Name: "list_members", Title: "List members", Annotations: readOnly,
		Description: "List the people in this workspace and pending invites."}, t.listMembers)
	mcp.AddTool(server, &mcp.Tool{Name: "invite_member", Title: "Invite member",
		Description: "Invite someone to this workspace by email; they join when they next sign in. Admins only."}, t.inviteMember)
	mcp.AddTool(server, &mcp.Tool{Name: "rename_workspace", Title: "Rename workspace",
		Description: "Rename this workspace. Admins only."}, t.renameWorkspace)
}

type empty struct{}

type attributeView struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Multi  bool   `json:"multi,omitempty"`
	Unique bool   `json:"unique,omitempty"`
	Target string `json:"target,omitempty"`
}

type objectView struct {
	Slug       string          `json:"slug"`
	Name       string          `json:"name"`
	Attributes []attributeView `json:"attributes"`
}

type objectsOutput struct {
	Objects []objectView `json:"objects"`
}

func (t tools) listObjects(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, objectsOutput, error) {
	objects, err := t.crm.Objects(ctx, actor(req))
	out := objectsOutput{Objects: []objectView{}}
	for _, o := range objects {
		view := objectView{Slug: o.Slug, Name: o.Name, Attributes: []attributeView{}}
		for _, a := range o.Attributes {
			view.Attributes = append(view.Attributes, attributeView(a))
		}
		out.Objects = append(out.Objects, view)
	}
	return nil, out, err
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
}

// view shows a single-valued attribute as its value and a multi-valued one
// as a list; references appear as the record's id and name.
func view(r records.Record) recordView {
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

func (t tools) searchRecords(ctx context.Context, req *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, recordsOutput, error) {
	found, err := t.crm.Search(ctx, actor(req), records.Search{Object: in.Object, Query: in.Query, Where: in.Where, Limit: in.Limit})
	out := recordsOutput{Records: []recordView{}}
	for _, r := range found {
		out.Records = append(out.Records, view(r))
	}
	return nil, out, err
}

type getInput struct {
	RecordID string `json:"record_id"`
}

func (t tools) getRecord(ctx context.Context, req *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, recordView, error) {
	record, err := t.crm.Get(ctx, actor(req), in.RecordID)
	if err != nil {
		return nil, recordView{}, err
	}
	return nil, view(record), nil
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
	Skipped []skipView `json:"skipped,omitempty"`
}

var errValues = errors.New("values must be strings or lists of strings")

func (t tools) upsertRecord(ctx context.Context, req *mcp.CallToolRequest, in upsertInput) (*mcp.CallToolResult, upsertOutput, error) {
	set := map[string][]string{}
	for slug, raw := range in.Values {
		switch v := raw.(type) {
		case string:
			set[slug] = []string{v}
		case []any:
			set[slug] = []string{}
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return nil, upsertOutput{}, errValues
				}
				set[slug] = append(set[slug], s)
			}
		default:
			return nil, upsertOutput{}, errValues
		}
	}
	record, skips, err := t.crm.Upsert(ctx, actor(req), records.SourceAgent, records.Write{Object: in.Object, RecordID: in.RecordID, Set: set, Remove: in.Remove})
	if err != nil {
		return nil, upsertOutput{}, err
	}
	out := upsertOutput{Record: view(record)}
	for _, s := range skips {
		out.Skipped = append(out.Skipped, skipView{Attribute: s.Attribute, Value: s.Value, SetBy: string(s.Source)})
	}
	return nil, out, nil
}

type memberView struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Admin bool   `json:"admin,omitempty"`
	IsMe  bool   `json:"is_me,omitempty"`
}

type membersOutput struct {
	Members []memberView `json:"members"`
	Invited []string     `json:"invited"`
}

func (t tools) listMembers(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, membersOutput, error) {
	me := actor(req)
	users, invites, err := t.members.Members(ctx, me)
	out := membersOutput{Members: []memberView{}, Invited: []string{}}
	for _, u := range users {
		out.Members = append(out.Members, memberView{Name: u.Name, Email: u.Email, Admin: u.Admin, IsMe: u.ID == me.UserID})
	}
	for _, i := range invites {
		out.Invited = append(out.Invited, i.Email)
	}
	return nil, out, err
}

type inviteInput struct {
	Email string `json:"email"`
}

type inviteOutput struct {
	Invited string `json:"invited"`
}

func (t tools) inviteMember(ctx context.Context, req *mcp.CallToolRequest, in inviteInput) (*mcp.CallToolResult, inviteOutput, error) {
	invite, err := t.members.Invite(ctx, actor(req), in.Email)
	return nil, inviteOutput{Invited: invite.Email}, err
}

type renameInput struct {
	Name string `json:"name"`
}

type renameOutput struct {
	Workspace string `json:"workspace"`
}

func (t tools) renameWorkspace(ctx context.Context, req *mcp.CallToolRequest, in renameInput) (*mcp.CallToolResult, renameOutput, error) {
	name, err := t.members.Rename(ctx, actor(req), in.Name)
	return nil, renameOutput{Workspace: name}, err
}
