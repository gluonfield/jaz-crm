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
	crm       *records.Service
	store     storage.InteractionStore
	conns     *connections.Service
	addresses storage.ConnectionStore
}

func NewService(crm *records.Service, store storage.InteractionStore, conns *connections.Service, addresses storage.ConnectionStore) *Service {
	return &Service{crm: crm, store: store, conns: conns, addresses: addresses}
}

// SaveDraft records an edit in the CRM composer with the same source on
// both transports, so a browser edit remains editable inside an MCP app.
type SavedDraft struct {
	Record   records.Record
	Revision string
}

func (s *Service) SaveDraft(ctx context.Context, actor auth.Actor, id, draft, subject, channel string, to, cc []string, revisions ...string) (SavedDraft, error) {
	imported, err := s.store.GmailDraft(ctx, actor.WorkspaceID, id)
	if err == nil && imported.State == "draft" {
		revision := ""
		if len(revisions) > 0 {
			revision = revisions[0]
		}
		return s.saveImported(ctx, actor, imported, revision, draft, subject, channel, to, cc)
	}
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return SavedDraft{}, err
	}
	record, err := s.saveRecordDraft(ctx, actor, id, draft, subject, channel, to, cc)
	return SavedDraft{Record: record}, err
}

func (s *Service) saveRecordDraft(ctx context.Context, actor auth.Actor, id, draft, subject, channel string, to, cc []string) (records.Record, error) {
	current, err := s.crm.Get(ctx, actor, id)
	if err != nil {
		return records.Record{}, err
	}
	if current.Object != records.FollowUps {
		return records.Record{}, errs.Invalidf("%s is not a follow-up", id)
	}
	set := map[string][]string{}
	remove := map[string][]string{}
	for attribute, next := range map[string][]string{"draft": {draft}, "subject": {subject}, "channel": {channel}, "to": to, "cc": cc} {
		if len(next) == 1 && strings.TrimSpace(next[0]) == "" {
			next = nil
		}
		if len(next) == 0 {
			remove[attribute] = nil
			continue
		}
		set[attribute] = next
		if dropped := slices.DeleteFunc(values(current, attribute), func(value string) bool {
			return slices.Contains(next, value)
		}); len(dropped) > 0 {
			remove[attribute] = dropped
		}
	}
	record, _, err := s.crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: id, Set: set, Remove: remove})
	return record, err
}

// Seen is the draft a person released, as they saw it, with the mailbox an
// email draft goes from.
type Seen struct {
	Confirmed bool
	Draft     string
	Subject   string
	From      string
	To, Cc    []string
	Bcc       []string
	Revision  string
}

