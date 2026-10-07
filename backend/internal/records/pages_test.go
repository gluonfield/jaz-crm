package records_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// Pages nest under pages, never inside themselves, and people and agents
// edit each other's pages: titles, places and content.
func TestPages(t *testing.T) {
	svc, a, _ := setup(t)
	root, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Research", "icon", "📚")})
	child, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Painpoints", "parent", root.ID)})
	leaf, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Quoting", "parent", child.ID)})
	for _, parent := range []string{root.ID, leaf.ID} {
		if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: root.ID, Set: set("parent", parent)}); err == nil {
			t.Errorf("a page moved inside itself, under %s", parent)
		}
	}
	mention := fmt.Sprintf("Slow quotes at [Acme](/r/%s).\n\n- [ ] Ask about lead times", root.ID)
	moved, skips := upsert(t, svc, a, records.SourceAgent, records.Write{Object: records.Pages, RecordID: leaf.ID, Set: set("name", "Slow quoting", "parent", root.ID, "content", mention, "icon", "icon:target")})
	if len(skips) > 0 || !slices.Equal(values(moved, "name"), []string{"Slow quoting"}) || !slices.Equal(values(moved, "parent"), []string{root.ID}) || !slices.Equal(values(moved, "content"), []string{mention}) {
		t.Fatalf("an agent's edit of a person's page: %+v %+v", moved, skips)
	}
	found, _, err := svc.Search(ctx, a, records.Search{Object: records.Pages, Query: "lead times"})
	if err != nil || len(found) != 1 || found[0].ID != leaf.ID || len(values(found[0], "content")) != 0 || !slices.Equal(values(found[0], "icon"), []string{"icon:target"}) {
		t.Fatalf("searching content finds the page without carrying it: %+v %v", found, err)
	}
	upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: leaf.ID, Remove: map[string][]string{"icon": {}}})
	cleared, err := svc.Get(ctx, a, leaf.ID)
	if err != nil || len(values(cleared, "icon")) != 0 || !slices.Equal(values(cleared, "content"), []string{mention}) {
		t.Fatalf("removing a page icon: %+v %v", cleared, err)
	}
	if err := svc.Delete(ctx, a, root.ID); err != nil {
		t.Fatal(err)
	}
	if orphan, _ := svc.Get(ctx, a, child.ID); len(values(orphan, "parent")) != 0 {
		t.Errorf("a deleted page's sub-page still names it: %v", values(orphan, "parent"))
	}
}

// One member's edits to a document make one version until someone else
// writes it, and a write expecting content it has not seen fails.
func TestDocumentVersions(t *testing.T) {
	svc, a, _ := setup(t)
	page, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Notes", "content", "one"), Expect: map[string][]string{"content": {""}}})
	upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: page.ID, Set: set("content", "two"), Expect: map[string][]string{"content": {"one"}}})
	upsert(t, svc, a, records.SourceAgent, records.Write{Object: records.Pages, RecordID: page.ID, Set: set("content", "three")})
	if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: page.ID, Set: set("content", "stale"), Expect: map[string][]string{"content": {"two"}}}); err == nil {
		t.Fatal("a write overwrote content it had not seen")
	}
	changes, err := svc.History(ctx, a, page.ID)
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	for _, c := range changes {
		if c.Attribute == "content" {
			versions = append(versions, fmt.Sprintf("%s %s", c.Source, c.Value.Text))
		}
	}
	if !slices.Equal(versions, []string{"agent three", "user two"}) {
		t.Fatalf("content versions: %v", versions)
	}
}

