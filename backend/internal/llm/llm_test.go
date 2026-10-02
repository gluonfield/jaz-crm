package llm_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/llm"
)

// respond serves OpenAI's Responses API: it checks a request asks for the
// configured model and effort with a strict schema, and answers with text.
func respond(t *testing.T, schema string, answer any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model        string
			Instructions string
			Input        string
			Store        *bool
			Reasoning    struct{ Effort string }
			Text         struct {
				Format struct {
					Type, Name string
					Strict     bool
					Schema     struct {
						Properties           map[string]any
						Required             []string
						AdditionalProperties bool `json:"additionalProperties"`
					}
				}
			}
		}
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("request %s with %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		f := req.Text.Format
		if req.Model != "gpt-6-luna" || req.Reasoning.Effort != "medium" || req.Store == nil || *req.Store || req.Instructions == "" || !json.Valid([]byte(req.Input)) {
			t.Errorf("request model %q effort %q store %v input %q", req.Model, req.Reasoning.Effort, req.Store, req.Input)
		}
		if f.Type != "json_schema" || f.Name != schema || !f.Strict || f.Schema.AdditionalProperties || len(f.Schema.Required) != len(f.Schema.Properties) {
			t.Errorf("format %+v", f)
		}
		text, _ := json.Marshal(answer)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"resp_1","object":"response","created_at":0,"status":"completed","model":"gpt-6-luna","output":[
			{"type":"reasoning","id":"rs_1","summary":[]},
			{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":%q,"annotations":[]}]}]}`, text)
	}
}

func TestPlanAndClassify(t *testing.T) {
	want := followups.Plan{FollowUps: []followups.Change{{Action: "Send revised quote", WaitingOn: "Us", ReviewOn: "2026-10-02", Status: "Open", Person: "p1", Reply: "Hi Jane"}}, Contexts: []followups.Context{{Person: "p1", Context: "- Head of purchasing at Acme"}}}
	judged := []interactions.Judgement{{Address: "jane@acme.com", Verdict: "keep", Reason: "customer"}}
	routes := http.NewServeMux()
	srv := httptest.NewServer(routes)
	t.Cleanup(srv.Close)
	client := llm.New(llm.Config{BaseURL: srv.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"})

	routes.HandleFunc("/responses", respond(t, "follow_ups", want))
	got, err := client.Plan(t.Context(), followups.Conversation{Kind: "email", Messages: []followups.Line{{Author: "Jane", Text: "Quote please"}}})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("plan %+v, %v", got, err)
	}

	routes = http.NewServeMux()
	srv.Config.Handler = routes
	routes.HandleFunc("/responses", respond(t, "triage", map[string]any{"judgements": judged}))
	verdicts, err := client.Classify(t.Context(), "", []interactions.Candidate{{Address: "jane@acme.com", Titles: []string{"Quote"}}})
	if err != nil || !slices.Equal(verdicts, judged) {
		t.Fatalf("judgements %+v, %v", verdicts, err)
	}
}
