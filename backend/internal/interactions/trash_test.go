package interactions_test

import (
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

func TestRestoreContactRetainsMessagesAndNewerDecisions(t *testing.T) {
	e := setup(t, nil)
	e.ingest(t, message(e.conn, "o1", "t1", "owner@cas.dev", "ada@customer.io"))
	e.triage(t)
	ada := e.contacts(t, interactions.Kept)["ada@customer.io"]
	thread := e.timeline(t, ada.PersonID)[0]
	parts, err := e.store.Parts(ctx, []string{thread.ID})
	if err != nil || len(parts) != 1 {
		t.Fatalf("parts: %v %v", parts, err)
	}
	if err := e.store.SetPartContent(ctx, parts[0].ID, "Original message", "<p>Original message</p>"); err != nil {
		t.Fatal(err)
	}
	if err := e.crm.Delete(ctx, e.a, ada.PersonID); err != nil {
		t.Fatal(err)
	}
	if trash, err := e.crm.Trash(ctx, e.a); err != nil || len(trash) != 1 || trash[0].Name != "ada@customer.io" {
		t.Fatalf("nameless contact lost its identifying address: %v %v", trash, err)
	}
	if err := e.store.Relink(ctx, []string{thread.ID}); err != nil {
		t.Fatal(err)
	}
	if timeline := e.timeline(t, ada.PersonID); len(timeline) != 0 {
		t.Fatalf("trashed person's timeline remains active: %v", timeline)
	}
	if err := e.crm.Restore(ctx, e.a, ada.PersonID); err != nil {
		t.Fatal(err)
	}
	if got := e.contacts(t, interactions.Kept)["ada@customer.io"]; got.PersonID != ada.PersonID {
		t.Fatalf("restored contact changed identity: %v", got)
	}
	parts, err = e.store.Parts(ctx, []string{thread.ID})
	if err != nil || len(parts) != 1 || parts[0].Content == nil || *parts[0].Content != "Original message" || parts[0].HTML == nil || *parts[0].HTML != "<p>Original message</p>" {
		t.Fatalf("sync or restore discarded provider content: %v %v", parts, err)
	}
	if got := e.timeline(t, ada.PersonID); len(got) != 1 || got[0].ID != thread.ID {
		t.Fatalf("restore lost conversation links: %v", got)
	}
	if err := e.crm.Delete(ctx, e.a, ada.PersonID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Addresses: []string{"ada@customer.io"}, Reason: "manual exclusion"}); err != nil {
		t.Fatal(err)
	}
	if err := e.crm.Restore(ctx, e.a, ada.PersonID); err != nil {
		t.Fatal(err)
	}
	if got := e.contacts(t, interactions.Skipped)["ada@customer.io"]; got.Reason != "manual exclusion" {
		t.Fatalf("restore overwrote a newer triage decision: %v", got)
	}
}

func TestRestoreContactKeepsWebmailCompanyInTrash(t *testing.T) {
	e := setupManual(t, nil)
	company, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "companies", Set: map[string][]string{"domains": {"gmail.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.crm.Delete(ctx, e.a, company.ID); err != nil {
		t.Fatal(err)
	}
	e.ingest(t, message(e.conn, "i1", "t1", "sam@gmail.com", "owner@cas.dev"))
	if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Addresses: []string{"sam@gmail.com"}, Keep: true}); err != nil {
		t.Fatal(err)
	}
	if got := e.contacts(t, interactions.Kept)["sam@gmail.com"]; got.PersonID == "" {
		t.Fatalf("address approval did not keep the contact: %v", got)
	}
	if trash, err := e.crm.Trash(ctx, e.a); err != nil || len(trash) != 1 || trash[0].ID != company.ID || trash[0].Name != "gmail.com" {
		t.Fatalf("a webmail address restored the domain's company: %v %v", trash, err)
	}
}

func TestRestoreCompanyReversesItsExclusionOnly(t *testing.T) {
	e := setup(t, nil)
	e.ingest(t, message(e.conn, "o1", "t1", "owner@cas.dev", "keep@a.io"))
	e.triage(t)
	company, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "companies", Set: map[string][]string{"domains": {"a.io", "b.io"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.crm.Delete(ctx, e.a, company.ID); err != nil {
		t.Fatal(err)
	}
	e.ingest(t, message(e.conn, "i2", "t2", "new@a.io", "owner@cas.dev"), message(e.conn, "i3", "t3", "skip@b.io", "owner@cas.dev"))
	if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Domains: []string{"b.io"}, Reason: "manual exclusion"}); err != nil {
		t.Fatal(err)
	}
	if err := e.crm.Restore(ctx, e.a, company.ID); err != nil {
		t.Fatal(err)
	}
	e.triage(t)
	for _, address := range []string{"keep@a.io", "new@a.io"} {
		if got := e.contacts(t, interactions.Kept)[address]; got.PersonID == "" {
			t.Fatalf("restored domain still excludes %s: %v", address, got)
		}
	}
	if got := e.contacts(t, interactions.Skipped)["skip@b.io"]; got.Reason != "manual exclusion" {
		t.Fatalf("restore overwrote a newer domain decision: %v", got)
	}
	companies, _, err := e.crm.Search(ctx, e.a, records.Search{Object: "companies"})
	if err != nil || len(companies) != 1 || companies[0].ID != company.ID {
		t.Fatalf("sync duplicated a restored company: %v %v", companies, err)
	}
}

func TestRestoreCustomRecordDoesNotRestoreCompanyTriage(t *testing.T) {
	e := setupManual(t, nil)
	if _, err := e.crm.CreateObject(ctx, e.a, "suppliers", "Suppliers"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.crm.CreateAttribute(ctx, e.a, "suppliers", records.Attribute{Slug: "domain", Name: "Domain", Type: records.Domain, Unique: true}); err != nil {
		t.Fatal(err)
	}
	company, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "companies", Set: map[string][]string{"domains": {"a.io"}}})
	if err != nil {
		t.Fatal(err)
	}
	supplier, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "suppliers", Set: map[string][]string{"domain": {"a.io"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{company.ID, supplier.ID} {
		if err := e.crm.Delete(ctx, e.a, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.crm.Restore(ctx, e.a, supplier.ID); err != nil {
		t.Fatal(err)
	}
	rules, err := e.svc.DomainRules(ctx, e.a)
	if err != nil || len(rules) != 1 || rules[0].Decision != interactions.Skipped {
		t.Fatalf("a custom record reversed company exclusion: %v %v", rules, err)
	}
	if err := e.crm.Delete(ctx, e.a, supplier.ID); err != nil {
		t.Fatal(err)
	}
	e.ingest(t, message(e.conn, "i1", "t1", "sam@a.io", "owner@cas.dev"))
	if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Addresses: []string{"sam@a.io"}, Keep: true}); err != nil {
		t.Fatal(err)
	}
	if trash, err := e.crm.Trash(ctx, e.a); err != nil || len(trash) != 1 || trash[0].ID != supplier.ID {
		t.Fatalf("contact approval restored a custom record: %v %v", trash, err)
	}
}
