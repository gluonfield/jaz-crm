package worker

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/internal/google"
	"github.com/gluonfield/jaz-crm/internal/status"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/temporal"
	"go.uber.org/fx"
)

type AIAvailability interface {
	Availability() status.Observation
}

type StatusParams struct {
	fx.In
	Status *status.Service
	Logger *log.Logger
	AI     AIAvailability `optional:"true"`
}

type StatusObserver struct {
	interceptor.WorkerInterceptorBase
	Status *status.Service
	Logger *log.Logger
	AI     AIAvailability
}

func NewStatusObserver(p StatusParams) *StatusObserver {
	return &StatusObserver{Status: p.Status, Logger: p.Logger, AI: p.AI}
}

func (o *StatusObserver) InterceptActivity(_ context.Context, next interceptor.ActivityInboundInterceptor) interceptor.ActivityInboundInterceptor {
	return &statusActivity{ActivityInboundInterceptorBase: interceptor.ActivityInboundInterceptorBase{Next: next}, observer: o}
}

type statusActivity struct {
	interceptor.ActivityInboundInterceptorBase
	observer *StatusObserver
}

func (a *statusActivity) ExecuteActivity(ctx context.Context, input *interceptor.ExecuteActivityInput) (any, error) {
	result, err := a.Next.ExecuteActivity(ctx, input)
	a.observer.record(ctx, activity.GetInfo(ctx).ActivityType.Name, input.Args, err)
	return result, err
}

func (o *StatusObserver) record(ctx context.Context, operation string, args []any, activityErr error) {
	if ctx.Err() == context.Canceled || errors.Is(activityErr, context.Canceled) || temporal.IsCanceledError(activityErr) {
		return
	}
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	report := func(operation string, connectionID *string, observation status.Observation) {
		if err := o.Status.Observe(ctx, operation, connectionID, observation); err != nil {
			o.Logger.Warn("operation status not recorded", "operation", operation, "error", err)
		}
	}
	if o.AI != nil {
		report(status.AI, nil, o.AI.Availability())
	}
	if !status.Operation(operation) || operation == status.AI || len(args) == 0 {
		return
	}
	var connectionID string
	if operation == "FetchTranscript" {
		meeting, ok := args[0].(MeetingRef)
		if !ok {
			return
		}
		connectionID = meeting.ConnectionID
	} else {
		connectionID, _ = args[0].(string)
	}
	observation := status.Observation{ObservedAt: now}
	if activityErr != nil {
		observation.OccurredAt = &now
		observation.Reason = "Sync could not finish."
		var apiError *google.APIError
		switch {
		case ending(activityErr) == errRevoked || errors.Is(activityErr, google.ErrRevoked):
			observation.Reason = "Reconnect your Google account in Connections to resume sync."
		case errors.As(activityErr, &apiError) && apiError.Status == http.StatusForbidden:
			observation.Reason = "Google denied access. Check the account's permissions in Connections."
		}
	}
	report(operation, &connectionID, observation)
}
