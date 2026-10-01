// Package interactions owns a workspace's conversations: email threads,
// meetings, calls and notes, the addresses in them, and triage, which decides
// whose conversations the CRM keeps.
package interactions

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"go.uber.org/fx"
)

// Interaction kinds.
const (
	Email   = "email"
	Message = "message"
	Meeting = "meeting"
	Call    = "call"
	Note    = "note"
)

// Triage verdicts.
const (
	Pending  = "pending"
	Kept     = "kept"
	Skipped  = "skipped"
	Internal = "internal"
)

// Who decided a verdict, strongest last.
const (
	ByRule       = "rule"
	ByAgent      = "agent"
	ByEngagement = "engagement"
	ByUser       = "user"
)

// Classifier judges addresses no evidence settles, against a workspace's
// description of who belongs in its CRM.
type Classifier interface {
	Classify(ctx context.Context, criteria string, candidates []Candidate) ([]Judgement, error)
}

type Candidate struct {
	Address string
	Name    string
	Titles  []string
}

// Judgement is keep, skip or ask, with a reason.
type Judgement struct {
	Address string `json:"address"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
}

type Service struct {
	store      storage.InteractionStore
	conns      storage.ConnectionStore
	workspaces storage.WorkspaceStore
	records    *records.Service
	classifier Classifier
}

type Params struct {
	fx.In
	Store       storage.InteractionStore
	Connections storage.ConnectionStore
	Workspaces  storage.WorkspaceStore
	Records     *records.Service
	Classifier  Classifier `optional:"true"`
}

func NewService(p Params) *Service {
	return &Service{store: p.Store, conns: p.Connections, workspaces: p.Workspaces, records: p.Records, classifier: p.Classifier}
}
