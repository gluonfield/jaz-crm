// Package app wires the process: configuration, storage, services and HTTP.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/server"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"go.uber.org/fx"
)

type Config struct {
	Addr        string
	DatabaseURL string
	PublicURL   string
	OIDC        auth.OIDCConfig
	Auth        auth.Config
	Workspaces  workspaces.Config
	Owner       Owner
}

// Owner is the account a deployment provisions at startup, and the API key
// it gives clients such as Jaz, so the deployment works without a sign-in.
type Owner struct {
	Email  string
	APIKey string
}

// ParseConfig reads flags, each defaulting to an environment variable, and
// the auth settings, which come from the environment only.
func ParseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	cfg := Config{}
	fs.StringVar(&cfg.Addr, "addr", env("ADDR", ":7500"), "HTTP listen address (ADDR)")
	fs.StringVar(&cfg.DatabaseURL, "database-url", env("DATABASE_URL", "postgres://jazcrm:jazcrm@localhost:55532/jazcrm?sslmode=disable"), "Postgres URL (DATABASE_URL)")
	fs.StringVar(&cfg.PublicURL, "public-url", env("PUBLIC_URL", "http://localhost:7500"), "URL people and agents reach the server at (PUBLIC_URL)")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	cfg.PublicURL = strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	cfg.OIDC = auth.OIDCConfig{
		Issuer:       strings.TrimSpace(os.Getenv("OIDC_ISSUER")),
		ClientID:     strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("OIDC_CLIENT_SECRET")),
		RedirectURL:  cfg.PublicURL + "/auth/callback",
	}
	cfg.Auth = auth.Config{PublicURL: cfg.PublicURL}
	cfg.Workspaces = workspaces.Config{
		AllowedEmailDomains: list(os.Getenv("ALLOWED_EMAIL_DOMAINS")),
		AllowedEmails:       list(os.Getenv("ALLOWED_EMAILS")),
	}
	cfg.Owner = Owner{Email: strings.TrimSpace(os.Getenv("OWNER_EMAIL")), APIKey: strings.TrimSpace(os.Getenv("OWNER_API_KEY"))}
	if cfg.Owner.APIKey != "" && cfg.Owner.Email == "" {
		return cfg, errors.New("OWNER_API_KEY needs OWNER_EMAIL, the account it belongs to")
	}
	return cfg, nil
}

func list(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Options(cfg Config) fx.Option {
	return fx.Options(
		fx.Supply(cfg, cfg.Auth, cfg.Workspaces, cfg.OIDC),
		fx.Provide(
			NewLogger,
			fx.Annotate(OpenStore, fx.As(fx.Self()), fx.As(new(storage.AuthStore)), fx.As(new(storage.WorkspaceStore)), fx.As(new(storage.RecordStore))),
			auth.NewService,
			workspaces.NewService,
			auth.NewOIDC,
			records.NewService,
			authapi.NewHandler,
			mcpapi.NewHandler,
			server.New,
		),
		fx.Invoke(ProvisionOwner, StartHTTP),
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
