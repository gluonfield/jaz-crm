// Package followups acts on follow-ups: it sends their drafts and keeps them
// current as conversations move.
package followups

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

type Service struct {
	crm   *records.Service
	store storage.InteractionStore
	conns *connections.Service
}

func NewService(crm *records.Service, store storage.InteractionStore, conns *connections.Service) *Service {
	return &Service{crm: crm, store: store, conns: conns}
}

// Release is a person letting a follow-up's draft go. An email draft is sent
// now as a reply in the conversation the follow-up is linked to; any other
// draft is approved for whoever sends it. Writes are a person's, since only
// people release drafts.
func (s *Service) Release(ctx context.Context, actor auth.Actor, id string) (records.Record, error) {
	f, err := s.crm.Get(ctx, actor, id)
	if err != nil {
		return records.Record{}, err
	}
	if f.Object != records.FollowUps {
		return records.Record{}, errs.Invalidf("%s is not a follow-up", id)
	}
	if value(f, "channel") != "Email" {
		return s.set(ctx, actor, id, "draft_status", records.DraftApproved)
	}
	reply, err := s.reply(ctx, actor, f)
	if err != nil {
		return records.Record{}, err
	}
	if value(f, "draft_status") == records.DraftWritten {
		if _, err := s.set(ctx, actor, id, "draft_status", records.DraftApproved); err != nil {
			return records.Record{}, err
		}
	}
	if _, err := s.set(ctx, actor, id, "draft_status", records.DraftSending); err != nil {
		return records.Record{}, err
	}
	if err := reply.send(ctx); err != nil {
		if _, undo := s.set(ctx, actor, id, "draft_status", records.DraftApproved); undo != nil {
			err = errors.Join(err, undo)
		}
		return records.Record{}, err
	}
	return s.set(ctx, actor, id, "draft_status", records.DraftSent, "status", "Done")
}

func (s *Service) set(ctx context.Context, actor auth.Actor, id string, pairs ...string) (records.Record, error) {
	set := map[string][]string{}
	for i := 0; i < len(pairs); i += 2 {
		set[pairs[i]] = []string{pairs[i+1]}
	}
	record, _, err := s.crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: id, Set: set})
	return record, err
}

// outgoing is a reply ready to send from one mailbox.
type outgoing struct {
	mailbox *google.Client
	account string
	message google.Outgoing
}

func (o outgoing) send(ctx context.Context) error {
	_, err := o.mailbox.Send(ctx, o.message)
	var api *google.APIError
	if errors.As(err, &api) && api.Status == http.StatusForbidden {
		return errs.Invalidf("%s must reconnect Google to send mail from the CRM", o.account)
	}
	return err
}

// reply addresses a follow-up's draft as a reply to the latest message of
// the email conversation it is linked to, from the mailbox that holds that
// message when the actor may send from it.
func (s *Service) reply(ctx context.Context, actor auth.Actor, f records.Record) (outgoing, error) {
	body := value(f, "draft")
	to := values(f, "to")
	if strings.TrimSpace(body) == "" || len(to) == 0 {
		return outgoing{}, errs.Invalidf("an email draft needs text and a recipient")
	}
	threads, err := s.store.Timeline(ctx, storage.TimelineQuery{RecordID: f.ID, WorkspaceID: actor.WorkspaceID, Kinds: []string{interactions.Email}, Limit: 1})
	if err != nil {
		return outgoing{}, err
	}
	if len(threads) == 0 {
		return outgoing{}, errs.Invalidf("link the email conversation this follow-up replies in")
	}
	parts, err := s.store.Parts(ctx, []string{threads[0].ID})
	if err != nil {
		return outgoing{}, err
	}
	parts = slices.DeleteFunc(parts, func(p storage.Part) bool { return p.Kind != "message" || p.ConnectionID == nil || p.ProviderID == nil })
	if len(parts) == 0 {
		return outgoing{}, errs.Invalidf("the linked conversation has no message to reply to")
	}
	last := parts[len(parts)-1]
	written, err := s.draftedAt(ctx, actor, f.ID)
	if err != nil {
		return outgoing{}, err
	}
	if last.At.After(written) {
		return outgoing{}, errs.Invalidf("a message arrived after this draft was written; review the draft first")
	}
	holder, err := s.conns.Connection(ctx, *last.ConnectionID)
	if err != nil {
		return outgoing{}, err
	}
	source, err := s.conns.Google(ctx, holder)
	if err != nil {
		return outgoing{}, err
	}
	found, err := source.Messages(ctx, []string{*last.ProviderID}, false)
	if err != nil {
		return outgoing{}, err
	}
	original := found[0]
	if original.ID == "" {
		return outgoing{}, errs.Invalidf("the message to reply to is gone from %s", holder.Account)
	}
	sender, err := s.conns.Mailbox(ctx, actor, holder.ID)
	if err != nil {
		return outgoing{}, err
	}
	mailbox, err := s.conns.Google(ctx, sender)
	if err != nil {
		return outgoing{}, err
	}
	thread := original.ThreadID
	if sender.ID != holder.ID && original.MessageID != "" {
		thread, err = mailbox.ThreadOf(ctx, original.MessageID)
		if errors.Is(err, google.ErrNotFound) {
			thread, err = "", nil
		}
		if err != nil {
			return outgoing{}, err
		}
	}
	subject := original.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	references := original.References
	if original.MessageID != "" {
		references = append(slices.Clone(references), original.MessageID)
	}
	return outgoing{mailbox: mailbox, account: sender.Account, message: google.Outgoing{
		From: sender.Account, To: to, Cc: values(f, "cc"), Subject: subject, Body: body,
		ThreadID: thread, InReplyTo: original.MessageID, References: references,
	}}, nil
}

// draftedAt is when the follow-up's draft text was last written.
func (s *Service) draftedAt(ctx context.Context, actor auth.Actor, id string) (time.Time, error) {
	changes, err := s.crm.History(ctx, actor, id)
	if err != nil {
		return time.Time{}, err
	}
	i := slices.IndexFunc(changes, func(c records.Change) bool { return c.Attribute == "draft" && !c.Removed })
	if i < 0 {
		return time.Time{}, errs.Invalidf("the follow-up has no draft")
	}
	return changes[i].At, nil
}

func value(r records.Record, attribute string) string {
	return slices.Concat(values(r, attribute), []string{""})[0]
}

func values(r records.Record, attribute string) []string {
	var out []string
	for _, f := range r.Fields {
		if f.Attribute == attribute {
			for _, v := range f.Values {
				out = append(out, v.Text)
			}
		}
	}
	return out
}
