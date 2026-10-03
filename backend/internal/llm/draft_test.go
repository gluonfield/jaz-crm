package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/llm"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestDraftLoopStopsWithoutPartialPlan(t *testing.T) {
	for _, test := range []struct {
		name      string
		status    string
		kind      string
		calls     int
		wantTurns int
		wantReads int
		wantError string
	}{
		{name: "incomplete", status: "incomplete", calls: 1, wantTurns: 1, wantError: "status incomplete"},
		{name: "refusal", status: "completed", kind: "refusal", wantTurns: 1, wantError: "not the requested JSON"},
		{name: "unsupported execution", status: "completed", kind: "program", wantTurns: 1, wantError: "unsupported"},
		{name: "unapproved web search", status: "completed", kind: "web", wantTurns: 1, wantError: "web access is disabled"},
		{name: "duplicate calls", status: "completed", kind: "duplicate", calls: 2, wantTurns: 1, wantError: "duplicate"},
		{name: "turn budget", status: "completed", calls: 1, wantTurns: 12, wantReads: 11, wantError: "retrieval limit"},
		{name: "tool budget", status: "completed", calls: 49, wantTurns: 1, wantError: "retrieval limit"},
		{name: "unfinished plans", status: "completed", wantTurns: 12, wantError: "retrieval limit"},
		{name: "storage failure", status: "completed", kind: "storage", calls: 1, wantTurns: 1, wantReads: 1, wantError: "database disconnected"},
		{name: "cancel during read", status: "completed", kind: "cancel", calls: 1, wantTurns: 1, wantReads: 1, wantError: "context canceled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			turns, reads := 0, 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				turns++
				output := []any{}
				for i := range test.calls {
					id := fmt.Sprintf("%d_%d", turns, i)
					if test.kind == "duplicate" {
						id = "duplicate"
					}
					output = append(output, map[string]any{"type": "function_call", "id": "fc_" + id, "call_id": id, "name": "get_record", "arguments": "{}", "status": "completed"})
				}
				switch test.kind {
				case "web":
					output = append(output, map[string]any{"type": "web_search_call", "id": "ws_unapproved", "status": "completed"})
				case "refusal":
					output = append(output, map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "refusal", "refusal": "Cannot answer"}}})
				case "program":
					output = append(output, map[string]any{"type": "program", "code": "must never execute"})
				default:
					output = append(output, map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": `{"follow_ups":[{"action":"Never save this partial plan"}],"contexts":[],"skip_reason":""}`}}})
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "resp_test", "status": test.status, "output": output})
			}))
			t.Cleanup(api.Close)
			client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
			tools := followups.ReadTools{WorkspaceID: "test", Tools: []followups.ReadTool{{Name: "get_record", Parameters: &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{}, AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}}, Run: func(ctx context.Context, _ json.RawMessage) (any, error) {
				reads++
				switch test.kind {
				case "storage":
					return nil, errors.New("database disconnected")
				case "cancel":
					cancel()
					return nil, ctx.Err()
				default:
					return map[string]string{"name": "More information"}, nil
				}
			}}}}
			plan, err := client.Plan(ctx, followups.Conversation{}, tools)
			if err == nil || !strings.Contains(err.Error(), test.wantError) || len(plan.FollowUps) != 0 || len(plan.Contexts) != 0 {
				t.Fatalf("partial plan or wrong failure: %+v (%v)", plan, err)
			}
			if turns != test.wantTurns || reads != test.wantReads {
				t.Fatalf("turns=%d reads=%d, want %d/%d", turns, reads, test.wantTurns, test.wantReads)
			}
		})
	}
}

func TestDraftLoopRepairsMissingOutcome(t *testing.T) {
	turns := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turns++
		var request struct {
			Input []struct {
				Role    string
				Content json.RawMessage
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		answer := `{"contexts":[{"context":"- Asked for ZX-17 public unit price and lead time.","person":"person-1"}],"follow_ups":[],"skip_reason":""}`
		if turns == 2 {
			if len(request.Input) != 3 || request.Input[1].Role != "assistant" || request.Input[2].Role != "developer" || !strings.Contains(string(request.Input[2].Content), "skip_reason") {
				t.Errorf("missing preserved answer and correction: %+v", request.Input)
			}
			answer = `{"contexts":[],"follow_ups":[{"action":"Reply with the catalogue price","id":"","person":"person-1","company":"","deal":"","reply":"The public unit price is GBP 42.","status":"Open","waiting_on":"Us","review_on":"2026-10-03"}],"skip_reason":""}`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "resp_final", "status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": answer}}}}})
	}))
	t.Cleanup(api.Close)
	client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
	plan, err := client.Plan(t.Context(), followups.Conversation{}, followups.ReadTools{})
	if err != nil || turns != 2 || len(plan.FollowUps) != 1 || !strings.Contains(plan.FollowUps[0].Reply, "42") {
		t.Fatalf("unfinished plan was not corrected: turns=%d plan=%+v err=%v", turns, plan, err)
	}
}

func TestDraftWebAccessAndReplay(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			turns := 0
			webItem := `{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"public capabilities","sources":[{"type":"url","url":"https://example.com"}]},"provider_field":"preserve"}`
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Tools        []struct{ Type string }
					Input        []json.RawMessage
					MaxToolCalls int `json:"max_tool_calls"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if enabled {
					if len(request.Tools) != 1 || request.Tools[0].Type != "web_search" || request.MaxToolCalls != 48-turns {
						t.Errorf("web capability or remaining budget missing: %+v", request.Tools)
					}
				} else if len(request.Tools) != 0 {
					t.Error("web tool exposed without workspace permission")
				}
				if turns == 1 && (len(request.Input) != 4 || string(request.Input[1]) != webItem) {
					t.Error("web result was lost or altered before continuation")
				}
				output := []any{}
				answer := `{"follow_ups":[],"contexts":[],"skip_reason":"No incoming question requires a reply."}`
				if enabled && turns == 0 {
					output = append(output, json.RawMessage(webItem))
					answer = `{"follow_ups":[],"contexts":[],"skip_reason":""}`
				}
				output = append(output, map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": answer}}})
				turns++
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "resp_web", "status": "completed", "output": output})
			}))
			t.Cleanup(api.Close)
			client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
			plan, err := client.Plan(t.Context(), followups.Conversation{WebAccess: enabled}, followups.ReadTools{})
			if err != nil || !plan.HasOutcome() || enabled && turns != 2 || !enabled && turns != 1 {
				t.Fatalf("web draft: turns=%d plan=%+v err=%v", turns, plan, err)
			}
		})
	}
}
