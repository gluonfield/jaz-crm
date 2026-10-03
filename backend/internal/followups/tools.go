package followups

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/google/jsonschema-go/jsonschema"
)

type ReadTools struct {
	WorkspaceID string
	Tools       []ReadTool
}

type ReadTool struct {
	Name        string
	Description string
	Parameters  *jsonschema.Schema
	Run         func(context.Context, json.RawMessage) (any, error)
}

type pageSize int
type recordOffset int32

type recordQuery struct {
	Object string        `json:"object" jsonschema:"Object slug from list_objects, such as people, companies, deals or pages"`
	Query  string        `json:"query" jsonschema:"Search text; empty lists all matches"`
	Where  []recordMatch `json:"where" jsonschema:"Equality filters; reference values are record IDs, e.g. parent for child pages or company for colleagues; empty means no filters"`
	Offset recordOffset  `json:"offset" jsonschema:"Number of records to skip, starting at zero"`
	Limit  pageSize      `json:"limit" jsonschema:"Page size"`
}

type recordMatch struct {
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
}

type recordID struct {
	ID string `json:"record_id"`
}

type interactionID struct {
	ID string `json:"interaction_id"`
}

type interactionQuery struct {
	Query string   `json:"query" jsonschema:"Search conversation titles and content; empty lists recent conversations"`
	Limit pageSize `json:"limit" jsonschema:"Maximum matches; narrow the query if the limit is reached"`
}

type timelineQuery struct {
	RecordID string   `json:"record_id"`
	Cursor   string   `json:"cursor" jsonschema:"Empty for the first page, then the last interaction ID from the preceding page"`
	Limit    pageSize `json:"limit" jsonschema:"Page size; continue until an empty page"`
}

// The same services back MCP reads. Bind the workspace here; model arguments
// cannot select another actor, add a tool or reach a write operation.
func NewReadTools(crm *records.Service, convs *interactions.Service, actor auth.Actor) ReadTools {
	return ReadTools{WorkspaceID: actor.WorkspaceID, Tools: []ReadTool{
		readTool("list_objects", "Discover this workspace's object and attribute schemas, including reference targets. Use these to choose record searches.", func(ctx context.Context, _ struct{}) (any, error) {
			return crm.Objects(ctx, actor)
		}),
		readTool("search_records", "Find records and pages anywhere in this workspace, regardless of the configured company knowledge root. Results omit document bodies: use get_record to read them. Use total and offset to page through all matches.", func(ctx context.Context, in recordQuery) (any, error) {
			filters := make([]records.Filter, 0, len(in.Where))
			for _, match := range in.Where {
				filters = append(filters, records.Filter{Attribute: match.Attribute, Operator: "is", Value: match.Value})
			}
			found, total, err := crm.Search(ctx, actor, records.Search{Object: in.Object, Query: in.Query, Filters: filters, Offset: int(in.Offset), Limit: int(in.Limit)})
			out := struct {
				Records []Record `json:"records"`
				Total   int      `json:"total"`
			}{Records: make([]Record, 0, len(found)), Total: total}
			for _, r := range found {
				out.Records = append(out.Records, recordInput(r))
			}
			return out, err
		}),
		readTool("get_record", "Read all fields of a record or the full content of a page. Reference IDs can be opened or used to find related records and child pages.", func(ctx context.Context, in recordID) (any, error) {
			r, err := crm.Get(ctx, actor, in.ID)
			return recordInput(r), err
		}),
		readTool("list_interactions", "List a person's, company's or deal's past conversations and notes, newest first, with previews. Page with cursor; use get_interaction for complete stored content.", func(ctx context.Context, in timelineQuery) (any, error) {
			return convs.Timeline(ctx, actor, in.RecordID, nil, in.Cursor, false, int(in.Limit))
		}),
		readTool("search_interactions", "Find relevant conversations and notes across this workspace by title or content. Results are previews, not full history; use get_interaction to read each relevant match.", func(ctx context.Context, in interactionQuery) (any, error) {
			return convs.Search(ctx, actor, in.Query, int(in.Limit))
		}),
		readTool("get_interaction", "Read an entire stored conversation, transcript or note, including original message text and quoted history.", func(ctx context.Context, in interactionID) (any, error) {
			return convs.Source(ctx, actor, in.ID)
		}),
	}}
}

func readTool[In any](name, description string, run func(context.Context, In) (any, error)) ReadTool {
	schema, err := jsonschema.For[In](&jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[pageSize]():     {Type: "integer", Minimum: new(1.0), Maximum: new(100.0)},
		reflect.TypeFor[recordOffset](): {Type: "integer", Minimum: new(0.0), Maximum: new(float64(math.MaxInt32))},
	}})
	if err != nil {
		panic(err)
	}
	if schema.Properties == nil {
		schema.Properties = map[string]*jsonschema.Schema{}
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		panic(err)
	}
	return ReadTool{Name: name, Description: description, Parameters: schema, Run: func(ctx context.Context, raw json.RawMessage) (any, error) {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, errs.Invalidf("invalid JSON arguments: %v", err)
		}
		if err := resolved.Validate(value); err != nil {
			return nil, errs.Invalidf("invalid arguments: %v", err)
		}
		var in In
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, errs.Invalidf("invalid arguments: %v", err)
		}
		return run(ctx, in)
	}}
}

func (r ReadTools) Call(ctx context.Context, name, arguments string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	index := slices.IndexFunc(r.Tools, func(tool ReadTool) bool { return tool.Name == name })
	var result any
	var err error
	if index < 0 {
		err = errs.Invalidf("tool %q is unavailable; use only the advertised read-only tools", name)
	} else {
		result, err = r.Tools[index].Run(ctx, json.RawMessage(arguments))
	}
	if err != nil {
		var invalid errs.Invalid
		if !errors.As(err, &invalid) {
			return "", err
		}
		result = map[string]string{"error": invalid.Error()}
	}
	data, err := json.Marshal(result)
	return string(data), err
}
