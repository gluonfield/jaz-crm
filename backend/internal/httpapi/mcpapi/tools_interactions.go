package mcpapi

import (
	"context"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerInteractions(r *registry, svc *interactions.Service) {
	add(r, &mcp.Tool{Name: "list_triage_rules", Title: "List triage domain rules", Annotations: readOnly,
		Description: "Explicit keep/skip rules for entire email domains, including exclusions from deleted companies."},
		func(ctx context.Context, actor auth.Actor, _ empty) (domainRulesOutput, error) {
			rules, err := svc.DomainRules(ctx, actor)
			return domainRulesOutput{Rules: rules}, err
		})
	add(r, &mcp.Tool{Name: "forget_triage_rule", Title: "Forget triage domain rule",
		Description: "Stop applying an email domain's rule to new addresses. Existing keep/skip decisions are preserved."},
		func(ctx context.Context, actor auth.Actor, in domainRuleInput) (empty, error) {
			return empty{}, svc.ForgetDomainRule(ctx, actor, in.Domain)
		})
	add(r, &mcp.Tool{Name: "list_interactions", Title: "List interactions", Annotations: readOnly,
		Description: "A record's emails, meetings, calls and notes up to now, newest first; with upcoming, its future ones, soonest first."},
		func(ctx context.Context, actor auth.Actor, in timelineInput) (interactionsOutput, error) {
			list, err := svc.Timeline(ctx, actor, in.RecordID, in.Kinds, in.Before, in.Upcoming, in.Limit)
			return interactionsOutput{Interactions: list}, err
		})
	add(r, &mcp.Tool{Name: "get_interaction", Title: "Get interaction", Annotations: readOnly,
		Description: "One interaction in full: every message, transcript line and note."},
		func(ctx context.Context, actor auth.Actor, in interactionInput) (interactions.Interaction, error) {
			i, err := svc.Get(ctx, actor, in.InteractionID)
			return i, err
		})
	add(r, &mcp.Tool{Name: "search_interactions", Title: "Search interactions", Annotations: readOnly,
		Description: "Find interactions by title or content, newest first; without a query, list the latest."},
		func(ctx context.Context, actor auth.Actor, in searchInteractionsInput) (interactionsOutput, error) {
			list, err := svc.Search(ctx, actor, in.Query, in.Limit)
			return interactionsOutput{Interactions: list}, err
		})
	add(r, &mcp.Tool{Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)}, Name: "log_interaction", Title: "Log interaction",
		Description: "Log a call, a meeting outside the calendar, or a note, with the people in it and the records it concerns. The people are kept in the CRM."},
		func(ctx context.Context, actor auth.Actor, in logInput) (interactions.Interaction, error) {
			i, err := svc.Log(ctx, actor, "manual", in.entry())
			return i, err
		})
	add(r, &mcp.Tool{Name: "link_interaction", Title: "Link interaction",
		Description: "Attach an interaction to a record, such as a deal."},
		func(ctx context.Context, actor auth.Actor, in linkInput) (interactions.Interaction, error) {
			i, err := svc.Link(ctx, actor, in.InteractionID, in.RecordID)
			return i, err
		})
	add(r, &mcp.Tool{Name: "unlink_interaction", Title: "Unlink interaction",
		Description: "Detach an interaction from a record."},
		func(ctx context.Context, actor auth.Actor, in linkInput) (empty, error) {
			return empty{}, svc.Unlink(ctx, actor, in.InteractionID, in.RecordID)
		})
	add(r, &mcp.Tool{Name: "skip_interaction", Title: "Skip interaction",
		Description: "Remove an interaction from the CRM for good, such as a personal thread; its content is deleted."},
		func(ctx context.Context, actor auth.Actor, in interactionInput) (empty, error) {
			return empty{}, svc.Skip(ctx, actor, in.InteractionID)
		})
	add(r, &mcp.Tool{Name: "list_triage", Title: "List triage", Annotations: readOnly,
		Description: "Addresses by triage status: pending ones await a decision; kept ones are people in the CRM; skipped ones are not."},
		func(ctx context.Context, actor auth.Actor, in triageInput) (contactsOutput, error) {
			contacts, err := svc.Contacts(ctx, actor, in.Status, in.Query, in.Limit)
			return contactsOutput{Contacts: contacts}, err
		})
	add(r, &mcp.Tool{Name: "decide_triage", Title: "Decide triage",
		Description: "Keep or skip addresses, or whole domains including addresses not seen yet. Kept addresses become people with their conversations; skipped ones leave the CRM."},
		func(ctx context.Context, actor auth.Actor, in decideInput) (decideOutput, error) {
			if in.Decision != "keep" && in.Decision != "skip" {
				return decideOutput{}, errs.Invalidf("decision is keep or skip")
			}
			n, err := svc.Decide(ctx, actor, interactions.Decision{Addresses: in.Addresses, Domains: in.Domains, Keep: in.Decision == "keep", Reason: in.Reason})
			return decideOutput{Changed: n}, err
		})
}

