package changesapi

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/changes"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	signin "github.com/gluonfield/jaz-tasks/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/fx/fxtest"
)

// A change reaches the open pages of its workspace whoever writes it, an
// agent over MCP or a sync in the worker, whether the changed rows name the
// workspace or a parent does, and no page of another workspace. What a quiet
// sync pass rewrites as it was notifies nothing.
func TestChangesReachTheirWorkspaceOnly(t *testing.T) {
	ctx := context.Background()
	s := start(t)
	connection, err := s.store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: s.workspace, UserID: s.owner, Provider: "google", Account: "owner@jaz.test", RefreshToken: []byte("sealed")})
	if err != nil {
		t.Fatal(err)
	}
	ours, theirs := s.open(t, s.owner), s.open(t, s.other)

	s.write(t, map[string]any{"name": "Acme", "domains": "acme.test"})
	ours.next(t)

	draft := storage.GmailDraft{ConnectionID: connection.ID, DraftID: "d1", MessageID: "m1", Subject: "Hello", State: "draft", At: time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)}
	if err := s.store.UpsertGmailDraft(ctx, draft); err != nil {
		t.Fatal(err)
	}
	ours.next(t)
	// A quiet pass moves its history cursor's time and rewrites its drafts.
	for range 2 {
		if err := s.store.SetCursor(ctx, connection.ID, connections.StreamHistory, "42"); err != nil {
			t.Fatal(err)
		}
		if err := s.store.UpsertGmailDraft(ctx, draft); err != nil {
			t.Fatal(err)
		}
	}
	ours.none(t, gap+time.Second)

	if err := s.store.SaveLogo(ctx, storage.Logo{Domain: "acme.test", ContentType: "image/svg+xml", Image: []byte("<svg/>")}); err != nil {
		t.Fatal(err)
	}
	ours.next(t)
	if err := s.store.DeleteConnection(ctx, s.workspace, connection.ID); err != nil {
		t.Fatal(err)
	}
	ours.next(t)
	theirs.none(t, 500*time.Millisecond)
}

// Closing the hub, as the server shuts down, ends open streams, which would
// otherwise hold the shutdown open.
func TestClosingTheHubEndsStreams(t *testing.T) {
	s := start(t)
	page := s.open(t, s.owner)
	s.hub.Close()
	select {
	case _, open := <-page:
		if open {
			t.Fatal("the stream sent an event instead of ending")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream outlived the hub")
	}
}

type stack struct {
	url               string
	store             *postgres.Store
	keys              *auth.Service
	hub               *changes.Hub
	owner, other      string
	workspace, apiKey string
}

func start(t *testing.T) stack {
	t.Helper()
	ctx := context.Background()
	store := postgrestest.New(t)
	logger := log.New(io.Discard)
	keys := auth.NewService(store, auth.Config{PublicURL: "http://crm.test"})
	people := workspaces.NewService(store, workspaces.Config{})
	owner, err := people.Provision(ctx, "owner@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	other, err := people.Provision(ctx, "other@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	apiKey, _, err := keys.CreateKey(ctx, owner.ID, "agent", "")
	if err != nil {
		t.Fatal(err)
	}
	authn, err := authapi.NewHandler(keys, people, signin.Config{PublicURL: "http://crm.test"}, logger)
	if err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	conns, err := connections.NewService(store, connections.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	agents := mcpapi.NewHandler(mcpapi.Services{Records: crm, Workspaces: people, Interactions: interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm}), Connections: conns, Logos: logos.NewService(store, logos.Fetcher{})}, keys, logger)
	lc := fxtest.NewLifecycle(t)
	hub := changes.NewHub(lc, store, logger)
	lc.RequireStart()
	t.Cleanup(hub.Close)
	mux := http.NewServeMux()
	mux.Handle("/mcp", agents.MCP)
	mux.Handle("GET /api/changes", authn.Session(NewHandler(hub)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return stack{url: srv.URL, store: store, keys: keys, hub: hub, owner: owner.ID, other: other.ID, workspace: owner.WorkspaceID, apiKey: apiKey}
}

// write adds a company as the owner's agent.
func (s stack) write(t *testing.T, values map[string]any) {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: s.url + "/mcp", HTTPClient: &http.Client{Transport: bearer(s.apiKey)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "upsert_record", Arguments: map[string]any{"object": "companies", "values": values}})
	if err != nil || res.IsError {
		t.Fatalf("upsert_record: %v %v", res, err)
	}
}

// page receives a signed-in page's events, and closes when its stream ends.
type page <-chan struct{}

func (s stack) open(t *testing.T, user string) page {
	t.Helper()
	token, _, err := s.keys.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/api/changes", nil)
	req.AddCookie(&http.Cookie{Name: "jc_session", Value: token})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	events := make(chan struct{}, 8)
	go func() {
		defer close(events)
		defer res.Body.Close()
		lines := bufio.NewScanner(res.Body)
		data := false
		for lines.Scan() {
			data = data || strings.HasPrefix(lines.Text(), "data:")
			if lines.Text() == "" && data {
				events <- struct{}{}
				data = false
			}
		}
	}()
	return events
}

func (p page) next(t *testing.T) {
	t.Helper()
	select {
	case _, open := <-p:
		if !open {
			t.Fatal("the stream ended")
		}
	case <-time.After(gap + 5*time.Second):
		t.Fatal("no event")
	}
}

func (p page) none(t *testing.T, wait time.Duration) {
	t.Helper()
	select {
	case <-p:
		t.Fatal("unexpected event")
	case <-time.After(wait):
	}
}

type bearer string

func (key bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+string(key))
	return http.DefaultTransport.RoundTrip(req)
}
