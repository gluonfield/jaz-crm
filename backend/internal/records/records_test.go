package records_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

var ctx = context.Background()

// setup returns the service and the owners of two separate workspaces.
func setup(t *testing.T) (*records.Service, auth.Actor, auth.Actor) {
	t.Helper()
	store := postgrestest.New(t)
	people := workspaces.NewService(store, workspaces.Config{})
	var actors []auth.Actor
	for _, email := range []string{"a@jaz.test", "b@jaz.test"} {
		user, err := people.Provision(ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		actors = append(actors, auth.Actor{UserID: user.ID, WorkspaceID: user.WorkspaceID})
	}
	return records.NewService(store), actors[0], actors[1]
}

func upsert(t *testing.T, svc *records.Service, actor auth.Actor, source records.Source, w records.Write) (records.Record, []records.Skip) {
	t.Helper()
	record, skips, err := svc.Upsert(ctx, actor, source, w)
	if err != nil {
		t.Fatalf("upsert %+v: %v", w, err)
	}
	return record, skips
}

// values lists an attribute's current values: text, or referenced record ids.
func values(r records.Record, attr string) []string {
	out := []string{}
	for _, f := range r.Fields {
		if f.Attribute != attr {
			continue
		}
		for _, v := range f.Values {
			if v.RecordID != "" {
				out = append(out, v.RecordID)
			} else {
				out = append(out, v.Text)
			}
		}
	}
	return out
}

func set(pairs ...string) map[string][]string {
	out := map[string][]string{}
	for i := 0; i < len(pairs); i += 2 {
		out[pairs[i]] = append(out[pairs[i]], pairs[i+1])
	}
	return out
}

func TestUpsertMatchesByUniqueValue(t *testing.T) {
	svc, a, _ := setup(t)
	ada, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Ada Lovelace", "email_addresses", "Ada <ADA@Example.com>")})
	if got := values(ada, "email_addresses"); !slices.Equal(got, []string{"ada@example.com"}) {
		t.Fatalf("email: %v", got)
	}
	again, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("email_addresses", "ada@example.com", "job_title", "Mathematician")})
	if again.ID != ada.ID || !slices.Equal(values(again, "name"), []string{"Ada Lovelace"}) || !slices.Equal(values(again, "job_title"), []string{"Mathematician"}) {
		t.Fatalf("a known email must update its record: %+v", again)
	}

	grace, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Grace", "email_addresses", "grace@example.com")})
	_, _, err := svc.Upsert(ctx, a, records.SourceAgent, records.Write{Object: "people", RecordID: grace.ID, Set: set("email_addresses", "ada@example.com")})
	if err == nil || !strings.Contains(err.Error(), ada.ID) {
		t.Fatalf("an email held by another record: %v", err)
	}
	_, _, err = svc.Upsert(ctx, a, records.SourceAgent, records.Write{Object: "people", Set: set("email_addresses", "ada@example.com", "email_addresses", "grace@example.com")})
	if err == nil || !strings.Contains(err.Error(), "several") {
		t.Fatalf("values matching two records: %v", err)
	}
	if _, _, err := svc.Upsert(ctx, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "One", "name", "Two")}); err == nil {
		t.Fatal("a single-valued attribute took two values")
	}
	if _, _, err := svc.Upsert(ctx, a, records.SourceAgent, records.Write{Object: "people", Set: set("email_addresses", "not an email")}); err == nil {
		t.Fatal("an invalid email was stored")
	}
}

// Links identify people the way emails do: variants of one profile are the
// same link, wherever the profile was copied from.
func TestLinksIdentifyRecords(t *testing.T) {
	svc, a, _ := setup(t)
	jim, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Jim", "links", "https://uk.linkedin.com/in/Jim-Mayer/?trk=profile", "links", "https://twitter.com/JimMayer")})
	for _, variant := range []string{"linkedin.com/in/jim-mayer", "https://www.linkedin.com/in/JIM-MAYER", "x.com/jimmayer/"} {
		again, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("links", variant)})
		if again.ID != jim.ID || len(values(again, "links")) != 2 {
			t.Fatalf("%s found another record or added a duplicate: %+v", variant, again)
		}
	}
	other, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Other", "links", "https://jim.example/about")})
	if _, _, err := svc.Upsert(ctx, a, records.SourceAgent, records.Write{Object: "people", RecordID: other.ID, Set: set("links", "https://x.com/jimmayer")}); err == nil || !strings.Contains(err.Error(), jim.ID) {
		t.Fatalf("a link held by another person: %v", err)
	}
}

