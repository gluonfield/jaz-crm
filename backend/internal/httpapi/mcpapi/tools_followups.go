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
			record, err := svc.Release(ctx, actor, in.RecordID, followups.Seen{Draft: in.Draft, From: in.From, To: in.To, Cc: in.Cc})
			return recordOf(record, nil), err
		})
	add(r, &mcp.Tool{Name: "get_draft_sender", Title: "Get draft sender", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "The mailbox, reply-all recipients and Gmail signature for a follow-up's email reply, including before a draft has been written."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (followups.Sender, error) {
			return svc.Sender(ctx, actor, in.RecordID)
		})
}

type sendInput struct {
	RecordID string   `json:"record_id"`
	Draft    string   `json:"draft" jsonschema:"the draft text as the person saw it"`
	From     string   `json:"from,omitempty" jsonschema:"the mailbox an email draft was shown going from"`
	To       []string `json:"to,omitempty"`
	Cc       []string `json:"cc,omitempty"`
}