type domainRulesOutput struct {
	Rules []interactions.DomainRule `json:"rules"`
}

type domainRuleInput struct {
	Domain string `json:"domain"`
}

type interactionsOutput struct {
	Interactions []interactions.Interaction `json:"interactions"`
}

type timelineInput struct {
	RecordID string     `json:"record_id"`
	Kinds    []string   `json:"kinds,omitempty" jsonschema:"email, meeting, call or note; all when empty"`
	Before   *time.Time `json:"before,omitempty" jsonschema:"only interactions that started earlier, to page back"`
	Upcoming bool       `json:"upcoming,omitempty" jsonschema:"list the interactions that start after now instead, such as scheduled meetings"`
	Limit    int        `json:"limit,omitempty" jsonschema:"at most 100, default 20"`
}

type interactionInput struct {
	InteractionID string `json:"interaction_id"`
}

type searchInteractionsInput struct {
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type logInput struct {
	Kind       string     `json:"kind" jsonschema:"call, meeting or note"`
	Title      string     `json:"title,omitempty"`
	At         *time.Time `json:"at,omitempty" jsonschema:"when it started; now when omitted"`
	End        *time.Time `json:"end,omitempty"`
	People     []string   `json:"people,omitempty" jsonschema:"email addresses or phone numbers of the people in it"`
	Records    []string   `json:"records,omitempty" jsonschema:"ids of records it concerns, such as a deal"`
	Notes      string     `json:"notes,omitempty"`
	Transcript string     `json:"transcript,omitempty"`
}

func (in logInput) entry() interactions.Entry {
	e := interactions.Entry{Kind: in.Kind, Title: in.Title, End: in.End, People: in.People, Records: in.Records, Notes: in.Notes, Transcript: in.Transcript}
	if in.At != nil {
		e.At = *in.At
	}
	return e
}

type linkInput struct {
	InteractionID string `json:"interaction_id"`
	RecordID      string `json:"record_id"`
}

type triageInput struct {
	Status string `json:"status,omitempty" jsonschema:"pending, kept or skipped; pending when omitted"`
	Query  string `json:"query,omitempty" jsonschema:"part of an address or name"`
	Limit  int    `json:"limit,omitempty" jsonschema:"at most 200, default 50"`
}

type contactsOutput struct {
	Contacts []interactions.Contact `json:"contacts"`
}

type decideInput struct {
	Addresses []string `json:"addresses,omitempty"`
	Domains   []string `json:"domains,omitempty" jsonschema:"such as acme.com; applies to addresses not seen yet"`
	Decision  string   `json:"decision" jsonschema:"keep or skip"`
	Reason    string   `json:"reason,omitempty"`
}

type decideOutput struct {
	Changed int `json:"changed"`
}
