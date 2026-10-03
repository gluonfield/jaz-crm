package storage

import (
	"context"
	"time"
)

type GmailDraft struct {
	InteractionID string
	ConnectionID  string
	DraftID       string
	MessageID     string
	RFCMessageID  string
	ThreadID      string
	FollowUpID    *string
	Subject       string
	Body          string
	HTML          string
	From          string
	To            []string
	Cc            []string
	Bcc           []string
	State         string
	SentMessageID string
	At            time.Time
	Attachments   []string
}

type DraftStore interface {
	// LockGmailDraft serializes a provider draft inside Atomically, including its first import.
	LockGmailDraft(ctx context.Context, connectionID, draftID string) error
	// GmailDrafts lists active drafts and recently missing drafts awaiting sent evidence.
	GmailDrafts(ctx context.Context, connectionID string) ([]GmailDraft, error)
	GmailDraftByID(ctx context.Context, connectionID, draftID string) (GmailDraft, error)
	GmailDraft(ctx context.Context, workspaceID, followUpID string) (GmailDraft, error)
	InteractionDrafts(ctx context.Context, interactionIDs []string) ([]GmailDraft, error)
	UpsertGmailDraft(ctx context.Context, draft GmailDraft) error
	BindGmailDraft(ctx context.Context, connectionID, draftID, followUpID string) error
	RepairDraftMessage(ctx context.Context, connectionID, providerID string) ([]string, error)
	LinkSentGmailDrafts(ctx context.Context, connectionID, providerID, interactionID string) error
}
