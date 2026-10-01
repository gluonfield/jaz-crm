package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerFollowUps(r *registry, svc *followups.Service) {
	add(r, &mcp.Tool{Name: "send_draft", Title: "Send draft", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "A person releases a follow-up's draft: an email is sent now as a reply in the conversation the follow-up is linked to; a LinkedIn draft is approved for whoever sends it."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (recordView, error) {
			record, err := svc.Release(ctx, actor, in.RecordID)
			return recordOf(record, nil), err
		})
}
