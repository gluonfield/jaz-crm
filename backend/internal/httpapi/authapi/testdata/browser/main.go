package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/crm/interactions"
	"github.com/gluonfield/jaz-crm/backend/crm/records"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/changes"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/changesapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/connectapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/logosapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/pageimagesapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/webhooks"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/server"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	signin "github.com/gluonfield/jaz-tasks/auth"
	"github.com/jackc/pgx/v5"
	"go.uber.org/fx"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = postgrestest.DefaultURL
	}
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		return err
	}
	defer admin.Close(ctx)
	name := "crm_shortcuts_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		return err
	}
	defer admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
	u, err := url.Parse(base)
	if err != nil {
		return err
	}
	u.Path = "/" + name
	store, err := postgres.Open(ctx, u.String())
	if err != nil {
		return err
	}
	defer store.Close()
	srv := httptest.NewUnstartedServer(nil)
	origin := "http://" + srv.Listener.Addr().String()
	keys := auth.NewService(store, auth.Config{PublicURL: origin})
	people := workspaces.NewService(store, workspaces.Config{})
	user, err := people.Provision(ctx, "shortcuts@example.test")
	if err != nil {
		return err
	}
	logger := log.New(io.Discard)
	authn, err := authapi.NewHandler(keys, people, signin.Config{PublicURL: origin}, logger)
	if err != nil {
		return err
	}
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	conns, err := connections.NewService(store, connections.Config{}, nil)
	if err != nil {
		return err
	}
	lg := logos.NewService(store, logos.Fetcher{})
	agents := mcpapi.NewHandler(mcpapi.Services{Records: crm, Workspaces: people, Interactions: convs, Connections: conns, Logos: lg}, keys, logger)
	var hub *changes.Hub
	lifecycle := fx.New(fx.NopLogger, fx.Invoke(func(lc fx.Lifecycle) {
		hub = changes.NewHub(lc, store, logger)
	}))
	if err := lifecycle.Start(ctx); err != nil {
		return err
	}
	defer lifecycle.Stop(ctx)
	srv.Config.Handler = server.New(authn, agents, changesapi.NewHandler(hub), connectapi.NewHandler(conns, keys, logger), webhooks.NewHandler(conns, nil, convs, keys, webhooks.Config{}, logger), logosapi.NewHandler(lg, logger), pageimagesapi.NewHandler(crm, logger), server.WebDir(os.Getenv("SHORTCUT_WEB_DIR")))
	srv.Start()
	defer srv.Close()
	token, _, err := keys.CreateSession(ctx, user.ID)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"url": origin, "session": token}); err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, os.Stdin)
	return err
}
