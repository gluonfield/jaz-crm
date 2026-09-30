// Package classifier judges addresses triage could not settle, with any
// OpenAI-compatible chat completions endpoint.
package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
)

// Config names the endpoint and model; the model has no default.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

type Classifier struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Classifier {
	return &Classifier{cfg: cfg, http: http.DefaultClient}
}

const instructions = `You decide which email correspondents belong in a CRM.
The CRM is for: %s
Keep people and companies the workspace has, or could plausibly have, a working relationship with.
Skip newsletters, notifications, marketing, recruiters and unanswered cold pitches, unless the CRM's purpose wants them.
Answer ask when the evidence does not settle it.
Reply with JSON only: {"judgements":[{"address":"...","verdict":"keep|skip|ask","reason":"a few words"}]}`

const defaultCriteria = "business relationships: customers, prospects, suppliers, partners and investors."

func (c *Classifier) Classify(ctx context.Context, criteria string, candidates []interactions.Candidate) ([]interactions.Judgement, error) {
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
	people, _ := json.Marshal(list)
	body, _ := json.Marshal(map[string]any{
		"model": c.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": fmt.Sprintf(instructions, criteria)},
			{"role": "user", "content": string(people)},
		},
		"response_format": map[string]string{"type": "json_object"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("classifier: %s: %.300s", res.Status, raw)
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &completion); err != nil || len(completion.Choices) == 0 {
		return nil, fmt.Errorf("classifier: unexpected response: %.300s", raw)
	}
	var out struct {
		Judgements []interactions.Judgement `json:"judgements"`
	}
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("classifier: reply is not the requested JSON: %w", err)
	}
	return out.Judgements, nil
}
