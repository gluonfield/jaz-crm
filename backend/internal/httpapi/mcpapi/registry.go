package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// op is a tool's transport-neutral operation.
type op[In, Out any] func(ctx context.Context, actor auth.Actor, in In) (Out, error)

// registry publishes each operation as an MCP tool for agents and the MCP
// App, and at POST /api/tools/{tool} for the web app.
type registry struct {
	server  *mcp.Server
	logger  *log.Logger
	members *workspaces.Service
	ops     map[string]func(ctx context.Context, actor auth.Actor, raw json.RawMessage) (any, error)
}

// add publishes an operation that acts in one workspace: the one its optional
// workspace argument names, else the caller's default.
func add[In, Out any](r *registry, tool *mcp.Tool, fn op[In, Out]) {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	if schema.Properties == nil {
		schema.Properties = map[string]*jsonschema.Schema{}
	}
	schema.Properties["workspace"] = &jsonschema.Schema{Type: "string", Description: "name of the workspace to act in; omit for the default one (list_workspaces)"}
	tool.InputSchema = schema
	publish(r, tool, func(ctx context.Context, actor auth.Actor, raw json.RawMessage, in In) (Out, error) {
		var args struct {
			Workspace string `json:"workspace"`
		}
		_ = json.Unmarshal(raw, &args)
		actor, err := r.members.In(ctx, actor, args.Workspace)
		if err != nil {
			var none Out
			return none, err
		}
		return fn(ctx, actor, in)
	})
}

// addUnscoped publishes an operation without the workspace argument: it acts
// across the person's workspaces, or in the default one as the app does.
func addUnscoped[In, Out any](r *registry, tool *mcp.Tool, fn op[In, Out]) {
	publish(r, tool, func(ctx context.Context, actor auth.Actor, _ json.RawMessage, in In) (Out, error) {
		return fn(ctx, actor, in)
	})
}

func publish[In, Out any](r *registry, tool *mcp.Tool, fn func(ctx context.Context, actor auth.Actor, raw json.RawMessage, in In) (Out, error)) {
	if tool.Meta == nil {
		tool.Meta = mcp.Meta{}
	}
	tool.Meta["securitySchemes"] = []map[string]any{{"type": "oauth2", "scopes": []string{}}}
	if tool.Annotations == nil {
		tool.Annotations = &mcp.ToolAnnotations{}
	}
	if tool.Annotations.DestructiveHint == nil {
		tool.Annotations.DestructiveHint = new(!tool.Annotations.ReadOnlyHint)
	}
	tool.Annotations.OpenWorldHint = new(false)
	mcp.AddTool(r.server, tool, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		out, err := fn(ctx, req.Extra.TokenInfo.Extra[actorKey].(auth.Actor), req.Params.Arguments, in)
		return nil, out, r.public(tool.Name, err)
	})
	r.ops[tool.Name] = func(ctx context.Context, actor auth.Actor, raw json.RawMessage) (any, error) {
		var in In
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, errs.Invalidf("invalid arguments: %v", err)
			}
		}
		out, err := fn(ctx, actor, raw, in)
		return out, r.public(tool.Name, err)
	}
}

// public passes a caller's mistake through and hides every other failure.
func (r *registry) public(tool string, err error) error {
	if err == nil || errors.As(err, new(errs.Invalid)) {
		return err
	}
	r.logger.Error("tool failed", "tool", tool, "error", err)
	return errors.New("internal error")
}

// api serves the operations to a signed-in browser; the session middleware
// puts its actor in the context.
func (r *registry) api(w http.ResponseWriter, req *http.Request) {
	fn, ok := r.ops[req.PathValue("tool")]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such tool"})
		return
	}
	actor, _ := auth.ActorFrom(req.Context())
	raw, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unreadable body"})
		return
	}
	out, err := fn(req.Context(), actor, raw)
	switch {
	case errors.As(err, new(errs.Invalid)):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, out)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
