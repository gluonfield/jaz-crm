// Package mcpapi exposes the CRM to agents as a Streamable HTTP MCP server.
package mcpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `Jaz CRM holds records of people, companies and other objects. Call list_objects to learn each
object's attributes. Emails, domains and phone numbers identify records: upsert_record with an email or
domain updates the record that holds it instead of creating a duplicate. Values you write are marked as
written by an agent; a value a person set is never overwritten, and the write reports it as skipped.`

// Handler serves /mcp. Requests carry an OAuth access token or API key as a
// Bearer token; a 401 points clients at the protected-resource metadata.
type Handler struct {
	http.Handler
}

func NewHandler(crm *records.Service, members *workspaces.Service, keys *auth.Service) *Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "jaz-crm",
		Title:   "Jaz CRM",
		Version: "0.1.0",
	}, &mcp.ServerOptions{Instructions: instructions})
	register(server, tools{crm: crm, members: members})
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
	return &Handler{Handler: requireToken(streamable)}
}

const actorKey = "actor"

type tools struct {
	crm     *records.Service
	members *workspaces.Service
}

// actor is who the bearer token resolved to for this request.
func actor(req *mcp.CallToolRequest) auth.Actor {
	return req.Extra.TokenInfo.Extra[actorKey].(auth.Actor)
}
