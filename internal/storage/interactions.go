package storage

import (
	"context"
	"time"
)

// Handle is an address seen in a workspace and its triage verdict.
type Handle struct {
	ID          string
	WorkspaceID string
	Kind        string
	Value       string
	Name        string
	PersonID    *string
	Triage      string
	DecidedBy   *string
	Reason      string
	CreatedAt   time.Time
	// PhotoURL is the address's profile picture, when Google has one.
	PhotoURL string
}

type NewHandle struct {
	WorkspaceID string
	Kind        string
	Value       string
	Name        string
	Triage      string
	DecidedBy   *string
	Reason      string
	ID          string
}

type Verdict struct {
	WorkspaceID string
	ID          string
	Triage      string
	DecidedBy   *string
	Reason      string
	PersonID    *string
}

type HandleQuery struct {
	WorkspaceID string
	Triage      string
	Query       *string
	Limit       int32
}

type HandleSummary struct {
	Handle       Handle
	Interactions int32
	LastSeen     time.Time
}

type HandleRecord struct {
	ID       string
	RecordID string
}

type UnassessedHandle struct {
	ID     string
	Value  string
	Name   string
	Titles []string
}

type Interaction struct {
	ID          string
	WorkspaceID string
	Kind        string
	ExternalID  string
	// ConnectionID is the account a synced conversation came from; logged
	// conversations have none.
	ConnectionID        *string
	UserID              *string
	Title               string
	StartedAt           time.Time
	EndedAt             *time.Time
	MeetCode            string
	TranscriptCheckedAt *time.Time
	Skipped             bool
	CreatedAt           time.Time
	Channel             string
	DateOnly            bool
	// FollowedUpAt is the newest content the follow-up agent has read.
	FollowedUpAt      *time.Time
	DraftingState     string
	DraftingReason    string
	DraftingStartedAt *time.Time
	// URL opens the original: a Gmail thread, calendar event or chat thread.
	URL string
}

type NewInteraction struct {
	WorkspaceID  string
	Kind         string
	Channel      string
	ExternalID   string
	ConnectionID *string
	UserID       *string
	Title        string
	StartedAt    time.Time
	EndedAt      *time.Time
	MeetCode     string
	Skipped      bool
	URL          string
	DateOnly     bool
	ID           string
}

// EmailThread widens a thread's time span to include a message.
type EmailThread struct {
	WorkspaceID  string
	ExternalID   string
	ConnectionID *string
	UserID       *string
	Title        string
	At           time.Time
	URL          string
	ID           string
}

type Participant struct {
	InteractionID string
	Role          string
	Handle        Handle
}

type NewPart struct {
	InteractionID  string
	Kind           string
	ExternalID     string
	ConnectionID   *string
	ProviderID     *string
	AuthorHandleID *string
	AuthorName     string
	At             *time.Time
	Content        *string
	Recipients     []string
	Direction      string
	DateOnly       bool
	Partial        bool
	Position       int32
}

type Part struct {
	ID            int64
	InteractionID string
	Kind          string
	ExternalID    string
	// ConnectionID and ProviderID locate a message in the mailbox it came from.
	ConnectionID   *string
	ProviderID     *string
	AuthorHandleID *string
	AuthorName     string
	At             *time.Time
	Content        *string
	Recipients     []string
	Direction      string
	DateOnly       bool
	Partial        bool
	Position       int32
	HTML           *string
}

type Link struct {
	InteractionID string
	RecordID      string
	Source        string
}

type UnfetchedPart struct {
	ID         int64
	ProviderID string
}

type TimelineQuery struct {
	RecordID    string
	Cursor      string
	WorkspaceID string
	Kinds       []string
	Upcoming    bool
	Limit       int32
}

type Activity struct {
	RecordID      string
	Interactions  int32
	FirstAt       time.Time
	LastAt        time.Time
	FirstDateOnly bool
	LastDateOnly  bool
}

// LastMessage is a record's latest message, its conversation's channel and
// its sender's address.
type LastMessage struct {
	RecordID      string
	Channel       string
	At            time.Time
	DateOnly      bool
	AuthorName    string
	Content       string
	Direction     string
	SenderAddress string
	SenderName    string
	PersonID      *string
}

// FollowUpCandidate is a conversation with content newer than what the
// follow-up agent last read.
type FollowUpCandidate struct {
	ID           string
	FollowedUpAt *time.Time
	LatestAt     time.Time
}

type DomainRule struct {
	WorkspaceID string
	Domain      string
	Triage      string
	Reason      string
	CreatedAt   time.Time
}

