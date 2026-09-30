package worker

import (
	"errors"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// SignalWake asks a sync to run a pass now, such as after a push notification.
	SignalWake   = "wake"
	pollInterval = 5 * time.Minute
	passesPerRun = 100

	// Activity error types that end a sync.
	errRevoked = "Revoked"
	errGone    = "Gone"
)

// ConnectionSync keeps one connection's mail and calendar synced: it runs
// passes while work remains, then waits for the poll interval or a wake.
func ConnectionSync(ctx workflow.Context, connectionID string) error {
	var a *Activities
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumInterval: 5 * time.Minute, MaximumAttempts: 5, NonRetryableErrorTypes: []string{errRevoked, errGone}},
	})
	wake := workflow.GetSignalChannel(ctx, SignalWake)
	for range passesPerRun {
		busy, err := pass(ctx, a, connectionID)
		switch ending(err) {
		case errGone:
			return nil
		case errRevoked:
			return workflow.ExecuteActivity(ctx, a.Revoke, connectionID).Get(ctx, nil)
		}
		if !busy {
			wait(ctx, wake)
		}
	}
	for wake.ReceiveAsync(nil) {
	}
	return workflow.NewContinueAsNewError(ctx, ConnectionSync, connectionID)
}

// pass runs every stream once and reports whether work remains. A failing
// stream is logged and skipped so the others keep moving; only a revoked or
// deleted connection stops the pass.
func pass(ctx workflow.Context, a *Activities, id string) (bool, error) {
	run := func(activity, out any) error {
		err := workflow.ExecuteActivity(ctx, activity, id).Get(ctx, out)
		if err != nil && ending(err) == "" {
			workflow.GetLogger(ctx).Warn("sync step failed", "connection", id, "error", err)
			return nil
		}
		return err
	}
	var backfilled bool
	var fetched int
	var due []MeetingRef
	steps := []struct {
		activity any
		out      any
	}{
		{a.GmailBackfill, &backfilled}, {a.GmailIncremental, nil}, {a.CalendarSync, nil}, {a.Triage, nil},
		{a.FetchContent, &fetched}, {a.Watch, nil}, {a.DueMeetings, &due},
	}
	for _, s := range steps {
		if err := run(s.activity, s.out); err != nil {
			return false, err
		}
	}
	for _, m := range due {
		child := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID:            "meeting-transcript-" + m.InteractionID,
			ParentClosePolicy:     enums.PARENT_CLOSE_POLICY_ABANDON,
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		})
		_ = workflow.ExecuteChildWorkflow(child, MeetingTranscript, m).GetChildWorkflowExecution().Get(ctx, nil)
	}
	return !backfilled || fetched == contentBatch, nil
}

// ending names the error type that ends a sync, if err is one.
func ending(err error) string {
	var app *temporal.ApplicationError
	if errors.As(err, &app) && (app.Type() == errRevoked || app.Type() == errGone) {
		return app.Type()
	}
	return ""
}

func wait(ctx workflow.Context, wake workflow.ReceiveChannel) {
	timerCtx, cancel := workflow.WithCancel(ctx)
	defer cancel()
	selector := workflow.NewSelector(ctx)
	selector.AddFuture(workflow.NewTimer(timerCtx, pollInterval), func(workflow.Future) {})
	selector.AddReceive(wake, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, nil)
	})
	selector.Select(ctx)
	for wake.ReceiveAsync(nil) {
	}
}

// MeetingTranscript fetches a Meet transcript, which Google publishes some
// time after the meeting: a recent meeting is retried for most of a day, an
// older one is tried once. Either way the meeting is then marked checked.
func MeetingTranscript(ctx workflow.Context, m MeetingRef) error {
	var a *Activities
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3, NonRetryableErrorTypes: []string{errRevoked, errGone}},
	})
	delays := []time.Duration{0, 15 * time.Minute, time.Hour, 4 * time.Hour, 12 * time.Hour}
	if workflow.Now(ctx).Sub(m.End) > 24*time.Hour {
		delays = delays[:1]
	}
	for _, delay := range delays {
		if err := workflow.Sleep(ctx, delay); err != nil {
			return err
		}
		var found bool
		err := workflow.ExecuteActivity(ctx, a.FetchTranscript, m).Get(ctx, &found)
		if found || ending(err) != "" {
			break
		}
	}
	return workflow.ExecuteActivity(ctx, a.TranscriptChecked, m.InteractionID).Get(ctx, nil)
}
