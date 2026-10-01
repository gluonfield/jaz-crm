// Package llm asks an OpenAI model for structured judgements: which
// correspondents belong in the CRM, and what a conversation means for its
// follow-ups.
package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Effort  string
}

type Client struct {
	model  string
	effort shared.ReasoningEffort
	api    openai.Client
}

func New(cfg Config) *Client {
	return &Client{
		model:  cfg.Model,
		effort: shared.ReasoningEffort(cfg.Effort),
		api:    openai.NewClient(option.WithAPIKey(cfg.APIKey), option.WithBaseURL(cfg.BaseURL)),
	}
}

// ask sends input as JSON under instructions and decodes the model's answer,
// which the strict schema shapes, into out.
func (c *Client) ask(ctx context.Context, name, instructions string, input any, schema map[string]any, out any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	res, err := c.api.Responses.New(ctx, responses.ResponseNewParams{
		Model:        c.model,
		Instructions: openai.String(instructions),
		Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(string(data))},
		Reasoning:    shared.ReasoningParam{Effort: c.effort},
		Store:        openai.Bool(false),
		Text: responses.ResponseTextConfigParam{Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{Name: name, Schema: schema, Strict: openai.Bool(true)},
		}},
	})
	if err != nil {
		return fmt.Errorf("llm %s: %w", name, err)
	}
	if err := json.Unmarshal([]byte(res.OutputText()), out); err != nil {
		return fmt.Errorf("llm %s: answer is not the requested JSON: %w", name, err)
	}
	return nil
}

// object is a strict JSON schema object: every property required and no
// others allowed.
func object(properties map[string]any) map[string]any {
	required := make([]string, 0, len(properties))
	for name := range properties {
		required = append(required, name)
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func list(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func text(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func choice(options ...string) map[string]any {
	return map[string]any{"type": "string", "enum": options}
}
