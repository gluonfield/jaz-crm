package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
)

const triageInstructions = `You decide which email correspondents belong in a CRM.
The CRM is for: %s
Keep people and companies the workspace has, or could plausibly have, a working relationship with.
Skip newsletters, notifications, marketing, recruiters and unanswered cold pitches, unless the CRM's purpose wants them.
Answer ask when the evidence does not settle it. Give each address one judgement with a reason of a few words.`

const defaultCriteria = "business relationships: customers, prospects, suppliers, partners and investors."

var judgements = object(map[string]any{"judgements": list(object(map[string]any{
	"address": text("the candidate's address"),
	"verdict": choice("keep", "skip", "ask"),
	"reason":  text("a few words"),
}))})

// Classify judges addresses triage could not settle against the workspace's
// criteria.
func (c *Client) Classify(ctx context.Context, criteria string, candidates []interactions.Candidate) ([]interactions.Judgement, error) {
	if strings.TrimSpace(criteria) == "" {
		criteria = defaultCriteria
	}
	type candidate struct {
		Address  string   `json:"address"`
		Name     string   `json:"name,omitempty"`
		Subjects []string `json:"recent_subjects"`
	}
	list := make([]candidate, len(candidates))
	for i, cand := range candidates {
		list[i] = candidate{Address: cand.Address, Name: cand.Name, Subjects: cand.Titles}
	}
	var out struct {
		Judgements []interactions.Judgement `json:"judgements"`
	}
	err := c.ask(ctx, "triage", fmt.Sprintf(triageInstructions, criteria), list, judgements, &out)
	return out.Judgements, err
}
