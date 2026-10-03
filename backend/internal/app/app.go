// Package app wires the processes: the server, which serves people and
// agents, and the worker, which syncs connections.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/connectapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/logosapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/webhooks"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/llm"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/server"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/worker"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// shared is what both processes run on: storage, the domain services and
// the Temporal client.
func shared(cfg Config) fx.Option {
	options := []fx.Option{
		fx.Supply(cfg, cfg.Auth, cfg.Workspaces, cfg.SignIn, cfg.Temporal, cfg.Connections, cfg.Sync, cfg.Webhooks, cfg.WebDir),
		fx.Provide(
			NewLogger,
			fx.Annotate(OpenStore, fx.As(fx.Self()), fx.As(new(storage.AuthStore)), fx.As(new(storage.WorkspaceStore)),
				fx.As(new(storage.RecordStore)), fx.As(new(storage.ConnectionStore)), fx.As(new(storage.InteractionStore)), fx.As(new(storage.LogoStore))),
			records.NewService,
			logos.NewFetcher,
			logos.NewService,
			interactions.NewService,
			connections.NewService,
			followups.NewService,
			followups.NewAgent,
			worker.NewClient,
			fx.Annotate(worker.NewStarter, fx.As(fx.Self()), fx.As(new(connections.Syncer))),
		),
	}
	if cfg.LLM.APIKey != "" {
		options = append(options, fx.Supply(cfg.LLM), fx.Provide(fx.Annotate(llm.New, fx.As(new(interactions.Classifier)), fx.As(new(followups.Planner)), fx.As(new(followups.Summarizer)), fx.As(new(followups.Rewriter)))))
	}
	return fx.Options(options...)
}

func Server(cfg Config) fx.Option {
	return fx.Options(
		shared(cfg),
		fx.Provide(
			auth.NewService,
			workspaces.NewService,
			authapi.NewHandler,
			mcpapi.NewHandler,
			connectapi.NewHandler,
			webhooks.NewHandler,
			logosapi.NewHandler,
			server.New,
		),
		fx.Invoke(ProvisionOwner, StartHTTP),
	)
}

func Worker(cfg Config) fx.Option {
	return fx.Options(
		shared(cfg),
		fx.Provide(worker.NewActivities, worker.NewWorker),
		fx.Invoke(worker.Run),
	)
}

func NewLogger() *log.Logger {
	level, err := log.ParseLevel(env("LOG_LEVEL", "info"))
	if err != nil {
		level = log.InfoLevel
	}
	return log.NewWithOptions(os.Stderr, log.Options{ReportTimestamp: true, Level: level})
}

func OpenStore(lc fx.Lifecycle, cfg Config) (*postgres.Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.StopHook(store.Close))
	return store, nil
}

// ProvisionOwner creates the OWNER_EMAIL account when it is new and registers
// OWNER_API_KEY for it, before the server takes requests.
func ProvisionOwner(cfg Config, people *workspaces.Service, keys *auth.Service) error {
	if cfg.Owner.Email == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	user, err := people.Provision(ctx, cfg.Owner.Email)
	if err != nil || cfg.Owner.APIKey == "" {
		return err
	}
	if err := keys.ProvisionKey(ctx, user.ID, "OWNER_API_KEY", cfg.Owner.APIKey); err != nil {
		return fmt.Errorf("OWNER_API_KEY: %w", err)
	}
	return nil
}

func StartHTTP(lc fx.Lifecycle, handler http.Handler, cfg Config, logger *log.Logger) {
	srv := &http.Server{Addr: cfg.Addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			listener, err := net.Listen("tcp", cfg.Addr)
			if err != nil {
				return err
			}
			logger.Info("jaz crm listening", "addr", cfg.Addr, "url", cfg.PublicURL)
			go func() {
				if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("serve", "error", err)
				}
			}()
			return nil
		},
		OnStop: srv.Shutdown,
	})
}

// MintAPIKey opens the database directly to issue a key outside the server.
func MintAPIKey(ctx context.Context, cfg Config, email, workspace string) (string, error) {
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return "", err
	}
	defer store.Close()
	return auth.NewService(store, cfg.Auth).CreateKeyForEmail(ctx, email, workspace)
}

// Run starts a process and stops it when the process is told to.
func Run(process fx.Option) error {
	fxApp := fx.New(fx.StopTimeout(time.Minute), fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }), process)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := fxApp.Start(ctx); err != nil {
		return err
	}
	<-fxApp.Wait()
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return fxApp.Stop(ctx)
}
