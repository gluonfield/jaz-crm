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
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/fx"
)

const instructions = `Jaz CRM holds records of people, companies and other objects, and the emails, meetings and
calls with them. Call list_objects to learn each object's attributes. Emails, domains and phone
numbers identify records: upsert_record with an email or domain updates the record that holds it
instead of creating a duplicate. Values you write are marked as written by an agent; a value a
person set is never overwritten, and the write reports it as skipped.`

// Handler serves /mcp to bearer tokens and /api/tools/{tool} to sessions.
type Handler struct {
	MCP http.Handler
	API http.Handler
}

// Services are what the tools operate on.
type Services struct {
	fx.In
	Records    *records.Service
	Workspaces *workspaces.Service
}

func NewHandler(svc Services, keys *auth.Service, logger *log.Logger) *Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "jaz-crm",
		Title:   "Jaz CRM",
		Version: "0.1.0",
	}, &mcp.ServerOptions{Instructions: instructions})
	r := &registry{server: server, logger: logger.WithPrefix("tools"), ops: map[string]func(context.Context, auth.Actor, json.RawMessage) (any, error){}}
	registerRecords(r, svc.Records)
	registerWorkspace(r, svc.Workspaces)
	verify := func(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		actor, err := keys.Authenticate(ctx, token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			return nil, mcpauth.ErrInvalidToken
		}
		if err != nil {
			return nil, err
		}
		return &mcpauth.TokenInfo{
			UserID:     actor.UserID,
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
