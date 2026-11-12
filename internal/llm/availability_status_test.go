package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/internal/followups"
	"github.com/gluonfield/jaz-crm/internal/status"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
)

func TestAvailabilityRequiresObservedRecoveryAcrossStores(t *testing.T) {
	ctx := t.Context()
	dsn := postgrestest.Database(t)
	writer := status.NewService(postgrestest.Open(t, dsn))
	reader := status.NewService(postgrestest.Open(t, dsn))
	var mode atomic.Value
	mode.Store("invalid")
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch mode.Load().(string) {
		case "invalid":
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"private diagnostics","type":"invalid_request_error","code":"invalid_prompt"}}`)
		case "quota":
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"private diagnostics","type":"insufficient_quota","code":"credit_balance_exhausted"}}`)
		case "failed":
			fmt.Fprint(w, `{"id":"resp_failed","status":"failed","error":{"code":"server_error","message":"private diagnostics"}}`)
		case "ok":
			fmt.Fprint(w, `{"id":"resp_ok","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"follow_ups\":[],\"contexts\":[],\"skip_reason\":\"No reply needed.\"}"}]}]}`)
		}
	}))
	t.Cleanup(api.Close)
	client := New(Config{BaseURL: api.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
	client.now = func() time.Time { return time.Unix(clock.Load(), 0) }
	old := client.now().Add(-time.Minute)
	if err := writer.Observe(ctx, status.AI, nil, status.Observation{ObservedAt: old, OccurredAt: &old, Reason: "Earlier worker reported a quota issue."}); err != nil {
		t.Fatal(err)
	}
	_, _ = client.Plan(ctx, followups.Conversation{}, followups.ReadTools{})
	if !client.Availability().ObservedAt.IsZero() {
		t.Fatal("invalid provider response was reported as recovery")
	}
	if err := writer.Observe(ctx, status.AI, nil, client.Availability()); err != nil {
		t.Fatal(err)
	}
	if rows, err := reader.Errors(ctx, "", ""); err != nil || len(rows) != 1 {
		t.Fatalf("fresh unknown client cleared the earlier worker's issue: %+v (%v)", rows, err)
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	unreachable := New(Config{BaseURL: closed.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, _ = unreachable.Plan(short, followups.Conversation{}, followups.ReadTools{})
	if !unreachable.Availability().ObservedAt.IsZero() {
		t.Fatal("network failure was reported as recovery")
	}
	mode.Store("quota")
	_, _ = client.Plan(ctx, followups.Conversation{}, followups.ReadTools{})
	paused := client.Availability()
	if paused.OccurredAt == nil || paused.RetryAt == nil || paused.Reason == "" {
		t.Fatalf("quota was not observed: %+v", paused)
	}
	if err := writer.Observe(ctx, status.AI, nil, paused); err != nil {
		t.Fatal(err)
	}
	clock.Add(int64(time.Hour.Seconds()))
	mode.Store("failed")
	_, _ = client.Plan(ctx, followups.Conversation{}, followups.ReadTools{})
	failed := client.Availability()
	if failed.Reason == "" || failed.OccurredAt == nil || !failed.OccurredAt.Equal(*paused.OccurredAt) || !failed.ObservedAt.After(paused.ObservedAt) {
		t.Fatalf("failed recovery changed the last real quota occurrence or claimed success: %+v", failed)
	}
	if err := writer.Observe(ctx, status.AI, nil, failed); err != nil {
		t.Fatal(err)
	}
	if rows, err := reader.Errors(ctx, "", ""); err != nil || len(rows) != 1 || !rows[0].OccurredAt.Equal(*paused.OccurredAt) {
		t.Fatalf("failed check cleared or rewrote the reported issue: %+v (%v)", rows, err)
	}
	clock.Add(int64(time.Hour.Seconds()))
	mode.Store("ok")
	if _, err := client.Plan(ctx, followups.Conversation{}, followups.ReadTools{}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Observe(ctx, status.AI, nil, client.Availability()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Observe(ctx, status.AI, nil, paused); err != nil {
		t.Fatal(err)
	}
	if rows, err := reader.Errors(ctx, "", ""); err != nil || len(rows) != 0 {
		t.Fatalf("actual recovery did not clear the issue, or stale failure reappeared: %+v (%v)", rows, err)
	}
}

func TestInFlightSuccessDoesNotClearQuotaObservation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(string(data), "slow request") {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"No credits","type":"insufficient_quota","code":"credit_balance_exhausted"}}`)
			return
		}
		close(started)
		<-release
		fmt.Fprint(w, `{"id":"resp_ok","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"follow_ups\":[],\"contexts\":[],\"skip_reason\":\"No reply needed.\"}"}]}]}`)
	}))
	t.Cleanup(api.Close)
	client := New(Config{BaseURL: api.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
	finished := make(chan error, 1)
	go func() {
		_, err := client.Plan(t.Context(), followups.Conversation{Title: "slow request"}, followups.ReadTools{})
		finished <- err
	}()
	<-started
	_, _ = client.Plan(t.Context(), followups.Conversation{}, followups.ReadTools{})
	paused := client.Availability()
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if after := client.Availability(); after.Reason == "" || !after.ObservedAt.Equal(paused.ObservedAt) {
		t.Fatalf("late success cleared or moved the quota observation: %+v", after)
	}
}
