package records_test

import (
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

func TestRecordUpdatedAtTracksChanges(t *testing.T) {
	svc, a, _ := setup(t)
	person, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Ada")})
	if person.UpdatedAt.IsZero() || person.UpdatedAt.Before(person.CreatedAt) {
		t.Fatalf("new record timestamps: %+v", person)
	}
	unchanged, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", RecordID: person.ID, Set: set("name", "Ada")})
	blocked, skips := upsert(t, svc, a, records.SourceSync, records.Write{Object: "people", RecordID: person.ID, Set: set("name", "A different name")})
	if !unchanged.UpdatedAt.Equal(person.UpdatedAt) || !blocked.UpdatedAt.Equal(person.UpdatedAt) || len(skips) != 1 {
		t.Fatal("identical or blocked writes changed updated_at")
	}
	changed, _ := upsert(t, svc, a, records.SourceAgent, records.Write{Object: "people", RecordID: person.ID, Set: set("job_title", "Engineer")})
	if !changed.UpdatedAt.After(person.UpdatedAt) || !changed.CreatedAt.Equal(person.CreatedAt) {
		t.Fatalf("field change timestamps: %+v", changed)
	}
	removed, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", RecordID: person.ID, Remove: map[string][]string{"job_title": {}}})
	if !removed.UpdatedAt.After(changed.UpdatedAt) {
		t.Fatal("removing a field did not update the timestamp")
	}
	page, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Notes", "content", "First draft")})
	edited, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: page.ID, Set: set("content", "Second draft")})
	if !edited.UpdatedAt.After(page.UpdatedAt) {
		t.Fatal("a coalesced document revision did not update the timestamp")
	}
	history, err := svc.History(ctx, a, page.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count := len(slices.DeleteFunc(history, func(c records.Change) bool { return c.Attribute != "content" })); count != 1 {
		t.Fatalf("expected one coalesced document version, got %d", count)
	}
	if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: page.ID, Set: set("content", "Stale overwrite"), Expect: set("content", "First draft")}); err == nil {
		t.Fatal("stale write succeeded")
	}
	afterFailure, err := svc.Get(ctx, a, page.ID)
	if err != nil || !afterFailure.UpdatedAt.Equal(edited.UpdatedAt) {
		t.Fatalf("failed write changed the timestamp: %+v %v", afterFailure, err)
	}
}

func TestSearchByLastUpdated(t *testing.T) {
	svc, a, _ := setup(t)
	older, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Older")})
	newer, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Newer")})
	upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", RecordID: older.ID, Set: set("job_title", "Updated")})
	for offset, want := range []string{older.ID, newer.ID} {
		found, total, err := svc.Search(ctx, a, records.Search{Object: "people", Sort: "updated_at", Offset: offset, Limit: 1})
		if err != nil || total != 2 || len(found) != 1 || found[0].ID != want || found[0].UpdatedAt.IsZero() {
			t.Fatalf("last-updated page %d: %+v total %d error %v", offset, found, total, err)
		}
	}
	defaultOrder, _, err := svc.Search(ctx, a, records.Search{Object: "people"})
	if err != nil || len(defaultOrder) != 2 || defaultOrder[0].ID != newer.ID {
		t.Fatalf("default creation order changed: %+v %v", defaultOrder, err)
	}
}
