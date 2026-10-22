// Package worker syncs connections with Temporal: one long-running workflow
// per connection, and one per meeting whose transcript is due.
package worker

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"

	"github.com/charmbracelet/log"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	tlog "go.temporal.io/sdk/log"
	"go.uber.org/fx"
)

const TaskQueue = "jaz-crm"

// TemporalConfig reaches a local Temporal server, or Temporal Cloud when an
// API key is set.
type TemporalConfig struct {
	Address   string
	Namespace string
	APIKey    string
}

// NewClient connects lazily, so the server starts before Temporal is up.
func NewClient(lc fx.Lifecycle, cfg TemporalConfig, logger *log.Logger) (client.Client, error) {
	opts := client.Options{HostPort: cfg.Address, Namespace: cfg.Namespace, Logger: tlog.NewStructuredLogger(slog.New(logger.WithPrefix("temporal")))}
	if cfg.APIKey != "" {
		opts.ConnectionOptions.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
		opts.Credentials = client.NewAPIKeyStaticCredentials(cfg.APIKey)
	}
	c, err := client.NewLazyClient(opts)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.StopHook(c.Close))
	return c, nil
}

// Starter starts, wakes and stops connection syncs.
type Starter struct {
	client client.Client
}

func NewStarter(c client.Client) *Starter {
	return &Starter{client: c}
}

func syncID(connectionID string) string {
	return "connection-sync-" + connectionID
}

// Start wakes a connection's sync, starting it when it is not running.
func (s *Starter) Start(ctx context.Context, connectionID string) error {
	_, err := s.client.SignalWithStartWorkflow(ctx, syncID(connectionID), SignalWake, nil,
		client.StartWorkflowOptions{ID: syncID(connectionID), TaskQueue: TaskQueue}, ConnectionSync, connectionID)
	return err
}

// Step names the activity the connection's sync is running, or "" between
// passes and when no sync runs.
func (s *Starter) Step(ctx context.Context, connectionID string) (string, error) {
	d, err := s.client.DescribeWorkflowExecution(ctx, syncID(connectionID), "")
	if errors.As(err, new(*serviceerror.NotFound)) || err == nil && len(d.PendingActivities) == 0 {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return d.PendingActivities[0].GetActivityType().GetName(), nil
}

func (s *Starter) Stop(ctx context.Context, connectionID string) error {
	err := s.client.TerminateWorkflow(ctx, syncID(connectionID), "", "disconnected")
	if errors.As(err, new(*serviceerror.NotFound)) {
		return nil
	}
	return err
}
