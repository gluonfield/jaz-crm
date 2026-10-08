package records_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func TestArchivedAttributesRetainValuesAndFilters(t *testing.T) {
	svc, actor, other := setup(t)
	for _, owner := range []auth.Actor{actor, other} {
		if _, err := svc.CreateAttribute(ctx, owner, "people", records.Attribute{Slug: "legacy", Name: "Legacy outreach", Type: records.Text}); err != nil {
			t.Fatal(err)
		}
	}
	person, _ := upsert(t, svc, actor, records.SourceSync, records.Write{Object: "people", Set: set("name", "Ada", "legacy", "First")})
	person, _ = upsert(t, svc, actor, records.SourceUser, records.Write{Object: "people", RecordID: person.ID, Set: set("legacy", "Hiddenneedle")})
	peer, _ := upsert(t, svc, other, records.SourceUser, records.Write{Object: "people", Set: set("name", "Other Ada", "legacy", "Hiddenneedle")})
	for _, channel := range []string{"Email", "LinkedIn"} {
		upsert(t, svc, actor, records.SourceUser, records.Write{Object: "follow_ups", Set: set("name", channel+" outreach", "person", person.ID, "channel", channel)})
	}
	filter, err := svc.SaveFilter(ctx, actor, "people", storage.SavedFilter{Name: "Legacy", Filters: []storage.RecordFilter{{Attribute: "legacy", Operator: "is", Value: "Hiddenneedle"}}})
	if err != nil {
		t.Fatal(err)
	}
	history, err := svc.History(ctx, actor, person.ID)
	if err != nil || len(history) < 3 {
		t.Fatalf("populated history: %+v %v", history, err)
	}
	if err := svc.EditAttribute(ctx, actor, "people", "legacy", "archive", ""); err != nil {
		t.Fatal(err)
	}
	for _, include := range []bool{false, true} {
		objects, err := svc.Objects(ctx, actor, include)
		if err != nil {
			t.Fatal(err)
		}
		people := objects[slices.IndexFunc(objects, func(object records.Object) bool { return object.Slug == "people" })]
		index := slices.IndexFunc(people.Attributes, func(attribute records.Attribute) bool { return attribute.Slug == "legacy" })
		if (index >= 0) != include || (include && !people.Attributes[index].Archived) {
			t.Fatalf("archived schema include=%v: %+v", include, people.Attributes)
		}
	}
	got, err := svc.Get(ctx, actor, person.ID)
	if err != nil || len(values(got, "legacy")) != 0 {
		t.Fatalf("active record exposed archived values: %+v %v", got, err)
	}
	if found, _, err := svc.Search(ctx, actor, records.Search{Object: "people", Query: "Hiddenneedle"}); err != nil || len(found) != 0 {
		t.Fatalf("text search matched archived values: %+v %v", found, err)
	}
	for _, query := range []records.Search{
		{Object: "people", Where: map[string]string{"legacy": "Hiddenneedle"}},
		{Object: "people", Sort: "legacy"},
	} {
		if _, _, err := svc.Search(ctx, actor, query); err == nil || !strings.Contains(err.Error(), "archived") {
			t.Fatalf("archived search must explain restoration: %v", err)
		}
	}
	for _, write := range []records.Write{
		{Object: "people", RecordID: person.ID, Set: set("legacy", "Replacement")},
		{Object: "people", RecordID: person.ID, Remove: map[string][]string{"legacy": {}}},
	} {
		if _, _, err := svc.Upsert(ctx, actor, records.SourceUser, write); err == nil || !strings.Contains(err.Error(), "archived") {
			t.Fatalf("archived write must explain restoration: %v", err)
		}
	}
	if filters, err := svc.SavedFilters(ctx, actor, "people"); err != nil || len(filters) != 0 {
		t.Fatalf("a filter must never widen when its property is archived: %+v %v", filters, err)
	}
	upsert(t, svc, actor, records.SourceUser, records.Write{Object: "people", RecordID: person.ID, Set: set("job_title", "Engineer")})
	if got, err := svc.Get(ctx, other, peer.ID); err != nil || !slices.Equal(values(got, "legacy"), []string{"Hiddenneedle"}) {
		t.Fatalf("archiving crossed workspaces: %+v %v", got, err)
	}
	if found, _, err := svc.Search(ctx, actor, records.Search{Object: "follow_ups", Where: map[string]string{"person": person.ID}}); err != nil || len(found) != 2 || values(found[0], "channel")[0] == values(found[1], "channel")[0] {
		t.Fatalf("individual outreach channels changed: %+v %v", found, err)
	}
	for _, attribute := range []string{"name", "email_addresses", "context"} {
		if err := svc.EditAttribute(ctx, actor, "people", attribute, "archive", ""); err == nil {
			t.Fatalf("archived built-in %s", attribute)
		}
	}
	if err := svc.EditAttribute(ctx, actor, "people", "legacy", "restore", ""); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Get(ctx, actor, person.ID)
	if err != nil || !slices.Equal(values(got, "legacy"), []string{"Hiddenneedle"}) || !slices.Equal(values(got, "job_title"), []string{"Engineer"}) {
		t.Fatalf("restore lost values or intervening edits: %+v %v", got, err)
	}
	restored, err := svc.History(ctx, actor, person.ID)
	if err != nil || !reflect.DeepEqual(restored[1:], history) {
		t.Fatalf("restore changed history: %+v %v", restored, err)
	}
	if filters, err := svc.SavedFilters(ctx, actor, "people"); err != nil || len(filters) != 1 || !reflect.DeepEqual(filters[0], filter) {
		t.Fatalf("restore lost the original preset: %+v %v", filters, err)
	}
	if found, _, err := svc.Search(ctx, actor, records.Search{Object: "people", Query: "Hiddenneedle"}); err != nil || len(found) != 1 || found[0].ID != person.ID {
		t.Fatalf("restored text search: %+v %v", found, err)
	}
	upsert(t, svc, actor, records.SourceUser, records.Write{Object: "people", RecordID: person.ID, Set: set("legacy", "Restored")})
}
