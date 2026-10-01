package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerFollowUps(r *registry, svc *followups.Service) {
	add(r, &mcp.Tool{Name: "send_draft", Title: "Send draft", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "A person in the CRM releases a follow-up's draft as they saw it: an email is sent now as a reply in its newest linked conversation; a LinkedIn draft is approved for whoever sends it. Agents are refused."},
		func(ctx context.Context, actor auth.Actor, in sendInput) (recordView, error) {
			record, err := svc.Release(ctx, actor, in.RecordID, followups.Seen{Draft: in.Draft, To: in.To, Cc: in.Cc})
			return recordOf(record, nil), err
		})
}

type sendInput struct {
	RecordID string   `json:"record_id"`
	Draft    string   `json:"draft" jsonschema:"the draft text as the person saw it"`
	To       []string `json:"to,omitempty"`
	Cc       []string `json:"cc,omitempty"`
}
