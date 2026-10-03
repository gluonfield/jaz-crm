package postgres

import (
	"context"
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	intdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/interactions"
)

func toGmailDraft(d intdb.GmailDraft) storage.GmailDraft {
	return storage.GmailDraft{
		ConnectionID: d.ConnectionID, DraftID: d.DraftID, MessageID: d.MessageID,
		RFCMessageID: d.RfcMessageID, ThreadID: d.ThreadID, FollowUpID: d.FollowUpID,
		Subject: d.Subject, Body: d.Body, HTML: d.HTML, From: d.Sender,
		To: d.Recipients, Cc: d.Cc, Bcc: d.Bcc, State: d.State, SentMessageID: d.SentMessageID,
		At: d.At, Attachments: d.Attachments,
	}
}

func (s *Store) LockGmailDraft(ctx context.Context, connectionID, draftID string) error {
	return mapError(s.in.LockGmailDraft(ctx, intdb.LockGmailDraftParams{ConnectionID: connectionID, DraftID: draftID}))
}

func (s *Store) GmailDrafts(ctx context.Context, connectionID string) ([]storage.GmailDraft, error) {
	return many(toGmailDraft)(s.in.GmailDrafts(ctx, connectionID))
}

func (s *Store) GmailDraftByID(ctx context.Context, connectionID, draftID string) (storage.GmailDraft, error) {
	return one(toGmailDraft)(s.in.GmailDraftByID(ctx, intdb.GmailDraftByIDParams{ConnectionID: connectionID, DraftID: draftID}))
}

func (s *Store) GmailDraft(ctx context.Context, workspaceID, followUpID string) (storage.GmailDraft, error) {
	return one(toGmailDraft)(s.in.GmailDraft(ctx, intdb.GmailDraftParams{WorkspaceID: workspaceID, FollowUpID: &followUpID}))
}

func (s *Store) InteractionDrafts(ctx context.Context, interactionIDs []string) ([]storage.GmailDraft, error) {
	return many(func(row intdb.InteractionDraftsRow) storage.GmailDraft {
		draft := toGmailDraft(row.GmailDraft)
		draft.InteractionID = row.InteractionID
		return draft
	})(s.in.InteractionDrafts(ctx, interactionIDs))
}

func (s *Store) UpsertGmailDraft(ctx context.Context, d storage.GmailDraft) error {
	return s.inTx(ctx, func(q *intdb.Queries) error {
		if err := q.UpsertGmailDraft(ctx, intdb.UpsertGmailDraftParams{
			ConnectionID: d.ConnectionID, DraftID: d.DraftID, MessageID: d.MessageID,
			RfcMessageID: d.RFCMessageID, ThreadID: d.ThreadID,
			Subject: d.Subject, Body: d.Body, HTML: d.HTML, Sender: d.From,
			Recipients: d.To, Cc: d.Cc, Bcc: d.Bcc, State: d.State, SentMessageID: d.SentMessageID,
			At: d.At, Attachments: d.Attachments,
		}); err != nil {
			return err
		}
		if d.State == "sent" {
			return q.LinkKnownSentGmailDrafts(ctx, intdb.LinkKnownSentGmailDraftsParams{ConnectionID: d.ConnectionID, ProviderID: d.SentMessageID})
		}
		return nil
	})
}

func (s *Store) LinkSentGmailDrafts(ctx context.Context, connectionID, providerID, interactionID string) error {
	return mapError(s.in.LinkSentGmailDrafts(ctx, intdb.LinkSentGmailDraftsParams{ConnectionID: connectionID, ProviderID: providerID, InteractionID: interactionID}))
}

func (s *Store) BindGmailDraft(ctx context.Context, connectionID, draftID, followUpID string) error {
	return affected(s.in.BindGmailDraft(ctx, intdb.BindGmailDraftParams{
		ConnectionID: connectionID, DraftID: draftID, FollowUpID: &followUpID,
	}))
}

func (s *Store) RepairDraftMessage(ctx context.Context, connectionID, providerID string) ([]string, error) {
	var ids []string
	err := s.inTx(ctx, func(q *intdb.Queries) error {
		var err error
		ids, err = q.MigrateDraftMessage(ctx, intdb.MigrateDraftMessageParams{ConnectionID: &connectionID, ProviderID: &providerID})
		if err != nil {
			return err
		}
		slices.Sort(ids)
		ids = slices.Compact(ids)
		return q.RepairDraftInteractions(ctx, ids)
	})
	return ids, err
}
