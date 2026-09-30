package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// syncEnv mocks every sync activity; backfill reports done from its second
// call, and each pass records its workflow time.
func syncEnv(t *testing.T, backfill func(int) (bool, error)) (*testsuite.TestWorkflowEnvironment, *[]time.Time) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var a *Activities
	var passes []time.Time
	env.OnActivity(a.GmailBackfill, mock.Anything, "c1").Return(func(context.Context, string) (bool, error) {
		passes = append(passes, env.Now())
		return backfill(len(passes))
	})
	for _, step := range []any{a.Aliases, a.GmailIncremental, a.CalendarSync, a.Triage, a.Photos, a.Watch} {
		env.OnActivity(step, mock.Anything, "c1").Return(nil).Maybe()
	}
	env.OnActivity(a.FetchContent, mock.Anything, "c1").Return(0, nil).Maybe()
	env.OnActivity(a.DueMeetings, mock.Anything, "c1").Return([]MeetingRef{}, nil).Maybe()
	env.OnActivity(a.Revoke, mock.Anything, "c1").Return(nil).Maybe()
	return env, &passes
}

// A sync runs passes back to back while work remains, then every poll
// interval, sooner when woken, and continues as new to bound its history.
func TestConnectionSyncPaces(t *testing.T) {
	env, passes := syncEnv(t, func(n int) (bool, error) { return n >= 2, nil })
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(SignalWake, nil) }, 6*time.Minute)
	env.ExecuteWorkflow(ConnectionSync, "c1")
	if !workflow.IsContinueAsNewError(env.GetWorkflowError()) {
		t.Fatalf("error: %v", env.GetWorkflowError())
	}
	p := *passes
	if len(p) != passesPerRun || p[1] != p[0] || p[2].Sub(p[1]) != pollInterval || p[3].Sub(p[2]) != time.Minute {
		t.Fatalf("passes %d at %v", len(p), p[:4])
	}
}

// A revoked grant marks the connection and ends its sync.
func TestConnectionSyncStopsWhenRevoked(t *testing.T) {
	env, _ := syncEnv(t, func(int) (bool, error) {
		return false, temporal.NewNonRetryableApplicationError("revoked", errRevoked, nil)
	})
	env.ExecuteWorkflow(ConnectionSync, "c1")
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	env.AssertActivityNumberOfCalls(t, "Revoke", 1)
}

// A recent meeting's transcript is retried until Google publishes it; an
// old meeting is tried once. Either way the meeting is marked checked.
func TestMeetingTranscriptRetries(t *testing.T) {
	for name, tc := range map[string]struct {
		age   time.Duration
		calls int
	}{"recent": {time.Hour, 3}, "old": {48 * time.Hour, 1}} {
		var suite testsuite.WorkflowTestSuite
		env := suite.NewTestWorkflowEnvironment()
		var a *Activities
		calls := 0
		env.OnActivity(a.FetchTranscript, mock.Anything, mock.Anything).Return(func(context.Context, MeetingRef) (bool, error) {
			calls++
			return calls == 3, nil
		})
		env.OnActivity(a.TranscriptChecked, mock.Anything, "m1").Return(nil)
		env.ExecuteWorkflow(MeetingTranscript, MeetingRef{InteractionID: "m1", End: env.Now().Add(-tc.age)})
		if err := env.GetWorkflowError(); err != nil || calls != tc.calls {
			t.Fatalf("%s: %d fetches, %v", name, calls, err)
		}
		env.AssertActivityNumberOfCalls(t, "TranscriptChecked", 1)
	}
}
