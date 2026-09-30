package mcpapi

import (
	"context"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerConnections(r *registry, conns *connections.Service, publicURL string) {
	add(r, &mcp.Tool{Name: "list_connections", Title: "List connections", Annotations: readOnly,
		Description: "Google accounts syncing mail and calendar into this workspace: when each stream last moved, the step running now, how much mail is in and whether its history back to since is complete, and where to connect another."},
		func(ctx context.Context, actor auth.Actor, _ empty) (connectionsOutput, error) {
			list, err := conns.List(ctx, actor)
			out := connectionsOutput{Connections: list, Since: conns.Since()}
			if conns.Enabled() {
				out.ConnectURL = publicURL + "/connections/google/start"
			}
			return out, err
		})
	add(r, &mcp.Tool{Name: "disconnect", Title: "Disconnect",
		Description: "Stop syncing a Google account; what it already synced stays."},
		func(ctx context.Context, actor auth.Actor, in connectionInput) (empty, error) {
			return empty{}, conns.Disconnect(ctx, actor, in.ConnectionID)
		})
}

type connectionsOutput struct {
	Connections []connections.View `json:"connections"`
	ConnectURL  string             `json:"connect_url,omitempty"`
	// Since is where the synced window starts.
	Since time.Time `json:"since"`
}

type connectionInput struct {
	ConnectionID string `json:"connection_id"`
}
