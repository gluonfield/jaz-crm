package interactions_test

import (
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func TestDeletedCompanyExcludesItsDomains(t *testing.T) {
	e := setupManual(t, &fakeClassifier{})
	setSettings(t, e, storage.TriageSettings{AutoKeepEmail: true, AutoKeepMeetings: true, AutoKeepAi: true})
	e.ingest(t, message(e.conn, "o1", "t1", "owner@cas.dev", "keep@a.io"))
	e.triage(t)
	company, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "companies", Set: map[string][]string{"domains": {"a.io", "boardy.ai"}}})
	if err != nil {
		t.Fatal(err)
	}
	known, err := e.svc.Known(ctx, e.a.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.crm.Delete(ctx, e.b, company.ID); err == nil {
		t.Fatal("another workspace deleted the company")
	}
	if err := e.crm.Delete(ctx, e.a, company.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.IngestEmail(ctx, known, message(e.conn, "o2", "t2", "owner@cas.dev", "new@boardy.ai", "keep@a.io")); err != nil {
		t.Fatal(err)
	}
	if got := e.contacts(t, interactions.Skipped)["new@boardy.ai"]; got.DecidedBy != interactions.ByUser {
		t.Fatalf("stale sync snapshot ignored domain exclusion: %+v", got)
	}
	e.triage(t)
	for _, address := range []string{"keep@a.io", "new@boardy.ai"} {
		if got := e.contacts(t, interactions.Skipped)[address]; got.DecidedBy != interactions.ByUser || got.Reason == "" {
			t.Fatalf("deleted domain allowed %s: %+v", address, got)
		}
	}
	companies, err := e.crm.Search(ctx, e.a, records.Search{Object: "companies"})
	if err != nil || len(companies) != 0 {
		t.Fatalf("company recreated: %+v %v", companies, err)
	}
	rules, err := e.svc.DomainRules(ctx, e.a)
	if err != nil || len(rules) != 2 {
		t.Fatalf("persistent domain exclusions: %+v %v", rules, err)
	}
	if err := e.svc.ForgetDomainRule(ctx, e.b, "boardy.ai"); err == nil {
		t.Fatal("another workspace removed the exclusion")
	}

	if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Addresses: []string{"new@boardy.ai"}, Keep: true}); err != nil {
		t.Fatal(err)
	}
	companies, err = e.crm.Search(ctx, e.a, records.Search{Object: "companies"})
	if err != nil || len(companies) != 1 {
		t.Fatalf("explicit address approval did not restore company: %+v %v", companies, err)
	}
	if err := e.crm.Delete(ctx, e.a, companies[0].ID); err != nil {
		t.Fatal(err)
	}
	setSettings(t, e, storage.TriageSettings{})
	if err := e.svc.ForgetDomainRule(ctx, e.a, "boardy.ai"); err != nil {
		t.Fatal(err)
	}
	e.ingest(t, message(e.conn, "i3", "t3", "review@boardy.ai", "owner@cas.dev"))
	e.triage(t)
	if got := e.contacts(t, interactions.Pending)["review@boardy.ai"]; got.Address == "" {
		t.Fatalf("removed rule still skips new contacts: %+v", got)
	}
	if n, err := e.svc.Decide(ctx, e.a, interactions.Decision{Domains: []string{"boardy.ai"}, Keep: true}); err != nil || n != 2 {
		t.Fatalf("explicitly restore the domain: %d %v", n, err)
	}
	e.ingest(t, message(e.conn, "i4", "t4", "another@boardy.ai", "owner@cas.dev"))
	e.triage(t)
	if got := e.contacts(t, interactions.Kept)["another@boardy.ai"]; got.PersonID == "" || got.DecidedBy != interactions.ByUser {
		t.Fatalf("explicit always-keep rule not applied: %+v", got)
	}
}

func TestLateClassifierCannotOverrideDeletionOrDisabledApprovals(t *testing.T) {
	f := &fakeClassifier{}
	e := setupManual(t, f)
	company, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "companies", Set: map[string][]string{"domains": {"a.io"}}})
	if err != nil {
		t.Fatal(err)
	}
	setSettings(t, e, storage.TriageSettings{AutoKeepAi: true})
	e.ingest(t, message(e.conn, "i1", "t1", "keep@a.io", "owner@cas.dev"))
	f.before = func() {
		if err := e.crm.Delete(ctx, e.a, company.ID); err != nil {
			t.Fatal(err)
		}
	}
	e.triage(t)
	if got := e.contacts(t, interactions.Skipped)["keep@a.io"]; got.DecidedBy != interactions.ByUser || got.Reason == "" {
		t.Fatalf("late AI approval overrode deletion: %+v", got)
	}
	companies, err := e.crm.Search(ctx, e.a, records.Search{Object: "companies"})
	if err != nil || len(companies) != 0 {
		t.Fatalf("late AI approval recreated company: %+v %v", companies, err)
	}
	if err := e.svc.ForgetDomainRule(ctx, e.a, "a.io"); err != nil {
		t.Fatal(err)
	}
	e.ingest(t, message(e.conn, "i2", "t2", "another@a.io", "owner@cas.dev"))
	f.before = func() {
		setSettings(t, e, storage.TriageSettings{})
	}
	e.triage(t)
	if got := e.contacts(t, interactions.Pending)["another@a.io"]; got.Address == "" || got.DecidedBy != "" {
		t.Fatalf("disabled AI still settled a contact: %+v", got)
	}
	setSettings(t, e, storage.TriageSettings{AutoKeepAi: true})
	e.ingest(t, message(e.conn, "i3", "t3", "skip@b.io", "owner@cas.dev"))
	f.before = func() {
		if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Addresses: []string{"skip@b.io"}, Keep: true}); err != nil {
			t.Fatal(err)
		}
	}
	e.triage(t)
	if got := e.contacts(t, interactions.Kept)["skip@b.io"]; got.DecidedBy != interactions.ByUser || got.PersonID == "" {
		t.Fatalf("late AI skip overrode manual approval: %+v", got)
	}
}

func setSettings(t *testing.T, e env, settings storage.TriageSettings) {
	t.Helper()
	if err := e.store.UpdateTriageSettings(ctx, e.a.WorkspaceID, settings); err != nil {
		t.Fatal(err)
	}
}
