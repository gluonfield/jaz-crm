package status

import (
	"context"
	"time"

	"github.com/gluonfield/jaz-crm/internal/storage"
)

const AI = "ai"

var operations = map[string]string{
	AI:                 "AI background work",
	"Aliases":          "Mailbox aliases",
	"GmailBackfill":    "Mail history sync",
	"GmailIncremental": "Mail sync",
	"GmailDrafts":      "Gmail drafts",
	"CalendarSync":     "Calendar sync",
	"Triage":           "Contact triage",
	"CompanyLogos":     "Company logos",
	"FetchContent":     "Message content",
	"Photos":           "Contact photos",
	"Watch":            "Google notifications",
	"DueMeetings":      "Meeting transcripts",
	"FollowUps":        "AI follow-ups",
	"FetchTranscript":  "Meeting transcripts",
}

type Observation struct {
	ObservedAt time.Time
	Reason     string
	OccurredAt *time.Time
	RetryAt    *time.Time
}

type Error struct {
	ID         string     `json:"id"`
	Operation  string     `json:"operation"`
	Reason     string     `json:"reason"`
	OccurredAt time.Time  `json:"occurred_at"`
	RetryAt    *time.Time `json:"retry_at,omitempty"`
}

type Service struct {
	store storage.OperationStore
}

func NewService(store storage.OperationStore) *Service {
	return &Service{store: store}
}

func Operation(name string) bool {
	_, ok := operations[name]
	return ok
}

func (s *Service) Observe(ctx context.Context, operation string, connectionID *string, observation Observation) error {
	if observation.ObservedAt.IsZero() {
		return nil
	}
	return s.store.ObserveOperation(ctx, storage.OperationStatus{
		ConnectionID: connectionID, Operation: operation, Reason: observation.Reason,
		ObservedAt: observation.ObservedAt, OccurredAt: observation.OccurredAt, RetryAt: observation.RetryAt,
	})
}

func (s *Service) Errors(ctx context.Context, workspaceID, userID string) ([]Error, error) {
	rows, err := s.store.OperationErrors(ctx, workspaceID, userID)
	out := []Error{}
	for _, row := range rows {
		id := row.Operation
		if row.ConnectionID != nil {
			id += ":" + *row.ConnectionID
		}
		out = append(out, Error{ID: id, Operation: operations[row.Operation], Reason: row.Reason, OccurredAt: *row.OccurredAt, RetryAt: row.RetryAt})
	}
	return out, err
}
