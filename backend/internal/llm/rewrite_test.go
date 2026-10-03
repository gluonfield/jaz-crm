package llm_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/llm"
)

func TestRewriteSendsFullContextOnceWithoutTools(t *testing.T) {
	input := followups.RewriteInput{
		Draft: "Could you send the drawing?", Subject: "Drawing", Action: "polish", To: []string{"jane@example.com"}, Cc: []string{"colleague@example.com"},
		Context: followups.Conversation{Channel: "email", WebAccess: true,
			Messages: []followups.Line{{Text: strings.Repeat("Full current conversation. ", 3000)}},
			History:  []interactions.Interaction{{Text: strings.Repeat("Full earlier conversation. ", 3000)}},
			Company:  []followups.Record{{Ref: interactions.Ref{ID: "knowledge", Name: "Company"}, Values: map[string][]string{"content": {strings.Repeat("Full knowledge page. ", 3000)}}}},
		},
	}
	wantInput, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Model, Input, Instructions, Truncation string
			Store                                  *bool
			Tools                                  []json.RawMessage
			Reasoning                              struct{ Effort string }
			Text                                   struct {
				Format struct {
					Type, Name string
					Strict     bool
					Schema     struct {
						Properties           map[string]struct{ Type string }
						Required             []string
						AdditionalProperties *bool `json:"additionalProperties"`
					}
				}
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if request.Model != "configured-model" || request.Reasoning.Effort != "medium" || request.Instructions == "" || request.Store == nil || *request.Store || request.Truncation != "disabled" || len(request.Tools) != 0 {
			t.Errorf("rewrite request changed model, retention, truncation or tools: %+v", request)
		}
		if request.Input != string(wantInput) {
			t.Errorf("rewrite context was altered: received %d bytes, want %d", len(request.Input), len(wantInput))
		}
		format := request.Text.Format
		if format.Type != "json_schema" || format.Name != "rewrite" || !format.Strict || format.Schema.AdditionalProperties == nil || *format.Schema.AdditionalProperties || !slices.Equal(format.Schema.Required, []string{"draft", "subject"}) || len(format.Schema.Properties) != 2 || format.Schema.Properties["draft"].Type != "string" || format.Schema.Properties["subject"].Type != "string" {
			t.Errorf("rewrite does not require exactly a body and subject: %+v", format)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"completed","output":[{"type":"reasoning","summary":[]},{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"{\"draft\":\"Please send the drawing.\",\"subject\":\"Unrequested new subject\"}"}]}]}`)
	}))
	t.Cleanup(api.Close)
	client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test-key", Model: "configured-model", Effort: "medium"}, log.New(io.Discard))
	result, err := client.Rewrite(t.Context(), input)
	if err != nil || calls.Load() != 1 || result.Draft != "Please send the drawing." || result.Subject != input.Subject {
		t.Fatalf("rewrite = %+v, calls = %d, error = %v", result, calls.Load(), err)
	}
}

func TestRewriteRejectsUnfinishedAnswersWithoutRetry(t *testing.T) {
	for _, test := range []struct {
		name, status, messageStatus, contentType, answer string
		httpStatus                                       int
	}{
		{name: "provider failure", httpStatus: http.StatusServiceUnavailable},
		{name: "incomplete response", status: "incomplete", answer: `{"draft":"Partial","subject":"Subject"}`},
		{name: "incomplete message", messageStatus: "incomplete", answer: `{"draft":"Partial","subject":"Subject"}`},
		{name: "refusal", contentType: "refusal", answer: `{"draft":"Do not accept adjacent text","subject":"Subject"}`},
		{name: "invalid JSON", answer: "Not JSON"},
		{name: "blank body", answer: `{"draft":" \n ","subject":"Subject"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if test.httpStatus != 0 {
					w.WriteHeader(test.httpStatus)
					fmt.Fprint(w, `{"error":{"message":"Unavailable","type":"server_error"}}`)
					return
				}
				status, messageStatus := test.status, test.messageStatus
				if status == "" {
					status = "completed"
				}
				if messageStatus == "" {
					messageStatus = "completed"
				}
				content := []any{map[string]any{"type": "output_text", "text": test.answer}}
				if test.contentType == "refusal" {
					content = append(content, map[string]any{"type": "refusal", "refusal": "Cannot comply"})
				}
				json.NewEncoder(w).Encode(map[string]any{"status": status, "output": []any{map[string]any{"type": "message", "role": "assistant", "status": messageStatus, "content": content}}})
			}))
			t.Cleanup(api.Close)
			client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test", Model: "configured-model"}, log.New(io.Discard))
			result, err := client.Rewrite(t.Context(), followups.RewriteInput{Draft: "Original", Subject: "Subject", Action: "polish"})
			if err == nil || result != (followups.RewriteResult{}) || calls.Load() != 1 {
				t.Fatalf("accepted partial answer or retried: result=%+v calls=%d error=%v", result, calls.Load(), err)
			}
		})
	}
}

func TestRewriteCustomSubjectRespectsChannel(t *testing.T) {
	api := httptest.NewServer(respond(t, "rewrite", followups.RewriteResult{Draft: "Edited message", Subject: "Requested subject"}))
	t.Cleanup(api.Close)
	client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
	for _, channel := range []string{"Email", "LinkedIn"} {
		input := followups.RewriteInput{Draft: "Original message", Action: "custom", Instruction: "Use Requested subject as the subject", Context: followups.Conversation{Channel: channel}}
		result, err := client.Rewrite(t.Context(), input)
		want := ""
		if channel == "Email" {
			want = "Requested subject"
		}
		if err != nil || result.Subject != want || result.Draft != "Edited message" {
			t.Fatalf("%s rewrite = %+v, error = %v", channel, result, err)
		}
	}
}
