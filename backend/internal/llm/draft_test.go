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
			tools := followups.ReadTools{WorkspaceID: "test", Tools: []followups.ReadTool{{Name: "get_record", Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}, "additionalProperties": false}, Run: func(ctx context.Context, _ json.RawMessage) (any, error) {
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