// Release is a person letting a follow-up's draft go. An email draft is sent
// now, replying in its newest linked email conversation when present; any other draft is
// approved for whoever sends it. Confirmation and the reviewed draft are
// required on both browser and connected-app transports.
func (s *Service) Release(ctx context.Context, actor auth.Actor, id string, seen Seen) (records.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if !seen.Confirmed {
		return records.Record{}, errs.Invalidf("confirm the reply before sending")
	}
	f, err := s.crm.Get(ctx, actor, id)
	if err != nil {
		return records.Record{}, err
	}
	if f.Object != records.FollowUps {
		return records.Record{}, errs.Invalidf("%s is not a follow-up", id)
	}
	if strings.TrimSpace(value(f, "draft")) != strings.TrimSpace(seen.Draft) {
		return records.Record{}, errs.Invalidf("the draft changed since you saw it; review it again")
	}
	if value(f, "channel") != "Email" {
		if !sameSet(values(f, "to"), seen.To) || !sameSet(values(f, "cc"), seen.Cc) {
			return records.Record{}, errs.Invalidf("the draft changed since you saw it; review it again")
		}
		return s.set(ctx, actor, id, "draft_status", records.DraftApproved)
	}
	reply, err := s.compose(ctx, actor, f)
	if err != nil {
		return records.Record{}, err
	}
	if !(google.Message{Text: reply.message.Body, HTML: reply.html}).HasDraftText() || len(reply.message.To)+len(reply.message.Cc)+len(reply.bcc) == 0 {
		return records.Record{}, errs.Invalidf("an email draft needs text and a recipient")
	}
	if reply.message.Subject == "" {
		return records.Record{}, errs.Invalidf("add a subject before sending")
	}
	if reply.message.Subject != seen.Subject {
		return records.Record{}, errs.Invalidf("the subject changed since you saw it; review it again")
	}
	if reply.latest != nil {
		written, err := s.draftedAt(ctx, actor, f.ID)
		if err != nil {
			return records.Record{}, err
		}
		if reply.latest.After(written) {
			return records.Record{}, errs.Invalidf("a message arrived after this draft was written; review the draft first")
		}
	}
	if reply.account != seen.From {
		return records.Record{}, errs.Invalidf("this reply now goes from %s; review it again", reply.account)
	}
	if !sameSet(reply.message.To, seen.To) || !sameSet(reply.message.Cc, seen.Cc) {
		return records.Record{}, errs.Invalidf("the reply recipients changed since you saw them; review them again")
	}
	if reply.imported != nil && (seen.Revision != reply.imported.MessageID || !sameSet(seen.Bcc, reply.bcc)) {
		return records.Record{}, errs.Invalidf("the Gmail draft changed since you reviewed it; review it again")
	}
	expect := map[string][]string{}
	for _, attribute := range []string{"draft", "subject", "channel", "to", "cc", "draft_status"} {
		expect[attribute] = values(f, attribute)
	}
	remove := map[string][]string{}
	for attribute, addresses := range map[string][]string{"to": seen.To, "cc": seen.Cc} {
		if dropped := slices.DeleteFunc(values(f, attribute), func(address string) bool { return slices.Contains(addresses, address) }); len(dropped) > 0 {
			remove[attribute] = dropped
		}
	}
	if _, _, err := s.crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: id,
		Set: map[string][]string{"draft_status": {records.DraftSending}, "to": seen.To, "cc": seen.Cc}, Remove: remove, Expect: expect,
	}); err != nil {
		return records.Record{}, err
	}
	sentID, err := reply.send(ctx)
	// Once Gmail answers, save its outcome even if the browser disconnected.
	settle, done := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer done()
	var api *google.APIError
	var invalid errs.Invalid
	switch {
	case errors.As(err, &invalid):
		_, undo := s.set(settle, actor, id, "draft_status", records.DraftApproved)
		return records.Record{}, errors.Join(err, undo)
	case errors.Is(err, google.ErrRevoked) || errors.As(err, &api) && api.Status >= 400 && api.Status < 500 && api.Status != http.StatusRequestTimeout:
		if _, undo := s.set(settle, actor, id, "draft_status", records.DraftApproved); undo != nil {
			return records.Record{}, errors.Join(err, undo)
		}
		if errors.Is(err, google.ErrRevoked) || api.Reason == "insufficientPermissions" {
			return records.Record{}, errs.Invalidf("%s must reconnect Google to send mail from the CRM", reply.account)
		}
		return records.Record{}, errs.Invalidf("Google refused to send the email: %s (%s). Review the error before trying again", api.Message, api.Reason)
	case err != nil:
		return records.Record{}, errs.Invalidf("the reply may have gone out from %s: check its Sent mail, then set the draft back to Approved to send it again", reply.account)
	}
	if reply.imported != nil {
		var record records.Record
		err := s.store.Atomically(settle, func(store storage.InteractionStore) error {
			snapshot := *reply.imported
			if err := store.LockGmailDraft(settle, snapshot.ConnectionID, snapshot.DraftID); err != nil {
				return err
			}
			snapshot, err := store.GmailDraftByID(settle, snapshot.ConnectionID, snapshot.DraftID)
			if err != nil {
				return err
			}
			writer := *s
			writer.store, writer.crm = store, records.NewService(store)
			if snapshot.State == "sent" {
				if snapshot.SentMessageID != sentID {
					return errs.Invalidf("Gmail reported a different sent message; review its Sent mail")
				}
				record, err = writer.crm.Get(settle, actor, id)
				return err
			}
			snapshot.State, snapshot.SentMessageID = "sent", sentID
			if err := store.UpsertGmailDraft(settle, snapshot); err != nil {
				return err
			}
			record, err = writer.set(settle, actor, id, "draft_status", records.DraftSent, "status", "Done")
			return err
		})
		return record, err
	}
	return s.set(settle, actor, id, "draft_status", records.DraftSent, "status", "Done")
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
	mailbox     *google.Client
	account     string
	message     google.Outgoing
	latest      *time.Time
	imported    *storage.GmailDraft
	bcc         []string
	attachments []string
	html        string
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
		sender, err := s.conns.Mailbox(ctx, actor, "")
		return storage.Part{}, sender, err
	}
	last := slices.MaxFunc(parts, func(a, b storage.Part) int { return a.At.Compare(*b.At) })
	sender, err := s.conns.Mailbox(ctx, actor, *last.ConnectionID)
	return last, sender, err
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
	out := []string{}
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
