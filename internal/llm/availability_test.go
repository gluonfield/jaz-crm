package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/errs"
	"github.com/gluonfield/jaz-crm/internal/followups"
)

func TestQuotaPauseAndRecoveryAcrossJobs(t *testing.T) {
	var requests, probes atomic.Int32
	var available atomic.Bool
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	started := make(chan struct{})
	release := make(chan struct{})
	cancelledProbe := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var request struct {
			Input           json.RawMessage
			Model           string
			MaxOutputTokens int `json:"max_output_tokens"`
			Reasoning       struct{ Effort string }
		}
		if err := json.Unmarshal(data, &request); err != nil {
			t.Error(err)
		}
		if string(request.Input) == `"Reply OK."` {
			if len(data) > 512 || request.MaxOutputTokens != 16 || request.Model != "gpt-6-luna" || request.Reasoning.Effort != "medium" {
				t.Errorf("unbounded or misconfigured recovery probe: %s", data)
			}
			switch probes.Add(1) {
			case 1:
				close(started)
				<-release
			case 2:
				close(cancelledProbe)
				<-r.Context().Done()
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if !available.Load() {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"No credits remaining","type":"insufficient_quota","code":"credit_balance_exhausted"}}`)
			return
		}
		fmt.Fprint(w, `{"id":"resp_ok","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"follow_ups\":[],\"contexts\":[],\"skip_reason\":\"No reply needed.\"}"}]}]}`)
	}))
	t.Cleanup(api.Close)
	var logs bytes.Buffer
	client := New(Config{BaseURL: api.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, log.New(&logs))
	client.now = func() time.Time { return time.Unix(clock.Load(), 0) }
	conversation := followups.Conversation{Messages: []followups.Line{{Text: strings.Repeat("synthetic private conversation ", 30_000)}}}
	_, err := client.Plan(t.Context(), conversation, followups.ReadTools{})
	if !errors.As(err, new(*errs.Deferred)) || requests.Load() != 1 {
		t.Fatalf("billing failure retried or not paused: requests=%d err=%v", requests.Load(), err)
	}
	_, triageErr := client.Classify(t.Context(), "relationships", nil)
	_, contextErr := client.Summarize(t.Context(), followups.History{})
	_, rewriteErr := client.Rewrite(t.Context(), followups.RewriteInput{})
	for _, err := range []error{triageErr, contextErr, rewriteErr} {
		if !errors.As(err, new(*errs.Deferred)) {
			t.Fatalf("job did not share provider pause: %v", err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("paused jobs uploaded requests: %d", requests.Load())
	}
	clock.Add(int64(time.Hour.Seconds()))
	finished := make(chan error, 1)
	go func() {
		_, err := client.Plan(t.Context(), conversation, followups.ReadTools{})
		finished <- err
	}()
	<-started
	var jobs sync.WaitGroup
	for range 20 {
		jobs.Go(func() {
			_, err := client.Classify(t.Context(), "relationships", nil)
			if !errors.As(err, new(*errs.Deferred)) {
				t.Errorf("parallel job did not stay paused: %v", err)
			}
		})
	}
	jobs.Wait()
	if requests.Load() != 2 || probes.Load() != 1 {
		t.Errorf("parallel recovery fanout: requests=%d probes=%d", requests.Load(), probes.Load())
	}
	close(release)
	if err := <-finished; !errors.As(err, new(*errs.Deferred)) {
		t.Fatalf("failed probe resumed full payload: %v", err)
	}
	clock.Add(int64(time.Hour.Seconds()))
	probeCtx, cancel := context.WithCancel(t.Context())
	go func() {
		_, err := client.Plan(probeCtx, conversation, followups.ReadTools{})
		finished <- err
	}()
	<-cancelledProbe
	cancel()
	if err := <-finished; !errors.As(err, new(*errs.Deferred)) {
		t.Fatalf("cancelled probe did not leave work deferred: %v", err)
	}
	if _, err := client.Classify(t.Context(), "relationships", nil); !errors.As(err, new(*errs.Deferred)) || requests.Load() != 3 {
		t.Fatalf("cancelled probe immediately retried: requests=%d err=%v", requests.Load(), err)
	}
	clock.Add(int64(time.Hour.Seconds()))
	available.Store(true)
	plan, err := client.Plan(t.Context(), conversation, followups.ReadTools{})
	if err != nil || !plan.HasOutcome() || requests.Load() != 5 || probes.Load() != 3 {
		t.Fatalf("recovery did not resume the original job: requests=%d probes=%d plan=%+v err=%v", requests.Load(), probes.Load(), plan, err)
	}
	if strings.Count(logs.String(), "model requests paused") != 1 || strings.Count(logs.String(), "model requests resumed") != 1 {
		t.Fatalf("provider transitions logged repeatedly: %s", logs.String())
	}
}

func TestBillingCodesDoNotRetry(t *testing.T) {
	for _, test := range []struct{ code, kind string }{
		{"insufficient_quota", "invalid_request_error"},
		{"credit_balance_exhausted", "billing_error"},
		{"organization_spend_limit_exceeded", "billing_error"},
		{"project_spend_limit_exceeded", "billing_error"},
		{"organization_usage_limit_exceeded", "billing_error"},
		{"", "insufficient_quota"},
	} {
		t.Run(test.code+test.kind, func(t *testing.T) {
			var calls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":{"message":"billing blocked","type":%q,"code":%q}}`, test.kind, test.code)
			}))
			t.Cleanup(api.Close)
			client := New(Config{BaseURL: api.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
			_, err := client.Classify(t.Context(), "relationships", nil)
			if !errors.As(err, new(*errs.Deferred)) || calls.Load() != 1 {
				t.Fatalf("billing error retried: calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}

func TestTemporaryRateLimitStillRetries(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) < 3 {
			w.Header().Set("retry-after-ms", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"Slow down","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)
			return
		}
		fmt.Fprint(w, `{"id":"resp_ok","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"judgements\":[]}"}]}]}`)
	}))
	t.Cleanup(api.Close)
	client := New(Config{BaseURL: api.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
	if _, err := client.Classify(t.Context(), "relationships", nil); err != nil || calls.Load() != 3 {
		t.Fatalf("temporary limit stopped retrying: calls=%d err=%v", calls.Load(), err)
	}
	if _, err := client.Classify(t.Context(), "relationships", nil); err != nil || calls.Load() != 4 {
		t.Fatalf("temporary limit paused provider: calls=%d err=%v", calls.Load(), err)
	}
}