// A write replaces only values from its own or a lower-ranked source.
func TestSourcePrecedence(t *testing.T) {
	svc, a, _ := setup(t)
	p, _ := upsert(t, svc, a, records.SourceSync, records.Write{Object: "people", Set: set("name", "Ada", "job_title", "Engineer", "email_addresses", "ada@example.com")})
	p, skips := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", RecordID: p.ID, Set: set("job_title", "CTO")})
	if !slices.Equal(values(p, "job_title"), []string{"CTO"}) || len(skips) != 0 {
		t.Fatalf("an agent replaces a synced value: %v %v", values(p, "job_title"), skips)
	}
	_, skips = upsert(t, svc, a, records.SourceSync, records.Write{Object: "people", RecordID: p.ID, Set: set("job_title", "Engineer")})
	if len(skips) != 1 || skips[0].Value != "CTO" || skips[0].Source != records.SourceAgent {
		t.Fatalf("sync must not replace an agent's value: %v", skips)
	}

	upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", RecordID: p.ID, Set: set("job_title", "Founder")})
	p, skips = upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", RecordID: p.ID, Set: set("job_title", "CEO")})
	if !slices.Equal(values(p, "job_title"), []string{"Founder"}) || len(skips) != 1 || skips[0].Source != records.SourceUser {
		t.Fatalf("an agent must not replace a person's value: %v %v", values(p, "job_title"), skips)
	}
	p, skips = upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", RecordID: p.ID, Remove: map[string][]string{"job_title": {}}})
	if !slices.Equal(values(p, "job_title"), []string{"Founder"}) || len(skips) != 1 {
		t.Fatalf("an agent must not remove a person's value: %v %v", values(p, "job_title"), skips)
	}

	// Asserting a synced value as an agent raises its rank, so sync can no longer remove it.
	upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", RecordID: p.ID, Set: set("email_addresses", "ada@example.com")})
	p, skips = upsert(t, svc, a, records.SourceSync, records.Write{Object: "people", RecordID: p.ID, Remove: map[string][]string{"email_addresses": {}}})
	if !slices.Equal(values(p, "email_addresses"), []string{"ada@example.com"}) || len(skips) != 1 {
		t.Fatalf("sync removed an agent's email: %v %v", values(p, "email_addresses"), skips)
	}
}

func TestMultiValues(t *testing.T) {
	svc, a, _ := setup(t)
	p, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Ada", "phone_numbers", "+44 7598 490355", "phone_numbers", "07598 490356")})
	p, _ = upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("phone_numbers", "+447598490355")})
	if got := values(p, "phone_numbers"); len(got) != 2 {
		t.Fatalf("the same number in another format matches, not adds: %v", got)
	}
	p, _ = upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", RecordID: p.ID, Remove: map[string][]string{"phone_numbers": {"+44 (7598) 490355"}}})
	if got := values(p, "phone_numbers"); !slices.Equal(got, []string{"07598 490356"}) {
		t.Fatalf("remove one: %v", got)
	}
	p, _ = upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", RecordID: p.ID, Remove: map[string][]string{"phone_numbers": {}}})
	if got := values(p, "phone_numbers"); len(got) != 0 {
		t.Fatalf("remove all: %v", got)
	}
}