// A workspace's tables are renamed and deleted with their columns; the CRM's
// own objects and attributes stay.
func TestEditTables(t *testing.T) {
	svc, a, _ := setup(t)
	pains, err := svc.CreateObject(ctx, a, "painpoints", "Painpoints")
	if err != nil {
		t.Fatal(err)
	}
	if pains.Standard || len(pains.Attributes) != 2 || pains.Attributes[1].Type != records.Markdown {
		t.Fatalf("a new table: %+v", pains)
	}
	if _, err := svc.CreateAttribute(ctx, a, "painpoints", records.Attribute{Slug: "notes", Name: "Notes", Type: records.Markdown}); err == nil {
		t.Error("a second document attribute was accepted")
	}
	for _, attr := range []records.Attribute{
		{Slug: "company", Name: "Company", Type: records.Reference, Target: "companies"},
		{Slug: "severity", Name: "Severity", Type: records.Select, Options: []string{"High", "Low"}},
	} {
		if _, err := svc.CreateAttribute(ctx, a, "painpoints", attr); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.CreateAttribute(ctx, a, "pages", records.Attribute{Slug: "pain", Name: "Pain", Type: records.Reference, Target: "painpoints"}); err != nil {
		t.Fatal(err)
	}
	acme, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "companies", Set: set("name", "Acme", "domains", "acme.com")})
	pain, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "painpoints", Set: set("name", "Slow quotes", "company", acme.ID, "severity", "High")})
	if _, err := svc.SaveFilter(ctx, a, "painpoints", storage.SavedFilter{Name: "Urgent", Filters: []storage.RecordFilter{
		{Attribute: "severity", Operator: "is", Value: "High"},
		{Attribute: "company", Operator: "is_not_empty"},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, refused := range [][2]string{{"painpoints", "name"}, {"painpoints", "content"}, {"people", "company"}, {"pages", "parent"}} {
		if err := svc.EditAttribute(ctx, a, refused[0], refused[1], "delete", ""); err == nil {
			t.Errorf("%s.%s was deleted", refused[0], refused[1])
		}
	}
	if err := svc.EditAttribute(ctx, a, "painpoints", "severity", "delete", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.EditAttribute(ctx, a, "painpoints", "company", "rename", "Customer"); err != nil {
		t.Fatal(err)
	}
	filters, err := svc.SavedFilters(ctx, a, "painpoints")
	if err != nil || len(filters) != 1 || len(filters[0].Filters) != 1 || filters[0].Filters[0].Attribute != "company" {
		t.Fatalf("a deleted column's filter conditions: %+v %v", filters, err)
	}
	if got, _ := svc.Get(ctx, a, pain.ID); len(values(got, "severity")) != 0 || !slices.Equal(values(got, "company"), []string{acme.ID}) {
		t.Fatalf("columns after the edits: %+v", got)
	}
	if _, err := svc.SaveFilter(ctx, a, "pages", storage.SavedFilter{Name: "Painful", Filters: []storage.RecordFilter{
		{Attribute: "pain", Operator: "is_not_empty"},
		{Attribute: "name", Operator: "contains", Value: "notes"},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, refused := range []string{"people", "pages"} {
		if err := svc.EditObject(ctx, a, refused, "delete", ""); err == nil {
			t.Errorf("%s was deleted", refused)
		}
	}
	if err := svc.EditObject(ctx, a, "painpoints", "rename", "Pains"); err != nil {
		t.Fatal(err)
	}
	if err := svc.EditObject(ctx, a, "painpoints", "delete", ""); err != nil {
		t.Fatal(err)
	}
	objects, err := svc.Objects(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range objects {
		if o.Slug == "painpoints" || o.Slug == "pages" && slices.ContainsFunc(o.Attributes, func(attr records.Attribute) bool { return attr.Slug == "pain" }) {
			t.Errorf("a deleted table or a column pointing at it remains: %+v", o)
		}
		if o.Standard != (o.Slug != "painpoints") {
			t.Errorf("%s standard: %v", o.Slug, o.Standard)
		}
	}
	if _, err := svc.Get(ctx, a, pain.ID); err == nil {
		t.Error("a deleted table's record remains")
	}
	if filters, err := svc.SavedFilters(ctx, a, "pages"); err != nil || len(filters[0].Filters) != 1 || filters[0].Filters[0].Attribute != "name" {
		t.Errorf("a filter on a column pointing at a deleted table: %+v %v", filters, err)
	}
}

// A status added to a table puts its existing records in the first stage,
// where new records start.
func TestNewStatusStartsRecords(t *testing.T) {
	svc, a, _ := setup(t)
	if _, err := svc.CreateObject(ctx, a, "problems", "Problems"); err != nil {
		t.Fatal(err)
	}
	row, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "problems", Set: set("name", "Untitled")})
	if _, err := svc.CreateAttribute(ctx, a, "problems", records.Attribute{Slug: "status", Name: "Status", Type: records.Status, Options: []string{"Not started", "Done"}}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, a, row.ID)
	if err != nil || !slices.Equal(values(got, "status"), []string{"Not started"}) {
		t.Fatalf("an existing record's stage: %v %v", values(got, "status"), err)
	}
	upsert(t, svc, a, records.SourceUser, records.Write{Object: "problems", RecordID: row.ID, Set: set("status", "Done")})
}
