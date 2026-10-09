package followups

import (
	"context"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

type RewriteInput struct {
	Draft       string       `json:"draft"`
	Subject     string       `json:"subject"`
	From        string       `json:"from,omitempty"`
	To          []string     `json:"to,omitempty"`
	Cc          []string     `json:"cc,omitempty"`
	Action      string       `json:"action"`
	Instruction string       `json:"instruction,omitempty"`
	Context     Conversation `json:"context"`
}

type RewriteResult struct {
	Draft   string `json:"draft"`
	Subject string `json:"subject"`
}

type Rewriter interface {
	Rewrite(context.Context, RewriteInput) (RewriteResult, error)
}

func (a *Agent) Rewrite(ctx context.Context, actor auth.Actor, id string, input RewriteInput) (RewriteResult, error) {
	if a.rewriter == nil {
		return RewriteResult{}, errs.Invalidf("AI drafting is not configured")
	}
	input.Draft = strings.TrimSpace(input.Draft)
	input.Subject = strings.TrimSpace(input.Subject)
	input.From = strings.TrimSpace(input.From)
	input.Instruction = strings.TrimSpace(input.Instruction)
	if input.Draft == "" && input.Action != "write" {
		return RewriteResult{}, errs.Invalidf("write a draft before editing it with AI")
	}
	if !slices.Contains([]string{"write", "shorten", "less_salesy", "one_clear_ask", "warmer", "polish", "custom"}, input.Action) {
		return RewriteResult{}, errs.Invalidf("choose a drafting action")
	}
	if input.Action == "custom" && input.Instruction == "" {
		return RewriteResult{}, errs.Invalidf("describe how to edit the draft")
	}
	if input.Action != "custom" {
		input.Instruction = ""
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	record, err := a.crm.Get(ctx, actor, id)
	if err != nil {
		return RewriteResult{}, err
	}
	if record.Object != records.FollowUps {
		return RewriteResult{}, errs.Invalidf("%s is not a follow-up", id)
	}
	if value(record, "draft_status") == records.DraftSending {
		return RewriteResult{}, errs.Invalidf("the draft is being sent")
	}
	conversation := interactions.Interaction{ID: record.ID, Kind: "draft", Channel: value(record, "channel"), Title: value(record, "name")}
	linked, err := a.convs.Timeline(ctx, actor, id, nil, "", false, 1)
	if err != nil {
		return RewriteResult{}, err
	}
	if len(linked) > 0 {
		conversation, err = a.convs.Source(ctx, actor, linked[0].ID)
		if err != nil {
			return RewriteResult{}, err
		}
	}
	conversation.Records = append(conversation.Records, interactions.Ref{ID: record.ID})
	for _, attribute := range []string{"person", "company", "deal"} {
		for _, ref := range refs(record, attribute) {
			conversation.Records = append(conversation.Records, interactions.Ref{ID: ref})
		}
	}
	for _, recipient := range slices.Concat(input.To, input.Cc) {
		address, err := mail.ParseAddress(recipient)
		if err != nil {
			continue
		}
		people, _, err := a.crm.Search(ctx, actor, records.Search{Object: "people", Where: map[string]string{"email_addresses": address.Address}, Limit: 1})
		if err != nil {
			return RewriteResult{}, err
		}
		for _, person := range people {
			conversation.Records = append(conversation.Records, interactions.Ref{ID: person.ID})
		}
	}
	input.Context, err = a.conversation(ctx, actor, conversation)
	if err != nil {
		return RewriteResult{}, err
	}
	input.Context.Channel = value(record, "channel")
	input.Context.WebAccess = false
	if input.From != "" {
		input.Context.Sender = &Person{Address: input.From}
	} else if value(record, "owner") != "" {
		input.Context.Sender = &Person{Address: value(record, "owner")}
	}
	for _, person := range input.Context.Us {
		if input.Context.Sender != nil && strings.EqualFold(person.Address, input.Context.Sender.Address) {
			input.Context.Sender.Name = person.Name
		}
	}
	if input.Context.Sender != nil && input.Context.Sender.Name == "" {
		connections, err := a.addresses.Connections(ctx, actor.WorkspaceID)
		if err != nil {
			return RewriteResult{}, err
		}
		for _, connection := range connections {
			if connection.Account != input.Context.Sender.Address && !slices.Contains(connection.Aliases, input.Context.Sender.Address) {
				continue
			}
			owner, err := a.workspaces.UserByID(ctx, connection.UserID)
			if err != nil {
				return RewriteResult{}, err
			}
			input.Context.Sender.Name = owner.Name
			break
		}
	}
	return a.rewriter.Rewrite(ctx, input)
}
