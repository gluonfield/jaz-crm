package worker

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/fx"
)

func NewWorker(c client.Client, a *Activities) worker.Worker {
	w := worker.New(c, TaskQueue, worker.Options{})
	w.RegisterWorkflow(ConnectionSync)
	w.RegisterWorkflow(MeetingTranscript)
	w.RegisterActivity(a)
	return w
}

// Run starts the worker and wakes every active connection's sync, so a sync
// lost to an outage resumes.
func Run(lc fx.Lifecycle, w worker.Worker, starter *Starter, conns *connections.Service, logger *log.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := w.Start(); err != nil {
				return err
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				active, err := conns.Active(ctx)
				if err != nil {
					logger.Error("resume syncs", "error", err)
				}
				for _, c := range active {
					if err := starter.Start(ctx, c.ID); err != nil {
						logger.Error("resume sync", "connection", c.ID, "error", err)
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			w.Stop()
			return nil
		},
	})
}