func TestReferencesAndSearch(t *testing.T) {
	svc, a, b := setup(t)
	acme, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "companies", Set: set("name", "Acme", "domains", "https://www.Acme.com/about")})
	if got := values(acme, "domains"); !slices.Equal(got, []string{"acme.com"}) {
		t.Fatalf("domain: %v", got)
	}
	bob, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Bob Stone", "email_addresses", "bob@acme.com", "company", "acme.com")})
	company := bob.Fields[slices.IndexFunc(bob.Fields, func(f records.Field) bool { return f.Attribute == "company" })].Values[0]
	if company.RecordID != acme.ID || company.Text != "Acme" {
		t.Fatalf("company reference: %+v", company)
	}
	upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Ann Stone")})
	amol, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", Set: set("email_addresses", "amol@jcbl.com", "context", "- Buyer at Jcbl")})
	deal, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "deals", Set: set("name", "Jcbl pilot", "people", amol.ID)})
	if got := deal.Fields[slices.IndexFunc(deal.Fields, func(f records.Field) bool { return f.Attribute == "people" })].Values[0].Text; got != "amol@jcbl.com" {
		t.Fatalf("a nameless person must be named by their address, not their context: %q", got)
	}

	for name, q := range map[string]records.Search{
		"company by domain": {Object: "people", Where: map[string]string{"company": "ACME.com"}},
		"company by id":     {Object: "people", Where: map[string]string{"company": acme.ID}},
		"email":             {Object: "people", Where: map[string]string{"email_addresses": "BOB@acme.com"}},
		"name":              {Object: "people", Where: map[string]string{"name": "bob stone"}},
		"text and filter":   {Object: "people", Query: "stone", Where: map[string]string{"company": "acme.com"}},
	} {
		found, _, err := svc.Search(ctx, a, q)
		if err != nil || len(found) != 1 || found[0].ID != bob.ID {
			t.Errorf("%s: %v %v", name, found, err)
		}
	}
	if found, _, _ := svc.Search(ctx, a, records.Search{Object: "people", Query: "STONE"}); len(found) != 2 {
		t.Errorf("text search: %d records", len(found))
	}
	if found, _, err := svc.Search(ctx, a, records.Search{Object: "people", Where: map[string]string{"company": "unknown.com"}}); err != nil || len(found) != 0 {
		t.Errorf("unknown company: %v %v", found, err)
	}
	if found, _, _ := svc.Search(ctx, a, records.Search{Object: "people", Query: "%"}); len(found) != 0 {
		t.Errorf("a %% in the query must match literally: %d records", len(found))
	}

	if _, _, err := svc.Upsert(ctx, b, records.SourceAgent, records.Write{Object: "people", Set: set("name", "Eve", "company", acme.ID)}); err == nil {
		t.Fatal("referenced a company in another workspace")
	}
	if _, err := svc.Get(ctx, b, bob.ID); err == nil {
		t.Fatal("read a record in another workspace")
	}
	if _, _, err := svc.Upsert(ctx, b, records.SourceAgent, records.Write{Object: "people", RecordID: bob.ID, Set: set("name", "Eve")}); err == nil {
		t.Fatal("wrote a record in another workspace")
	}
	if found, _, _ := svc.Search(ctx, b, records.Search{Object: "people"}); len(found) != 0 {
		t.Fatalf("searched another workspace: %v", found)
	}
	other, _ := upsert(t, svc, b, records.SourceAgent, records.Write{Object: "people", Set: set("email_addresses", "bob@acme.com")})
	if other.ID == bob.ID {
		t.Fatal("unique values are per workspace")
	}
}

// Concurrent upserts of one new email end in one record.
func TestConcurrentUpsertsCreateOneRecord(t *testing.T) {
	svc, a, _ := setup(t)
	ids := make([]string, 8)
	var wg sync.WaitGroup
	// Open a connection per writer first, or the first writer finishes while
	// the others are still dialing and no two ever race.
	for range ids {
		wg.Go(func() {
			_, _, _ = svc.Search(ctx, a, records.Search{Object: "people"})
		})
	}
	wg.Wait()
	for i := range ids {
		wg.Go(func() {
			record, _, err := svc.Upsert(ctx, a, records.SourceAgent, records.Write{Object: "people", Set: set("email_addresses", "same@example.com")})
			if err != nil {
				t.Error(err)
			}
			ids[i] = record.ID
		})
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("records: %v", ids)
		}
	}
}

