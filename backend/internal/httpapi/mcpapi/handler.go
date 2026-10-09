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
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/fx"
)

const instructions = `Jaz CRM holds records of people, companies and other objects, and the emails, meetings and calls
with them. A person's context is the TLDR of the relationship as bullet points of one short line each: who they are
and how we know them, then dated events, newest first. Read it with get_record. To add what you
learned, write values.context with upsert_record as the whole new version, merged from the current
context and the new information, keeping what is still true; every write replaces it, whoever wrote
it before. Log dated notes with log_interaction. Companies have founded_year (a whole year) and size
(an employee range). Follow-ups track what we or they owe next on a person, company or deal, with a
action_date (YYYY-MM-DD or RFC3339 with offset) and an optional draft: get_record lists a record's follow-ups, and search_records on
follow_ups with Needs attention shows open actions waiting on Us or with Waiting on unset, regardless of date; Chase shows open, overdue actions waiting on Them.
An agent setting action_date must also set action_date_basis (Stated or Suggested) and
action_date_reason with evidence or the scheduling convention. Manual dates and explicit clears
are preserved by automatic drafting. Waiting on Them never gets an automatic draft. To send an approved draft on a
chat channel such as LinkedIn or X, set its draft_status to Sending, send it, log the sent message with log_interaction
(kind message, its channel, direction sent, the thread's url), then set Sent. A person's linkedin_url and x_url
link their LinkedIn and X profiles, which identify them in chats. Call list_objects to learn each object's
attributes and options. Emails, domains and phone numbers identify records: upsert_record with an
email or domain updates the record that holds it instead of creating a duplicate. Values you write
are marked as written by an agent; apart from context, a value a person set is never overwritten,
and the write reports it as skipped. Tools act in your default workspace; to work in another, pass
its name as the workspace argument (list_workspaces).`

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
	FollowUps    *followups.Service
	Drafting     *followups.Agent
}

func NewHandler(svc Services, keys *auth.Service, logger *log.Logger) *Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "jaz-crm",
		Title:   "Jaz CRM",
		Version: "0.1.0",
		Icons:   icons,
	}, &mcp.ServerOptions{Instructions: instructions})
	r := &registry{server: server, logger: logger.WithPrefix("tools"), publicURL: keys.Issuer(), members: svc.Workspaces, ops: map[string]func(context.Context, auth.Actor, json.RawMessage) (any, error){}}
	registerRecords(r, svc.Records, svc.Interactions, pictures{svc.Interactions, svc.Logos, keys.Issuer()})
	registerSearch(r, svc.Records)
	registerWorkspace(r, svc.Workspaces, keys)
	registerInteractions(r, svc.Interactions)
	registerConnections(r, svc.Connections, keys.Issuer())
	registerFollowUps(r, svc.FollowUps, svc.Drafting)
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
