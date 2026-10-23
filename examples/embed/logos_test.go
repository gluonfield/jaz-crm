package embed

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	crm "github.com/gluonfield/jaz-crm"
	"github.com/gluonfield/jaz-crm/records"
)

// A host looks up its tenant's company logos through the public API and
// reads each by the token its records map to; another tenant neither sees
// those tokens nor sets off a look at the first tenant's sites.
func TestEmbeddedLogos(t *testing.T) {
	ctx := context.Background()
	pool := scratch(t)
	if err := crm.Migrate(ctx, pool, "crm"); err != nil {
		t.Fatal(err)
	}
	png := "\x89PNG\r\n\x1a\nbrightwater"
	pages := map[string]string{
		"/brightwater.example/":         `<html><head><link rel="icon" href="mark.png"></head></html>`,
		"/brightwater.example/mark.png": png,
		"/quietworks.example/":          `<html><head><title>Quietworks</title></head></html>`,
	}
	var mu sync.Mutex
	var asked []string
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path)
		mu.Unlock()
		page, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(page))
	}))
	t.Cleanup(site.Close)
	looks := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(asked)
	}
	c, err := crm.Open(ctx, pool, crm.Options{Schema: "crm", LogoClient: site.Client(), LogoHome: func(domain string) string { return site.URL + "/" + domain + "/" }})
	if err != nil {
		t.Fatal(err)
	}
	ours, err := c.Actor(ctx, crm.User{Tenant: "co-ours", ID: "u-ada", Name: "Ada", Email: "ada@ours.test"})
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := c.Actor(ctx, crm.User{Tenant: "co-theirs", ID: "u-eve", Name: "Eve", Email: "eve@theirs.test"})
	if err != nil {
		t.Fatal(err)
	}
	company := func(name string, domain string) records.Record {
		t.Helper()
		record, _, err := c.Records.Upsert(ctx, ours, ours.Source(), records.Write{Object: "companies", Set: map[string][]string{"name": {name}, "domains": {domain}}})
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	branded := company("Brightwater", "brightwater.example")
	plain := company("Quietworks", "quietworks.example")

	if err := c.RefreshLogos(ctx, theirs); err != nil || len(looks()) != 0 {
		t.Fatalf("another tenant's refresh looked at %v: %v", looks(), err)
	}
	if err := c.RefreshLogos(ctx, ours); err != nil {
		t.Fatal(err)
	}
	tokens, err := c.LogoTokens(ctx, ours, []string{branded.ID, plain.ID})
	if err != nil || len(tokens) != 1 || tokens[branded.ID] == "" {
		t.Fatalf("tokens %v: %v", tokens, err)
	}
	logo, err := c.Logo(ctx, tokens[branded.ID])
	if err != nil || logo.ContentType != "image/png" || string(logo.Image) != png {
		t.Fatalf("logo %q %q: %v", logo.ContentType, logo.Image, err)
	}
	if theirTokens, err := c.LogoTokens(ctx, theirs, []string{branded.ID}); err != nil || len(theirTokens) != 0 {
		t.Fatalf("another tenant maps our record to %v: %v", theirTokens, err)
	}
	for _, token := range []string{"00000000-0000-4000-8000-000000000000", "not-a-token"} {
		if _, err := c.Logo(ctx, token); !errors.Is(err, crm.ErrNoLogo) {
			t.Fatalf("token %s: %v", token, err)
		}
	}

	seen := len(looks())
	if err := c.RefreshLogos(ctx, ours); err != nil || len(looks()) != seen {
		t.Fatalf("a second refresh looked again at %v: %v", looks()[seen:], err)
	}
}