// Custom objects hold typed values and references; deleting a record drops
// the references to it.
func TestCustomObjects(t *testing.T) {
	svc, a, _ := setup(t)
	if _, err := svc.CreateObject(ctx, a, "quotes", "Quotes"); err != nil {
		t.Fatal(err)
	}
	for _, attr := range []records.Attribute{
		{Slug: "stage", Name: "Stage", Type: records.Select, Options: []string{"Lead", "Won"}},
		{Slug: "amount", Name: "Amount", Type: records.Number},
		{Slug: "closes", Name: "Closes", Type: records.Date},
		{Slug: "company", Name: "Company", Type: records.Reference, Target: "companies"},
	} {
		if _, err := svc.CreateAttribute(ctx, a, "quotes", attr); err != nil {
			t.Fatalf("%s: %v", attr.Slug, err)
		}
	}
	for name, bad := range map[string]records.Attribute{
		"unique select":         {Slug: "x", Name: "X", Type: records.Select, Options: []string{"a"}, Unique: true},
		"status without stages": {Slug: "x", Name: "X", Type: records.Status},
		"multi status":          {Slug: "x", Name: "X", Type: records.Status, Options: []string{"a"}, Multi: true},
		"unknown target":        {Slug: "x", Name: "X", Type: records.Reference, Target: "nope"},
		"taken slug":            {Slug: "amount", Name: "X", Type: records.Number},
		"bad slug":              {Slug: "Bad Slug", Name: "X", Type: records.Text},
	} {
		if _, err := svc.CreateAttribute(ctx, a, "quotes", bad); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	acme, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "companies", Set: set("name", "Acme", "domains", "acme.com")})
	quote, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "quotes", Set: set("name", "Press line", "stage", "won", "amount", "1,200.50", "closes", "2026-10-01T09:00:00Z", "company", "acme.com")})
	for attr, want := range map[string]string{"stage": "Won", "amount": "1200.5", "closes": "2026-10-01", "company": acme.ID} {
		if got := values(quote, attr); !slices.Equal(got, []string{want}) {
			t.Errorf("%s: %v", attr, got)
		}
	}
	if _, _, err := svc.Upsert(ctx, a, records.SourceAgent, records.Write{Object: "quotes", RecordID: quote.ID, Set: set("stage", "Lost")}); err == nil {
		t.Error("a stage outside the options was accepted")
	}
	if err := svc.Delete(ctx, a, acme.ID); err != nil {
		t.Fatal(err)
	}
	if quote, _ = svc.Get(ctx, a, quote.ID); len(values(quote, "company")) != 0 {
		t.Errorf("a deleted company is still referenced: %v", values(quote, "company"))
	}
}

// A new record starts in the first stage of a status it is not given and
// belongs to whoever creates it; a given stage is matched to its option, a
// later write keeps it, and an owner must be a member of the workspace.
func TestNewRecordDefaults(t *testing.T) {
	svc, a, _ := setup(t)
	lead, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "Press line")})
	if got := values(lead, "stage"); !slices.Equal(got, []string{"Lead"}) {
		t.Errorf("a new deal's stage: %v", got)
	}
	if got := values(lead, "owner"); !slices.Equal(got, []string{"a@jaz.test"}) {
		t.Errorf("a new deal's owner: %v", got)
	}
	if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: "deals", RecordID: lead.ID, Set: set("owner", "b@jaz.test")}); err == nil {
		t.Error("an owner from another workspace was accepted")
	}
	if got, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", RecordID: lead.ID, Set: set("owner", "A@Jaz.test")}); !slices.Equal(values(got, "owner"), []string{"a@jaz.test"}) {
		t.Errorf("an owner by email: %v", values(got, "owner"))
	}
	won, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "Dies", "stage", "won")})
	won, _ = upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", RecordID: won.ID, Set: set("name", "Dies for Acme")})
	if got := values(won, "stage"); !slices.Equal(got, []string{"Won"}) {
		t.Errorf("a given stage: %v", got)
	}
}

// History shows what changed, from which source and by whom: replacing a
// value is one change, removing one is another, and another workspace sees
// nothing.
func TestHistory(t *testing.T) {
	svc, a, b := setup(t)
	ada, _ := upsert(t, svc, a, records.SourceSync, records.Write{Object: "people", Set: set("name", "Ada", "email_addresses", "ada@example.com")})
	upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", RecordID: ada.ID, Set: set("name", "Ada Lovelace")})
	upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", RecordID: ada.ID, Remove: set("email_addresses", "ada@example.com")})
	changes, err := svc.History(ctx, a, ada.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range changes {
		got = append(got, fmt.Sprintf("%s %s=%s removed=%v actor=%v", c.Source, c.Attribute, c.Value.Text, c.Removed, c.Actor != ""))
	}
	want := []string{
		" email_addresses=ada@example.com removed=true actor=false",
		"user name=Ada Lovelace removed=false actor=true",
	}
	if len(got) != 5 || !slices.Equal(got[:2], want) || !slices.Contains(got, "sync name=Ada removed=false actor=true") || !slices.Contains(got, "sync owner=a@jaz.test removed=false actor=true") {
		t.Fatalf("history:\n%s", strings.Join(got, "\n"))
	}
	if _, err := svc.History(ctx, b, ada.ID); err == nil {
		t.Fatal("another workspace read the history")
	}
}
