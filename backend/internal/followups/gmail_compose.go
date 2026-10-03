package followups

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func (s *Service) importedMailbox(ctx context.Context, actor auth.Actor, draft storage.GmailDraft) (storage.Connection, *google.Client, error) {
	connection, err := s.conns.Mailbox(ctx, actor, draft.ConnectionID)
	if err != nil {
		return connection, nil, err
	}
	if connection.ID != draft.ConnectionID {
		return connection, nil, errs.Invalidf("this Gmail draft belongs to another mailbox; its owner must allow teammates to send")
	}
	mailbox, err := s.conns.Google(ctx, connection)
	return connection, mailbox, err
}

func (s *Service) composeImported(ctx context.Context, actor auth.Actor, f records.Record, snapshot storage.GmailDraft) (outgoing, error) {
	if snapshot.State != "draft" {
		return outgoing{}, errs.Invalidf("this draft is no longer in Gmail Drafts; refresh the conversation")
	}
	_, mailbox, err := s.importedMailbox(ctx, actor, snapshot)
	if err != nil {
		return outgoing{}, err
	}
	draft, err := mailbox.Draft(ctx, snapshot.DraftID)
	if errors.Is(err, google.ErrNotFound) {
		return outgoing{}, errs.Invalidf("this draft was sent or removed in Gmail; refresh the conversation")
	}
	if err != nil {
		return outgoing{}, err
	}
	if draft.Message.ID != snapshot.MessageID || strings.TrimSpace(draft.Message.Text) != value(f, "draft") || draft.Message.Subject != value(f, "subject") || !sameSet(emails(draft.Message.To), values(f, "to")) || !sameSet(emails(draft.Message.Cc), values(f, "cc")) {
		return outgoing{}, errs.Invalidf("the draft changed in Gmail; wait for sync and review the latest draft")
	}
	message := google.Outgoing{From: draft.Message.From, Body: draft.Message.Text, Subject: draft.Message.Subject, To: emails(draft.Message.To), Cc: emails(draft.Message.Cc), ThreadID: draft.Message.ThreadID}
	return outgoing{mailbox: mailbox, account: draft.Message.From.Email, message: message, imported: &snapshot, bcc: emails(draft.Message.Bcc), attachments: draft.Message.Attachments, html: draft.Message.HTML}, nil
}

func (s *Service) saveImported(ctx context.Context, actor auth.Actor, snapshot storage.GmailDraft, revision, body, subject, channel string, to, cc []string) (SavedDraft, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result SavedDraft
	err := s.store.Atomically(ctx, func(store storage.InteractionStore) error {
		if err := store.LockGmailDraft(ctx, snapshot.ConnectionID, snapshot.DraftID); err != nil {
			return err
		}
		writer := *s
		writer.store, writer.crm = store, records.NewService(store)
		var err error
		result, err = writer.saveImportedLocked(ctx, actor, snapshot, revision, body, subject, channel, to, cc)
		return err
	})
	return result, err
}

func (s *Service) saveImportedLocked(ctx context.Context, actor auth.Actor, snapshot storage.GmailDraft, revision, body, subject, channel string, to, cc []string) (SavedDraft, error) {
	if channel != "Email" {
		return SavedDraft{}, errs.Invalidf("a Gmail draft must remain an email")
	}
	if revision == "" || revision != snapshot.MessageID || snapshot.State != "draft" {
		return SavedDraft{}, errs.Invalidf("the Gmail draft changed; review it again before saving")
	}
	connection, mailbox, err := s.importedMailbox(ctx, actor, snapshot)
	if err != nil {
		return SavedDraft{}, err
	}
	current, err := mailbox.Draft(ctx, snapshot.DraftID)
	if err != nil {
		return SavedDraft{}, err
	}
	if current.Message.ID != revision {
		return SavedDraft{}, errs.Invalidf("the draft changed in Gmail; review it again before saving")
	}
	if _, err := s.saveRecordDraft(ctx, actor, *snapshot.FollowUpID, body, subject, channel, to, cc); err != nil {
		return SavedDraft{}, err
	}
	if body != current.Message.Text || subject != current.Message.Subject || !sameSet(to, emails(current.Message.To)) || !sameSet(cc, emails(current.Message.Cc)) {
		raw, err := mailbox.DraftRaw(ctx, snapshot.DraftID)
		if err != nil {
			return SavedDraft{}, err
		}
		if raw.Message.ID != revision {
			return SavedDraft{}, errs.Invalidf("the draft changed in Gmail; review it again before saving")
		}
		mime, err := google.EditDraft(raw.Raw, current.Message, google.Outgoing{Body: body, Subject: subject, To: to, Cc: cc})
		if err != nil {
			return SavedDraft{}, err
		}
		updated, err := mailbox.UpdateDraft(ctx, snapshot.DraftID, snapshot.ThreadID, mime)
		if err != nil {
			return SavedDraft{}, err
		}
		revision = updated.Message.ID
		current, err = mailbox.Draft(ctx, snapshot.DraftID)
		if err != nil {
			return SavedDraft{}, err
		}
	}
	if current.Message.ID != revision {
		return SavedDraft{}, errs.Invalidf("the draft changed in Gmail while saving; review it again")
	}
	if err := s.importGmailDraft(ctx, connection, current, snapshot.MessageID); err != nil {
		return SavedDraft{}, err
	}
	record, err := s.crm.Get(ctx, actor, *snapshot.FollowUpID)
	return SavedDraft{Record: record, Revision: revision}, err
}

func (out outgoing) send(ctx context.Context) (string, error) {
	if out.imported == nil {
		return out.mailbox.Send(ctx, out.message)
	}
	raw, err := out.mailbox.DraftRaw(ctx, out.imported.DraftID)
	if err != nil {
		return "", errs.Invalidf("could not recheck the Gmail draft before sending: %v", err)
	}
	if raw.Message.ID != out.imported.MessageID {
		return "", errs.Invalidf("the draft changed in Gmail after confirmation; review it again")
	}
	// Send the exact reviewed MIME, retaining its attachments, HTML and Bcc.
	return out.mailbox.SendDraft(ctx, out.imported.DraftID, out.imported.ThreadID, raw.Raw)
}
