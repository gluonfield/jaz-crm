package logosapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/logosapi"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

// A logo serves by its token, sandboxed; neither a guessed token nor the
// domain finds it.
func TestServesLogosByToken(t *testing.T) {
	ctx := context.Background()
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@cas.dev")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	acme, _, err := records.NewService(store).Upsert(ctx, actor, records.SourceUser, records.Write{Object: "companies", Set: map[string][]string{"name": {"Acme"}, "domains": {"acme.io"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLogo(ctx, storage.Logo{Domain: "acme.io", ContentType: "image/svg+xml", Image: []byte("<svg/>")}); err != nil {
		t.Fatal(err)
	}
	svc := logos.NewService(store, logos.Fetcher{})
	tokens, err := svc.Tokens(ctx, actor, []string{acme.ID})
	if err != nil || tokens[acme.ID] == "" {
		t.Fatalf("tokens %v %v", tokens, err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /logos/{token}", logosapi.NewHandler(svc, log.New(io.Discard)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/logos/" + tokens[acme.ID])
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "<svg/>" || res.Header.Get("Content-Type") != "image/svg+xml" || res.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("logo: %d %q %v", res.StatusCode, body, res.Header)
	}
	for _, probe := range []string{"acme.io", "00000000-0000-4000-8000-000000000000", "urn:uuid:00000000-0000-4000-8000-000000000000"} {
		res, err := http.Get(srv.URL + "/logos/" + probe)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", probe, res.StatusCode)
		}
	}
}
