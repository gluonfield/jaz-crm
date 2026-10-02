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

// Seen is the draft a person released, as they saw it, with the mailbox an
// email draft goes from.
type Seen struct {
	Draft  string
	From   string
	To, Cc []string
}

// Release is a person letting a follow-up's draft go. An email draft is sent
// now as a reply in its newest linked conversation; any other draft is
// approved for whoever sends it. A bearer credential is refused, since it may
// be an agent; and a draft that changed since the person saw it is refused.
func (s *Service) Release(ctx context.Context, actor auth.Actor, id string, seen Seen) (records.Record, error) {
	if actor.Agent {
		return records.Record{}, errs.Invalidf("only a person sends or approves a draft, in the CRM in a browser")
	}
	f, err := s.crm.Get(ctx, actor, id)
	if err != nil {
		return records.Record{}, err
	}
	if f.Object != records.FollowUps {
		return records.Record{}, errs.Invalidf("%s is not a follow-up", id)
	}
	if strings.TrimSpace(value(f, "draft")) != strings.TrimSpace(seen.Draft) || !sameSet(values(f, "to"), seen.To) || !sameSet(values(f, "cc"), seen.Cc) {
		return records.Record{}, errs.Invalidf("the draft changed since you saw it; review it again")
	}
	if value(f, "channel") != "Email" {
		return s.set(ctx, actor, id, "draft_status", records.DraftApproved)
	}
	reply, err := s.reply(ctx, actor, f)
	if err != nil {
		return records.Record{}, err
	}
	if reply.account != seen.From {
		return records.Record{}, errs.Invalidf("this reply now goes from %s; review it again", reply.account)
	}
	if value(f, "draft_status") == records.DraftWritten {
		if _, err := s.set(ctx, actor, id, "draft_status", records.DraftApproved); err != nil {
			return records.Record{}, err
		}
	}
	if _, err := s.set(ctx, actor, id, "draft_status", records.DraftSending); err != nil {
		return records.Record{}, err
	}
	_, err = reply.mailbox.Send(ctx, reply.message)
	var api *google.APIError
	switch {
	case errors.As(err, &api):
		if _, undo := s.set(ctx, actor, id, "draft_status", records.DraftApproved); undo != nil {
			return records.Record{}, errors.Join(err, undo)
		}
		if api.Status == http.StatusForbidden {
			return records.Record{}, errs.Invalidf("%s must reconnect Google to send mail from the CRM", reply.account)
		}
		return records.Record{}, err
	case err != nil:
		return records.Record{}, errs.Invalidf("the reply may have gone out from %s: check its Sent mail, then set the draft back to Approved to send it again", reply.account)
	}
	return s.set(ctx, actor, id, "draft_status", records.DraftSent, "status", "Done")
}

func sameSet(a, b []string) bool {
	a = slices.Sorted(slices.Values(a))
	b = slices.Sorted(slices.Values(b))
	return slices.Equal(slices.Compact(a), slices.Compact(b))
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

// Sender is the mailbox a follow-up's email draft goes from for the actor,
// with the text of the signature Gmail adds below it.
func (s *Service) Sender(ctx context.Context, actor auth.Actor, id string) (account string, signature string, err error) {
	f, err := s.crm.Get(ctx, actor, id)
	if err != nil {
		return "", "", err
	}
	_, sender, err := s.sender(ctx, actor, f)
	if err != nil {
		return "", "", err
	}
	mailbox, err := s.conns.Google(ctx, sender)
	if err != nil {
		return "", "", err
	}
	sig, err := mailbox.Signature(ctx, sender.Account)
	return sender.Account, sig.Text, err
}

// sender finds the newest message of the email conversations a follow-up is
// linked to, and the mailbox a reply to it goes from: the one holding it when
// the actor may send from it, else their own.
func (s *Service) sender(ctx context.Context, actor auth.Actor, f records.Record) (storage.Part, storage.Connection, error) {
	threads, err := s.store.Timeline(ctx, storage.TimelineQuery{RecordID: f.ID, WorkspaceID: actor.WorkspaceID, Kinds: []string{interactions.Email}, Limit: 20})
	if err != nil {
		return storage.Part{}, storage.Connection{}, err
	}
	ids := make([]string, len(threads))
	for i, t := range threads {
		ids[i] = t.ID
	}
	parts, err := s.store.Parts(ctx, ids)
	if err != nil {
		return storage.Part{}, storage.Connection{}, err
	}
	parts = slices.DeleteFunc(parts, func(p storage.Part) bool { return p.Kind != "message" || p.ConnectionID == nil || p.ProviderID == nil })
	if len(parts) == 0 {
		return storage.Part{}, storage.Connection{}, errs.Invalidf("link the email conversation this follow-up replies in")
	}
	last := slices.MaxFunc(parts, func(a, b storage.Part) int { return a.At.Compare(*b.At) })
	sender, err := s.conns.Mailbox(ctx, actor, *last.ConnectionID)
	return last, sender, err
}

// reply addresses a follow-up's draft as a reply to the newest message of
// the email conversations it is linked to, from its sender.
func (s *Service) reply(ctx context.Context, actor auth.Actor, f records.Record) (outgoing, error) {
	body := value(f, "draft")
	to := values(f, "to")
	if strings.TrimSpace(body) == "" || len(to) == 0 {
		return outgoing{}, errs.Invalidf("an email draft needs text and a recipient")
	}
	last, sender, err := s.sender(ctx, actor, f)
	if err != nil {
		return outgoing{}, err
	}
	written, err := s.draftedAt(ctx, actor, f.ID)
	if err != nil {
		return outgoing{}, err
	}
	if last.At.After(written) {
		return outgoing{}, errs.Invalidf("a message arrived after this draft was written; review the draft first")
	}
	original, holder, err := s.original(ctx, last)
	if err != nil {
		return outgoing{}, err
	}
	mailbox, err := s.conns.Google(ctx, sender)
	if err != nil {
		return outgoing{}, err
	}
	signature, err := mailbox.Signature(ctx, sender.Account)
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
		From: sender.Account, To: to, Cc: values(f, "cc"), Subject: subject, Body: body, Signature: signature,
		ThreadID: thread, InReplyTo: original.MessageID, References: references,
	}}, nil
}

// original reads a message's headers from the mailbox that holds it.
func (s *Service) original(ctx context.Context, part storage.Part) (google.Message, storage.Connection, error) {
	holder, err := s.conns.Connection(ctx, *part.ConnectionID)
	if err != nil {
		return google.Message{}, holder, err
	}
	mailbox, err := s.conns.Google(ctx, holder)
	if err != nil {
		return google.Message{}, holder, err
	}
	found, err := mailbox.Messages(ctx, []string{*part.ProviderID}, false)
	if err != nil {
		return google.Message{}, holder, err
	}
	if found[0].ID == "" {
		return google.Message{}, holder, errs.Invalidf("the message to reply to is gone from %s", holder.Account)
	}
	return found[0], holder, nil
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

// refs lists the records a reference attribute points at.
func refs(r records.Record, attribute string) []string {
	var out []string
	for _, f := range r.Fields {
		if f.Attribute == attribute {
			for _, v := range f.Values {
				out = append(out, v.RecordID)
			}
		}
	}
	return out
}
