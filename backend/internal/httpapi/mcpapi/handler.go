// Package mcpapi publishes the CRM's operations: to agents as a Streamable
// HTTP MCP server, and to the web app as JSON over HTTP.
package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/fx"
)

const instructions = `Jaz CRM holds records of people, companies and other objects, and the emails, meetings and
calls with them. People have a built-in context attribute for their background and relationship
summary. Read it with get_record and write values.context with upsert_record; log dated notes with
log_interaction. Companies have founded_year (a whole year) and size (an employee range). Call
list_objects to learn each object's attributes and options. Emails, domains and phone
numbers identify records: upsert_record with an email or domain updates the record that holds it
instead of creating a duplicate. Values you write are marked as written by an agent; a value a
person set is never overwritten, and the write reports it as skipped. Tools act in your default
workspace; to work in another, pass its name as the workspace argument (list_workspaces).`

// Handler serves /mcp to bearer tokens and /api/tools/{tool} to sessions.
type Handler struct {
	MCP http.Handler
	API http.Handler
}

// Services are what the tools operate on.
type Services struct {
	fx.In
	Records      *records.Service
	Workspaces   *workspaces.Service
	Interactions *interactions.Service
	Connections  *connections.Service
	Logos        *logos.Service
}

func NewHandler(svc Services, keys *auth.Service, logger *log.Logger) *Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "jaz-crm",
		Title:   "Jaz CRM",
		Version: "0.1.0",
		Icons:   icons,
	}, &mcp.ServerOptions{Instructions: instructions})
	r := &registry{server: server, logger: logger.WithPrefix("tools"), members: svc.Workspaces, ops: map[string]func(context.Context, auth.Actor, json.RawMessage) (any, error){}}
	registerRecords(r, svc.Records, svc.Interactions, pictures{svc.Interactions, svc.Logos, keys.Issuer()})
	registerWorkspace(r, svc.Workspaces, keys)
	registerInteractions(r, svc.Interactions)
	registerConnections(r, svc.Connections, keys.Issuer())
	registerApp(r, keys.Issuer())
	registerProfile(r, keys)
	verify := func(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		actor, err := keys.Authenticate(ctx, token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			return nil, mcpauth.ErrInvalidToken
		}
		if err != nil {
			return nil, err
		}
		// The MCP session belongs to the credential, which keeps it across a
		// switch of workspace.
		return &mcpauth.TokenInfo{
			UserID:     actor.Principal(),
			Expiration: time.Now().Add(time.Hour),
			Extra:      map[string]any{actorKey: actor},
		}, nil
	}
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	requireToken := mcpauth.RequireBearerToken(verify, &mcpauth.RequireBearerTokenOptions{ResourceMetadataURL: keys.ResourceMetadataURL("/mcp")})
	return &Handler{MCP: requireToken(streamable), API: http.HandlerFunc(r.api)}
}

const actorKey = "actor"

var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true}

type empty struct{}
