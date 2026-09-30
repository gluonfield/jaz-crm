package app

import (
	"encoding/base64"
	"errors"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/classifier"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/webhooks"
	"github.com/gluonfield/jaz-crm/backend/internal/server"
	"github.com/gluonfield/jaz-crm/backend/internal/worker"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

type Config struct {
	Addr        string
	DatabaseURL string
	PublicURL   string
	WebDir      server.WebDir
	OIDC        auth.OIDCConfig
	Auth        auth.Config
	Workspaces  workspaces.Config
	Owner       Owner
	Temporal    worker.TemporalConfig
	Connections connections.Config
	Sync        worker.Config
	Webhooks    webhooks.Config
	LLM         classifier.Config
}

// Owner is the account a deployment provisions at startup, and the API key
// it gives clients such as Jaz, so the deployment works without a sign-in.
type Owner struct {
	Email  string
	APIKey string
}

// ParseConfig reads flags, each defaulting to an environment variable, and
// the remaining settings from the environment.
func ParseConfig(name string, args []string) (Config, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	cfg := Config{}
	var web string
	fs.StringVar(&cfg.Addr, "addr", env("ADDR", ":7500"), "HTTP listen address (ADDR)")
	fs.StringVar(&cfg.DatabaseURL, "database-url", env("DATABASE_URL", "postgres://jazcrm:jazcrm@localhost:55532/jazcrm?sslmode=disable"), "Postgres URL (DATABASE_URL)")
	fs.StringVar(&cfg.PublicURL, "public-url", env("PUBLIC_URL", "http://localhost:7500"), "URL people and agents reach the server at (PUBLIC_URL)")
	fs.StringVar(&web, "web-dir", env("WEB_DIR", ""), "directory of the built web app (WEB_DIR)")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	cfg.WebDir = server.WebDir(web)
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
	cfg.Temporal = worker.TemporalConfig{
		Address:   env("TEMPORAL_ADDRESS", "localhost:7533"),
		Namespace: env("TEMPORAL_NAMESPACE", "default"),
		APIKey:    strings.TrimSpace(os.Getenv("TEMPORAL_API_KEY")),
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("ENCRYPTION_KEY")))
	if err != nil {
		return cfg, errors.New("ENCRYPTION_KEY must be base64, such as the output of openssl rand -base64 32")
	}
	days, err := strconv.Atoi(env("BACKFILL_DAYS", "730"))
	if err != nil || days < 1 {
		return cfg, errors.New("BACKFILL_DAYS must be a positive number of days")
	}
	cfg.Connections = connections.Config{
		Google: google.OAuthConfig{
			ClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
			ClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
			RedirectURL:  cfg.PublicURL + "/connections/google/callback",
		},
		Key:       key,
		Endpoints: google.Production,
		Backfill:  time.Duration(days) * 24 * time.Hour,
	}
	cfg.Sync = worker.Config{PubSubTopic: strings.TrimSpace(os.Getenv("GMAIL_PUBSUB_TOPIC"))}
	if strings.HasPrefix(cfg.PublicURL, "https://") {
		cfg.Sync.CalendarWebhook = cfg.PublicURL + "/webhooks/google/calendar"
	}
	if cfg.Sync.PubSubTopic != "" {
		cfg.Webhooks = webhooks.Config{Audience: cfg.PublicURL + "/webhooks/google/gmail", ServiceAccount: strings.TrimSpace(os.Getenv("GMAIL_PUBSUB_SERVICE_ACCOUNT"))}
	}
	cfg.LLM = classifier.Config{
		BaseURL: env("LLM_BASE_URL", "https://api.openai.com/v1"),
		APIKey:  strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		Model:   strings.TrimSpace(os.Getenv("LLM_MODEL")),
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
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
