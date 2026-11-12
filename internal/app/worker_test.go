package app

import (
	"testing"

	"github.com/gluonfield/jaz-crm/internal/llm"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/worker"
	tworker "go.temporal.io/sdk/worker"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type offlineWorker struct {
	tworker.Worker
	started bool
	stopped bool
}

func (w *offlineWorker) Start() error {
	w.started = true
	return nil
}

func (w *offlineWorker) Stop() {
	w.stopped = true
}

func TestWorkerStartsWithOptionalAI(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "without AI"
		if configured {
			name = "with AI"
		}
		t.Run(name, func(t *testing.T) {
			cfg := Config{
				DatabaseURL: postgrestest.Database(t),
				Temporal:    worker.TemporalConfig{Address: "127.0.0.1:1", Namespace: "default"},
			}
			if configured {
				cfg.LLM = llm.Config{APIKey: "local-test", BaseURL: "http://127.0.0.1:1/v1", Model: "gpt-6-luna", Effort: "medium"}
			}
			var boundary *offlineWorker
			var observer *worker.StatusObserver
			process := fxtest.New(t,
				Worker(cfg),
				fx.Decorate(func(w tworker.Worker) tworker.Worker {
					boundary = &offlineWorker{Worker: w}
					return boundary
				}),
				fx.Populate(&observer),
				fx.NopLogger,
			)
			process.RequireStart()
			t.Cleanup(func() {
				process.RequireStop()
				if !boundary.stopped {
					t.Error("worker did not stop")
				}
			})
			if !boundary.started {
				t.Fatal("worker did not start")
			}
			if (observer.AI != nil) != configured {
				t.Fatalf("AI availability binding does not match configuration: %v", observer.AI)
			}
		})
	}
}
