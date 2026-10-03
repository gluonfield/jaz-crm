package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerFollowUps(r *registry, svc *followups.Service, drafting *followups.Agent) {
	add(r, &mcp.Tool{Name: "rewrite_draft", Title: "Edit draft with AI", Annotations: readOnly, Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "Propose an edit to the current draft in one model call using stored conversation and CRM context. Does not save or send."},
		func(ctx context.Context, actor auth.Actor, in rewriteInput) (followups.RewriteResult, error) {
			return drafting.Rewrite(ctx, actor, in.RecordID, followups.RewriteInput{Draft: in.Draft, Subject: in.Subject, From: in.From, To: in.To, Cc: in.Cc, Action: in.Action, Instruction: in.Instruction})
		})
	add(r, &mcp.Tool{Name: "save_draft", Title: "Save draft", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "Save the reply edited in the CRM composer. Replaces draft text, subject, channel and recipients; editing withdraws prior approval."},
		func(ctx context.Context, actor auth.Actor, in saveDraftInput) (saveDraftOutput, error) {
			saved, err := svc.SaveDraft(ctx, actor, in.RecordID, in.Draft, in.Subject, in.Channel, in.To, in.Cc, in.Revision)
			return saveDraftOutput{recordView: recordOf(saved.Record, nil), Revision: saved.Revision}, err
		})
	add(r, &mcp.Tool{Name: "send_draft", Title: "Send draft", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "Release a follow-up's reviewed draft after explicit confirmation: an email is sent now, replying in its newest linked email conversation when present or starting a new email otherwise; a LinkedIn draft is approved for whoever sends it. The CRM app calls this only after Send/Approve is confirmed."},
		func(ctx context.Context, actor auth.Actor, in sendInput) (recordView, error) {
			record, err := svc.Release(ctx, actor, in.RecordID, followups.Seen{Confirmed: in.Confirmed, Draft: in.Draft, Subject: in.Subject, From: in.From, To: in.To, Cc: in.Cc, Bcc: in.Bcc, Revision: in.Revision})
			return recordOf(record, nil), err
		})
	add(r, &mcp.Tool{Name: "get_draft_sender", Title: "Get draft sender", Meta: mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "The mailbox, subject, recipients and Gmail signature for a follow-up's email, including before a draft has been written."},
		func(ctx context.Context, actor auth.Actor, in recordInput) (followups.Sender, error) {
			return svc.Sender(ctx, actor, in.RecordID)
		})
}

type rewriteInput struct {
	RecordID    string   `json:"record_id"`
	Draft       string   `json:"draft"`
	Subject     string   `json:"subject"`
	From        string   `json:"from,omitempty"`
	To          []string `json:"to,omitempty"`
	Cc          []string `json:"cc,omitempty"`
	Action      string   `json:"action"`
	Instruction string   `json:"instruction,omitempty"`
}

type saveDraftOutput struct {
	recordView
	Revision string `json:"revision,omitempty"`
}

type saveDraftInput struct {
	Revision string   `json:"revision,omitempty"`
	RecordID string   `json:"record_id"`
	Draft    string   `json:"draft"`
	Subject  string   `json:"subject"`
	Channel  string   `json:"channel"`
	To       []string `json:"to"`
	Cc       []string `json:"cc"`
}

type sendInput struct {
	Revision  string   `json:"revision,omitempty"`
	Bcc       []string `json:"bcc,omitempty"`
	Confirmed bool     `json:"confirmed" jsonschema:"true only after explicit confirmation of the displayed reply and recipients"`
	RecordID  string   `json:"record_id"`
	Draft     string   `json:"draft" jsonschema:"the draft text as the person saw it"`
	Subject   string   `json:"subject" jsonschema:"the email subject as the person saw it"`
	From      string   `json:"from,omitempty" jsonschema:"the mailbox an email draft was shown going from"`
	To        []string `json:"to,omitempty"`
	Cc        []string `json:"cc,omitempty"`
}
