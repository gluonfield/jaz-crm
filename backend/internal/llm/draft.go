package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func (c *Client) draft(ctx context.Context, conv followups.Conversation, reads followups.ReadTools, out *followups.Plan) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	data, err := json.Marshal(conv)
	if err != nil {
		return err
	}
	request := responses.ResponseNewParams{
		Model:        c.model,
		Instructions: openai.String(followUpInstructions),
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: responses.ResponseInputParam{
			responses.ResponseInputItemParamOfMessage(string(data), "user"),
		}},
		Reasoning:      shared.ReasoningParam{Effort: c.effort},
		Store:          openai.Bool(false),
		Include:        []responses.ResponseIncludable{"reasoning.encrypted_content"},
		Truncation:     "disabled",
		PromptCacheKey: openai.String("crm-draft:" + reads.WorkspaceID),
		Text: responses.ResponseTextConfigParam{Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{Name: "follow_ups", Schema: plan, Strict: openai.Bool(true)},
		}},
	}
	for _, tool := range reads.Tools {
		function := responses.FunctionToolParam{
			Name: tool.Name, Description: openai.String(tool.Description),
			Strict: openai.Bool(true), AllowedCallers: []string{"direct"},
		}
		// The SDK's map-only field cannot hold the typed schema unchanged.
		function.SetExtraFields(map[string]any{"parameters": tool.Parameters})
		request.Tools = append(request.Tools, responses.ToolUnionParam{OfFunction: &function})
	}
	called := map[string]bool{}
	for turn := range 12 {
		res, err := c.api.Responses.New(ctx, request)
		if err != nil {
			return fmt.Errorf("draft response: %w", err)
		}
		c.logger.Info("draft response", "workspace", reads.WorkspaceID, "response", res.ID, "turn", turn+1, "usage", res.Usage.RawJSON())
		if res.Status != "completed" {
			return fmt.Errorf("draft response %s: status %s", res.ID, res.Status)
		}
		calls := []responses.ResponseFunctionToolCall{}
		for _, item := range res.Output {
			switch item.Type {
			case "function_call":
				call := item.AsFunctionCall()
				if call.CallID == "" || called[call.CallID] {
					return errors.New("draft response has missing or duplicate tool call ID")
				}
				called[call.CallID] = true
				calls = append(calls, call)
			case "message", "reasoning":
			default:
				return fmt.Errorf("unsupported draft output type %q", item.Type)
			}
			// Preserve provider-owned fields, especially encrypted reasoning. Appending
			// to the unchanged input also preserves prefixes for automatic KV caching.
			request.Input.OfInputItemList = append(request.Input.OfInputItemList, param.Override[responses.ResponseInputItemUnionParam](json.RawMessage(item.RawJSON())))
		}
		if len(calls) == 0 {
			var candidate followups.Plan
			if err := json.Unmarshal([]byte(res.OutputText()), &candidate); err != nil {
				return fmt.Errorf("draft answer is not the requested JSON: %w", err)
			}
			if candidate.HasOutcome() {
				*out = candidate
				return nil
			}
			request.Input.OfInputItemList = append(request.Input.OfInputItemList, responses.ResponseInputItemParamOfMessage("The final plan has neither a reply nor a skip_reason. Finish the task: include the ready-to-send reply in a follow_ups entry when the incoming message needs an answer and the facts are available; otherwise give a specific, nonempty skip_reason. Return the complete corrected plan.", "developer"))
		}
		if turn == 11 || len(called) > 48 {
			return errors.New("draft retrieval limit reached without a final answer")
		}
		for _, call := range calls {
			result, err := reads.Call(ctx, call.Name, call.Arguments)
			if err != nil {
				return fmt.Errorf("draft tool %s: %w", call.Name, err)
			}
			item := responses.ResponseInputItemParamOfFunctionCallOutput(result)
			item.OfFunctionCallOutput.CallID = openai.String(call.CallID)
			request.Input.OfInputItemList = append(request.Input.OfInputItemList, item)
		}
	}
	panic("unreachable")
}
