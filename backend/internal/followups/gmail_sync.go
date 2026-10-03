package followups

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// SyncGmailDrafts snapshots the provider's stable draft identities. Disappearance
// clears the composer; only a SENT message establishes that a draft was sent.
func (s *Service) SyncGmailDrafts(ctx context.Context, connection storage.Connection, mailbox *google.Client) error {
	stored, err := s.store.GmailDrafts(ctx, connection.ID)
	if err != nil {
		return err
	}
	current := map[string]bool{}
	pace := time.NewTicker(100 * time.Millisecond)
	defer pace.Stop()
	for page := ""; ; {
		list, err := mailbox.Drafts(ctx, page)
		if err != nil {
			return err
		}
		for _, ref := range list.Drafts {
			current[ref.ID] = true
			i := slices.IndexFunc(stored, func(d storage.GmailDraft) bool { return d.DraftID == ref.ID })
			if i >= 0 && stored[i].MessageID == ref.Message.ID && stored[i].FollowUpID != nil && stored[i].State == "draft" {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-pace.C:
			}
			draft, err := mailbox.Draft(ctx, ref.ID)
			if errors.Is(err, google.ErrNotFound) {
				delete(current, ref.ID)
				continue
			}
			if err != nil {
				return err
			}
			expected := ""
			if i >= 0 {
				expected = stored[i].MessageID
			}
			if err := s.importGmailDraft(ctx, connection, draft, expected); err != nil {
				return err
			}
		}
		if page = list.Next; page == "" {
			break
		}
	}
	for _, old := range stored {
		if current[old.DraftID] {
			continue
		}
		old.State = "missing"
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pace.C:
		}
		messages, err := mailbox.ThreadMessages(ctx, old.ThreadID)
		if err != nil && !errors.Is(err, google.ErrNotFound) {
			return err
		}
		var sent []google.Message
		for _, message := range messages {
			if sentDraft(old, message) {
				sent = append(sent, message)
			}
		}
		if len(sent) == 1 {
			old.State, old.SentMessageID = "sent", sent[0].ID
		}
		if err := s.finishGmailDraft(ctx, connection, old); err != nil {
			return err
		}
		if len(sent) == 1 && old.FollowUpID != nil {
			thread, err := s.store.EmailThreadByMessageIDs(ctx, connection.WorkspaceID, []string{sent[0].MessageID})
			if err != nil && !errors.Is(err, storage.ErrNotFound) {
				return err
			}
			if err == nil {
				if err := s.store.AddLink(ctx, thread, *old.FollowUpID, interactions.ByAgent); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Service) importGmailDraft(ctx context.Context, connection storage.Connection, draft google.Draft, expectedRevision string) error {
	m := draft.Message
	own, err := s.addresses.InternalAddresses(ctx, connection.WorkspaceID)
	if err != nil {
		return err
	}
	ready := m.HasDraftText() && slices.ContainsFunc(slices.Concat(m.To, m.Cc, m.Bcc), func(address google.Address) bool {
		return address.Email != "" && !slices.Contains(own, address.Email)
	})
	return s.store.Atomically(ctx, func(store storage.InteractionStore) error {
		if err := store.LockGmailDraft(ctx, connection.ID, draft.ID); err != nil {
			return err
		}
		current, err := store.GmailDraftByID(ctx, connection.ID, draft.ID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return err
		}
		if current.State == "sent" || current.MessageID != expectedRevision && current.MessageID != m.ID {
			return nil
		}
		crm := records.NewService(store)
		actor := auth.Actor{WorkspaceID: connection.WorkspaceID, UserID: connection.UserID}
		if current.FollowUpID != nil {
			record, err := crm.Get(ctx, actor, *current.FollowUpID)
			if err != nil {
				return err
			}
			if value(record, "draft_status") == records.DraftSending {
				return nil
			}
		}
		affected, err := store.RepairDraftMessage(ctx, connection.ID, m.ID)
		if err != nil {
			return err
		}
		if current.MessageID != "" && current.MessageID != m.ID {
			previous, err := store.RepairDraftMessage(ctx, connection.ID, current.MessageID)
			if err != nil {
				return err
			}
			affected = append(affected, previous...)
		}
		snapshot := storage.GmailDraft{ConnectionID: connection.ID, DraftID: draft.ID, MessageID: m.ID, RFCMessageID: m.MessageID, ThreadID: m.ThreadID,
			Subject: m.Subject, Body: m.Text, HTML: m.HTML, From: m.From.Email, At: m.Date, Attachments: m.Attachments, To: emails(m.To), Cc: emails(m.Cc), Bcc: emails(m.Bcc), State: "draft"}
		if err := store.UpsertGmailDraft(ctx, snapshot); err != nil {
			return err
		}
		if !ready && current.FollowUpID == nil {
			return nil
		}
		if thread, err := store.EmailThreadByMessageIDs(ctx, connection.WorkspaceID, append(slices.Clone(m.References), m.InReplyTo)); err == nil {
			affected = append(affected, thread)
		} else if !errors.Is(err, storage.ErrNotFound) {
			return err
		}
		if thread, err := store.InteractionByExternalID(ctx, connection.WorkspaceID, "gmail", m.ThreadID); err == nil {
			affected = append(affected, thread)
		} else if !errors.Is(err, storage.ErrNotFound) {
			return err
		}
		id := ""
		if current.FollowUpID != nil {
			id = *current.FollowUpID
		} else {
			if err := store.LockWorkspace(ctx, connection.WorkspaceID); err != nil {
				return err
			}
			links, err := store.Links(ctx, affected)
			if err != nil {
				return err
			}
			for _, link := range links {
				record, err := crm.Get(ctx, actor, link.RecordID)
				if err != nil {
					return err
				}
				parts, err := store.Parts(ctx, []string{link.InteractionID})
				if err != nil {
					return err
				}
				_, bound := store.GmailDraft(ctx, actor.WorkspaceID, record.ID)
				if record.Object == records.FollowUps && link.Source == interactions.ByAgent && errors.Is(bound, storage.ErrNotFound) && (!slices.ContainsFunc(parts, func(p storage.Part) bool { return p.Kind == "message" }) && value(record, "waiting_on") == "Them" || value(record, "status") == "Open" && value(record, "waiting_on") != "Them" && value(record, "channel") == "Email" && value(record, "draft") != "") {
					id = record.ID
					break
				}
			}
		}
		set := map[string][]string{"status": {"Open"}, "waiting_on": {"Us"}, "channel": {"Email"}}
		if !ready {
			set["status"] = []string{"Dismissed"}
		}
		remove := map[string][]string{}
		if current.FollowUpID == nil {
			set["name"], set["owner"] = []string{"Review email: " + m.Subject}, []string{connection.Account}
			remove["action_date"] = nil
		}
		for attribute, next := range map[string][]string{"draft": {m.Text}, "subject": {m.Subject}, "to": snapshot.To, "cc": snapshot.Cc} {
			if len(next) == 0 || len(next) == 1 && strings.TrimSpace(next[0]) == "" {
				remove[attribute] = nil
			} else {
				set[attribute] = next
			}
		}
		if id != "" {
			previous, err := crm.Get(ctx, actor, id)
			if err != nil {
				return err
			}
			if value(previous, "draft_status") == records.DraftSending {
				return nil
			}
			if value(previous, "action_date_basis") == "Manual" {
				delete(remove, "action_date")
			}
			for _, attribute := range []string{"to", "cc"} {
				if dropped := slices.DeleteFunc(values(previous, attribute), func(address string) bool { return slices.Contains(set[attribute], address) }); len(dropped) > 0 {
					remove[attribute] = dropped
				}
			}
		}
		handles, err := store.HandlesByValue(ctx, connection.WorkspaceID, append(slices.Clone(snapshot.To), snapshot.Cc...))
		if err != nil {
			return err
		}
		for _, handle := range handles {
			if handle.PersonID != nil {
				set["person"] = []string{*handle.PersonID}
				break
			}
		}
		record, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: id, Set: set, Remove: remove})
		if err != nil {
			return err
		}
		if err := store.BindGmailDraft(ctx, connection.ID, draft.ID, record.ID); err != nil {
			return err
		}
		for _, thread := range affected {
			if err := store.AddLink(ctx, thread, record.ID, interactions.ByAgent); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) finishGmailDraft(ctx context.Context, connection storage.Connection, draft storage.GmailDraft) error {
	return s.store.Atomically(ctx, func(store storage.InteractionStore) error {
		if err := store.LockGmailDraft(ctx, connection.ID, draft.DraftID); err != nil {
			return err
		}
		held, err := store.GmailDraftByID(ctx, connection.ID, draft.DraftID)
		if err != nil {
			return err
		}
		if held.MessageID != draft.MessageID || held.State == "sent" && draft.State != "sent" {
			return nil
		}
		if held.State != "draft" || draft.FollowUpID == nil {
			if held.State == draft.State {
				return nil
			}
			return store.UpsertGmailDraft(ctx, draft)
		}
		crm := records.NewService(store)
		actor := auth.Actor{WorkspaceID: connection.WorkspaceID, UserID: connection.UserID}
		current, err := crm.Get(ctx, actor, *draft.FollowUpID)
		if err != nil {
			return err
		}
		if value(current, "draft_status") == records.DraftSending && draft.State != "sent" {
			return nil
		}
		if err := store.UpsertGmailDraft(ctx, draft); err != nil {
			return err
		}
		if draft.State == "sent" && value(current, "draft") != "" {
			if value(current, "draft_status") != records.DraftSending {
				if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: current.ID, Set: map[string][]string{"draft_status": {records.DraftSending}}}); err != nil {
					return err
				}
			}
			_, _, err = crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: current.ID, Set: map[string][]string{"status": {"Done"}, "draft_status": {records.DraftSent}}})
			return err
		}
		if draft.State == "sent" || value(current, "draft") == "" && value(current, "status") != "Open" {
			return nil
		}
		_, _, err = crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: current.ID,
			Set: map[string][]string{"status": {"Dismissed"}}, Remove: map[string][]string{"draft": nil, "subject": nil, "draft_status": nil, "action_date": nil}})
		return err
	})
}

func emails(addresses []google.Address) []string {
	result := make([]string, len(addresses))
	for i, address := range addresses {
		result[i] = address.Email
	}
	return result
}

func sentDraft(draft storage.GmailDraft, message google.Message) bool {
	return slices.Contains(message.Labels, "SENT") && !message.Date.Before(draft.At) &&
		message.ThreadID == draft.ThreadID && message.From.Email == draft.From &&
		message.Subject == draft.Subject && message.Text == draft.Body && message.HTML == draft.HTML &&
		sameSet(emails(message.To), draft.To) && sameSet(emails(message.Cc), draft.Cc) && sameSet(emails(message.Bcc), draft.Bcc) &&
		sameSet(message.Attachments, draft.Attachments)
}