type InteractionStore interface {
	RecordStore
	DraftStore
	// Atomically runs fn against the store within one transaction.
	Atomically(ctx context.Context, fn func(InteractionStore) error) error
	LockWorkspace(ctx context.Context, workspaceID string) error
	SetDomainRule(ctx context.Context, workspaceID, domain, triage, reason string) error
	DomainRules(ctx context.Context, workspaceID string) ([]DomainRule, error)
	DeleteDomainRule(ctx context.Context, workspaceID, domain string) error
	// InteractionByExternalID returns ErrNotFound when no conversation on the
	// channel has the id.
	InteractionByExternalID(ctx context.Context, workspaceID, channel, externalID string) (string, error)
	// UpsertHandle creates a handle with its first verdict, or returns the
	// existing one, filling in a missing name.
	UpsertHandle(ctx context.Context, h NewHandle) (Handle, error)
	SkipPendingHandle(ctx context.Context, id, reason string) error
	SetTriage(ctx context.Context, v Verdict) error
	RestoreContactRecords(ctx context.Context, workspaceID, address, domain string) error
	Handles(ctx context.Context, workspaceID string, ids []string) ([]Handle, error)
	HandlesByValue(ctx context.Context, workspaceID string, values []string) ([]Handle, error)
	HandlesByDomain(ctx context.Context, workspaceID, domain string) ([]Handle, error)
	ListHandles(ctx context.Context, q HandleQuery) ([]HandleSummary, error)
	// SetPhotos records profile pictures by email address.
	SetPhotos(ctx context.Context, workspaceID string, photos map[string]string) error
	// PersonPhotos maps people to a profile picture of one of their addresses.
	PersonPhotos(ctx context.Context, workspaceID string, personIDs []string) (map[string]string, error)
	// MarkInternal files undecided handles at the given addresses or domains as
	// internal.
	MarkInternal(ctx context.Context, workspaceID string, addresses, domains []string) error
	EngagedHandles(ctx context.Context, workspaceID string, maxSize int32, email, meetings bool) ([]string, error)
	HandlesOnRecords(ctx context.Context, workspaceID string) ([]HandleRecord, error)
	KeptWithoutPerson(ctx context.Context, workspaceID string) ([]Handle, error)
	UnassessedHandles(ctx context.Context, workspaceID string, limit int32) ([]UnassessedHandle, error)

	// EmailThreadByMessageIDs returns ErrNotFound when no message is known.
	EmailThreadByMessageIDs(ctx context.Context, workspaceID string, messageIDs []string) (string, error)
	UpsertEmailThread(ctx context.Context, t EmailThread) (string, error)
	ExtendEmailThread(ctx context.Context, id string, at time.Time, title string) error
	UpsertInteraction(ctx context.Context, i NewInteraction) (Interaction, error)
	ClearParticipants(ctx context.Context, interactionID string) error
	AddParticipant(ctx context.Context, interactionID, handleID, role string) error
	UpsertPart(ctx context.Context, p NewPart) error
	ClearParts(ctx context.Context, interactionID, kind string) error
	// SpanMessages stretches a conversation over its messages.
	SpanMessages(ctx context.Context, interactionID string) error
	// Relink rebuilds the interactions' sync links from their kept
	// participants and forgets provider content no record links.
	Relink(ctx context.Context, interactionIDs []string) error
	InteractionsOfHandles(ctx context.Context, handleIDs []string) ([]string, error)
	AddLink(ctx context.Context, interactionID, recordID, source string) error
	DeleteLink(ctx context.Context, interactionID, recordID string) error
	// SkipInteraction hides an interaction, drops its links and content.
	SkipInteraction(ctx context.Context, workspaceID, id string) error
	UnfetchedParts(ctx context.Context, connectionID string, limit int32) ([]UnfetchedPart, error)
	SetPartContent(ctx context.Context, id int64, content, html string) error
	DueMeetings(ctx context.Context, connectionID string) ([]Interaction, error)
	MarkTranscriptChecked(ctx context.Context, interactionID string) error

	Interactions(ctx context.Context, workspaceID string, ids []string) ([]Interaction, error)
	Timeline(ctx context.Context, q TimelineQuery) ([]Interaction, error)
	SearchInteractions(ctx context.Context, workspaceID, query string, limit int32) ([]Interaction, error)
	// Participants, Parts and Links take ids of already-scoped interactions.
	Participants(ctx context.Context, interactionIDs []string) ([]Participant, error)
	Parts(ctx context.Context, interactionIDs []string) ([]Part, error)
	Links(ctx context.Context, interactionIDs []string) ([]Link, error)
	RecordActivity(ctx context.Context, workspaceID string, recordIDs []string) ([]Activity, error)
	RecordLastMessages(ctx context.Context, workspaceID string, recordIDs []string) ([]LastMessage, error)
	FollowUpCandidates(ctx context.Context, workspaceID string, since time.Time, limit int32) ([]FollowUpCandidate, error)
	// ClaimFollowUp moves what the agent has read from previous to at,
	// returning the attempt's start time, or nil when another worker owns it.
	ClaimFollowUp(ctx context.Context, id string, previous, at *time.Time) (*time.Time, error)
	FinishFollowUp(ctx context.Context, id string, started time.Time, at *time.Time, state, reason string) (bool, error)
}
