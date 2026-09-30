package webhooks_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/webhooks"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

type woken []string

func (w *woken) Start(_ context.Context, id string) error {
	*w = append(*w, id)
	return nil
}

func (w *woken) Stop(context.Context, string) error { return nil }

func (w *woken) Step(context.Context, string) (string, error) { return "", nil }

func post(t *testing.T, h http.HandlerFunc, headers map[string]string, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res := httptest.NewRecorder()
	h(res, req)
	out, _ := io.ReadAll(res.Body)
	return res.Code, string(out)
}

func TestWebhooks(t *testing.T) {
	ctx := context.Background()
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@cas.dev")
	if err != nil {
		t.Fatal(err)
	}
	keys := auth.NewService(store, auth.Config{PublicURL: "http://crm.test"})
	key, _, err := keys.CreateKey(ctx, owner.ID, "recorder", "")
	if err != nil {
		t.Fatal(err)
	}
	var wakes woken
	conns, err := connections.NewService(store, connections.Config{}, &wakes)
	if err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	conn, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: owner.WorkspaceID, UserID: owner.ID, Provider: "google", Account: "owner@cas.dev", RefreshToken: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	if err := conns.SetChannel(ctx, conn.ID, connections.Channel{ID: conn.ID + ".1", Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	h := webhooks.NewHandler(conns, &wakes, convs, keys, webhooks.Config{}, log.New(io.Discard))

	if status, _ := post(t, h.Calendar, map[string]string{"X-Goog-Channel-ID": conn.ID + ".1", "X-Goog-Channel-Token": "guess"}, ""); status != http.StatusNotFound || len(wakes) != 0 {
		t.Fatalf("a wrong channel token woke the sync: %d %v", status, wakes)
	}
	if status, _ := post(t, h.Calendar, map[string]string{"X-Goog-Channel-ID": conn.ID + ".1", "X-Goog-Channel-Token": "secret"}, ""); status != http.StatusNoContent || len(wakes) != 1 || wakes[0] != conn.ID {
		t.Fatalf("calendar push: %d %v", status, wakes)
	}
	if status, _ := post(t, h.Gmail, nil, "{}"); status != http.StatusNotFound {
		t.Fatalf("Gmail push without Pub/Sub configured: %d", status)
	}
	signed := webhooks.NewHandler(conns, &wakes, convs, keys, webhooks.Config{Audience: "http://crm.test/webhooks/google/gmail"}, log.New(io.Discard))
	if status, _ := post(t, signed.Gmail, map[string]string{"Authorization": "Bearer forged"}, `{"message":{"data":""}}`); status != http.StatusUnauthorized {
		t.Fatalf("an unsigned Gmail push: %d", status)
	}

	report := `{"external_id":"rec-1","kind":"call","title":"Intro","people":["ada@acme.com"],"notes":"First call."}`
	if status, _ := post(t, h.Interactions, nil, report); status != http.StatusUnauthorized {
		t.Fatalf("a report without a key: %d", status)
	}
	auth := map[string]string{"Authorization": "Bearer " + key}
	status, first := post(t, h.Interactions, auth, report)
	if status != http.StatusOK {
		t.Fatalf("report: %d %s", status, first)
	}
	if _, again := post(t, h.Interactions, auth, strings.Replace(report, "First call.", "First call, revised.", 1)); again != first {
		t.Fatalf("a repeated report must update one interaction: %s vs %s", again, first)
	}
	if status, body := post(t, h.Interactions, auth, `{"kind":"lunch"}`); status != http.StatusBadRequest || !strings.Contains(body, "call, meeting or note") {
		t.Fatalf("a bad report: %d %s", status, body)
	}
}
