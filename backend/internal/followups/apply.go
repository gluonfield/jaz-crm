package followups

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func (a *Agent) savePlan(ctx context.Context, actor auth.Actor, conv interactions.Interaction, in Conversation, plan Plan) (bool, error) {
	drafts := make([]map[string][]string, len(plan.FollowUps))
	replies := 0
	for i, c := range plan.FollowUps {
		drafts[i] = map[string][]string{}
		if c.ID != "" && !slices.ContainsFunc(in.FollowUps, func(o Open) bool { return o.ID == c.ID }) {
			continue
		}
		if c.WaitingOn != "Them" && c.Status != "Done" && c.Status != "Dismissed" && strings.TrimSpace(c.Reply) != "" {
			if err := a.draft(ctx, actor, conv, c, drafts[i]); err != nil {
				return false, err
			}
			replies++
		}
	}
	if conv.Channel == "email" && replies > 1 {
		return false, errs.Invalidf("combine this conversation's response into one reply follow-up")
	}
	drafted := false
	err := a.store.Atomically(ctx, func(store storage.InteractionStore) error {
		if err := store.LockWorkspace(ctx, actor.WorkspaceID); err != nil {
			return err
		}
		writer := Agent{Service: NewService(records.NewService(store), store, a.conns, a.addresses)}
		for i, change := range plan.FollowUps {
			written, err := writer.apply(ctx, actor, conv, in, change, drafts[i])
			if err != nil {
				return err
			}
			drafted = drafted || written
		}
		for _, c := range plan.Contexts {
			i := slices.IndexFunc(in.Records, func(current Record) bool { return current.Object == "people" && current.ID == c.Person })
			if i < 0 || strings.TrimSpace(c.Context) == "" || slices.Equal([]string{strings.TrimSpace(c.Context)}, in.Records[i].Values[records.ContextAttribute]) {
				continue
			}
			if _, _, err := writer.crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: "people", RecordID: c.Person, Set: map[string][]string{records.ContextAttribute: {plain(c.Context)}}}); err != nil {
				return err
			}
		}
		return nil
	})
	return drafted, err
}

// apply writes one change and its conversation link in the plan's transaction.
func (a *Agent) apply(ctx context.Context, actor auth.Actor, conv interactions.Interaction, in Conversation, c Change, set map[string][]string) (bool, error) {
	if c.ID != "" && !slices.ContainsFunc(in.FollowUps, func(o Open) bool { return o.ID == c.ID }) {
		return false, nil
	}
	if len(set["draft"]) > 0 && conv.Channel == "email" {
		if c.ID == "" {
			for _, open := range in.FollowUps {
				if open.Conversation && open.Channel == "Email" {
					c.ID = open.ID
					break
				}
			}
		}
		id, err := a.replyTarget(ctx, actor, conv.ID, c.ID)
		if err != nil {
			return false, err
		}
		c.ID = id
	}
	if c.ID != "" {
		current, err := a.crm.Get(ctx, actor, c.ID)
		if err != nil {
			return false, err
		}
		if value(current, "status") != "Open" {
			return false, nil
		}
		draft, err := a.store.GmailDraft(ctx, actor.WorkspaceID, c.ID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return false, err
		}
		if err == nil && draft.State == "draft" {
			return false, nil
		}
	}
	put := func(attribute, value string) {
		if value = strings.TrimSpace(value); value != "" {
			set[attribute] = []string{value}
		}
	}
	if c.ID == "" && in.Sender != nil {
		put("owner", in.Sender.Address)
	}
	put("name", plain(c.Action))
	put("waiting_on", c.WaitingOn)
	put("status", c.Status)
	remove := map[string][]string{}
	if c.ActionDate != nil {
		if strings.TrimSpace(c.ActionDate.Value) == "" {
			remove["action_date"] = nil
		} else {
			date, err := actionDate(c.ActionDate.Value, in.Timezone)
			if err != nil {
				return false, err
			}
			put("action_date", date)
			put("action_date_basis", c.ActionDate.Basis)
			put("action_date_reason", c.ActionDate.Reason)
			put("action_date_source", conv.ID)
		}
	}
	for object, attribute := range subjects {
		id := map[string]string{"person": c.Person, "company": c.Company, "deal": c.Deal}[attribute]
		if slices.ContainsFunc(in.Records, func(r Record) bool { return r.ID == id && r.Object == object }) {
			put(attribute, id)
		}
	}
	if len(set["draft"]) > 0 && c.ID != "" && set["to"] != nil {
		current, err := a.crm.Get(ctx, actor, c.ID)
		if err != nil {
			return false, err
		}
		for _, attribute := range []string{"to", "cc"} {
			remove[attribute] = slices.DeleteFunc(values(current, attribute), func(address string) bool { return slices.Contains(set[attribute], address) })
			if len(remove[attribute]) == 0 {
				delete(remove, attribute)
			}
		}
	}
	if c.ID == "" && set["name"] == nil || len(set) == 0 && len(remove) == 0 {
		return false, nil
	}
	f, _, err := a.crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: records.FollowUps, RecordID: c.ID, Set: set, Remove: remove})
	if err != nil {
		return false, err
	}
	drafted := len(set["draft"]) > 0 && value(f, "waiting_on") != "Them" && value(f, "status") == "Open" && value(f, "draft") == set["draft"][0]
	return drafted, a.store.AddLink(ctx, conv.ID, f.ID, interactions.ByAgent)
}

func (a *Agent) replyTarget(ctx context.Context, actor auth.Actor, conversationID, requested string) (string, error) {
	links, err := a.store.Links(ctx, []string{conversationID})
	if err != nil {
		return "", err
	}
	target := requested
	for _, link := range links {
		r, err := a.crm.Get(ctx, actor, link.RecordID)
		if err != nil {
			return "", err
		}
		if r.Object != records.FollowUps || value(r, "status") != "Open" || value(r, "channel") != "Email" {
			continue
		}
		if target != "" && target != r.ID {
			return "", errs.Invalidf("this conversation already has a reply follow-up; update %s instead of creating another reply", r.ID)
		}
		target = r.ID
	}
	return target, nil
}

// draft sets a reply's text and channel; an email reply goes to everyone on
// the latest message, except the sending mailbox and its aliases.
func (a *Agent) draft(ctx context.Context, actor auth.Actor, conv interactions.Interaction, c Change, set map[string][]string) error {
	channel := cmp.Or(c.Channel, records.ChannelName(conv.Channel))
	if !slices.Contains(records.Channels, channel) {
		return nil
	}
	set["draft"] = []string{plain(c.Reply)}
	set["channel"] = []string{channel}
	if channel != "Email" {
		return nil
	}
	if c.Subject != "" {
		set["subject"] = []string{plain(c.Subject)}
	}
	if conv.Channel != "email" {
		return nil
	}
	parts, err := a.store.Parts(ctx, []string{conv.ID})
	if err != nil {
		return err
	}
	parts = slices.DeleteFunc(parts, func(p storage.Part) bool { return p.Kind != "message" || p.ConnectionID == nil || p.ProviderID == nil })
	if len(parts) == 0 {
		return nil
	}
	last, holder, err := a.original(ctx, parts[len(parts)-1])
	if err != nil {
		return err
	}
	sender, err := a.conns.Mailbox(ctx, actor, holder.ID)
	if err != nil {
		return err
	}
	to, cc := replyAll(last, slices.Concat([]string{sender.Account}, sender.Aliases))
	if len(to) > 0 {
		set["to"] = to
	}
	if len(cc) > 0 {
		set["cc"] = cc
	}
	return nil
}
