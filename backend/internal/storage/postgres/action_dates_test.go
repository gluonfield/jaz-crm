package postgres_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

func TestActionDateMigration(t *testing.T) {
	ctx := context.Background()
	db, dsn := legacy(t, 31)
	var workspace, object string
	if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('Existing') RETURNING id").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,'follow_ups','Follow-ups') RETURNING id", workspace).Scan(&object); err != nil {
		t.Fatal(err)
	}
	for _, attr := range []struct{ slug, kind string }{{"name", "text"}, {"review_on", "date"}, {"status", "status"}, {"waiting_on", "select"}, {"draft", "text"}, {"draft_status", "select"}} {
		if _, err := db.ExecContext(ctx, "INSERT INTO attributes(object_id,slug,name,type) VALUES($1,$2,$2,$3)", object, attr.slug, attr.kind); err != nil {
			t.Fatal(err)
		}
	}
	var ids []string
	for _, seed := range []struct{ source, status, draftStatus string }{{"agent", "Open", "Draft"}, {"user", "Open", "Draft"}, {"agent", "Done", "Draft"}, {"agent", "Open", "Sending"}} {
		var id string
		if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id) VALUES($1,$2) RETURNING id", workspace, object).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		for slug, value := range map[string]string{"name": seed.source + seed.status, "review_on": "2026-10-19", "status": seed.status, "waiting_on": "Them", "draft": "Preserved in history", "draft_status": seed.draftStatus} {
			if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id,attribute_id,text,source) SELECT $1,id,$3,$4 FROM attributes WHERE object_id=$2 AND slug=$5", id, object, value, seed.source, slug); err != nil {
				t.Fatal(err)
			}
		}
	}
	filterID := shortuuid.New()
	filter := `[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is","value":"Them"},{"attribute":"review_on","operator":"before","value":"today"}]`
	if _, err := db.ExecContext(ctx, "INSERT INTO saved_filters(id,workspace_id,object_id,name,filters) VALUES($1,$2,$3,'Chase',$4::jsonb)", filterID, workspace, object, filter); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO active_filters(object_id,filters,saved_id) VALUES($1,$2::jsonb,$3)", object, filter, filterID); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc := records.NewService(store)
	actor := auth.Actor{WorkspaceID: workspace}
	for i, id := range ids {
		r, err := svc.Get(ctx, actor, id)
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]string{}
		for _, f := range r.Fields {
			if len(f.Values) > 0 {
				values[f.Attribute] = f.Values[0].Text
			}
		}
		if i == 2 {
			if values["action_date"] != "" || values["action_date_basis"] != "" {
				t.Fatalf("closed action kept its active date: %v", values)
			}
		} else if values["action_date"] != "2026-10-19" || values["review_on"] != "" {
			t.Fatalf("migration lost the original date: %v", values)
		}
		if i == 0 && (values["action_date_basis"] != "Suggested" || !strings.Contains(values["action_date_reason"], "not recorded")) || i == 1 && values["action_date_basis"] != "Manual" {
			t.Fatalf("legacy date provenance: %v", values)
		}
		if (i == 0 || i == 2) && (values["draft"] != "" || values["draft_status"] != "") || (i == 1 || i == 3) && values["draft"] == "" {
			t.Fatalf("migration must withdraw stale AI drafts and preserve human/sending drafts: %d %v", i, values)
		}
	}
	filters, err := svc.SavedFilters(ctx, actor, records.FollowUps)
	if err != nil || len(filters) != 1 || filters[0].ID != filterID || filters[0].Filters[2].Attribute != "action_date" || filters[0].Filters[2].Value != "now" {
		t.Fatalf("existing Chase preset identity and deadline semantics: %+v %v", filters, err)
	}
	var originalDates int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM record_values v JOIN attributes a ON a.id=v.attribute_id WHERE a.object_id=$1 AND a.slug='action_date' AND v.text='2026-10-19'", object).Scan(&originalDates); err != nil || originalDates != 4 {
		t.Fatalf("migration must retain original dates in history: %d %v", originalDates, err)
	}
}

func TestNeedsAttentionMigration(t *testing.T) {
	ctx := context.Background()
	db, dsn := legacy(t, 33)
	old := `[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"action_date","operator":"on_or_before","value":"today"}]`
	custom := `[{"attribute":"status","operator":"is","value":"Open"}]`
	assigned := `[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is","value":"Us"}]`
	updated := `[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is_not","value":"Them"}]`
	type fixture struct {
		name, slug, query, saved, wantSaved string
		workspace, object, id               string
	}
	fixtures := []fixture{
		{name: "Needs attention", slug: "follow_ups", saved: old, wantSaved: updated},
		{name: "Needs attention", slug: "follow_ups", saved: custom, wantSaved: custom},
		{name: "Needs attention", slug: "follow_ups", query: "Customer", saved: old, wantSaved: old},
		{name: "My deadlines", slug: "follow_ups", saved: old, wantSaved: old},
		{name: "Needs attention", slug: "deals", saved: old, wantSaved: old},
		{name: "Needs attention", slug: "follow_ups", saved: assigned, wantSaved: updated},
		{name: "Needs attention", slug: "follow_ups", query: "Customer", saved: assigned, wantSaved: assigned},
	}
	for i := range fixtures {
		f := &fixtures[i]
		f.id = shortuuid.New()
		if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('Existing') RETURNING id").Scan(&f.workspace); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,$2,'Actions') RETURNING id", f.workspace, f.slug).Scan(&f.object); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO saved_filters(id,workspace_id,object_id,name,query,filters) VALUES($1,$2,$3,$4,$5,$6::jsonb)", f.id, f.workspace, f.object, f.name, f.query, f.saved); err != nil {
			t.Fatal(err)
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for i, f := range fixtures {
		filters, err := store.SavedFilters(ctx, f.workspace, f.object)
		if err != nil || len(filters) != 1 || filters[0].ID != f.id || filters[0].Query != f.query {
			t.Fatalf("case %d lost saved filter identity or query: %+v %v", i, filters, err)
		}
		var want []storage.RecordFilter
		if err := json.Unmarshal([]byte(f.wantSaved), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(filters[0].Filters, want) {
			t.Fatalf("case %d migration: got %+v, want %+v", i, filters[0].Filters, want)
		}
	}
}
