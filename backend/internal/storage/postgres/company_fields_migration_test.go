package postgres_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

func TestCASCompanyFieldsMigrationPreservesInformationAndOtherWorkspaces(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 54)
	const cas = "cf52341c-db12-46a3-a60e-4060c0380200"
	other := shortuuid.New()
	at := time.Date(2026, 10, 1, 12, 30, 0, 0, time.UTC)
	type fixture struct {
		workspace string
		object    string
		user      string
		attrs     map[string]string
		records   []string
	}
	var fixtures []fixture
	for _, workspace := range []string{cas, other} {
		f := fixture{workspace: workspace, object: shortuuid.New(), user: shortuuid.New(), attrs: map[string]string{}}
		if _, err := db.ExecContext(ctx, "INSERT INTO workspaces(id, name) VALUES ($1, 'CAS')", workspace); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO users(id, workspace_id, name, email) VALUES ($1, $2, 'Owner', $3)", f.user, workspace, f.user+"@jaz.test"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO objects(id, workspace_id, slug, name) VALUES ($1, $2, 'companies', 'Companies')", f.object, workspace); err != nil {
			t.Fatal(err)
		}
		for _, slug := range []string{"name", "description", "primary_activities", "key_products", "categories", "manufacturer", "outreach_notes", "fit_profile", "last_activity_date"} {
			id := shortuuid.New()
			f.attrs[slug] = id
			kind := "text"
			if slug == "categories" || slug == "manufacturer" || slug == "fit_profile" {
				kind = "select"
			}
			if slug == "last_activity_date" {
				kind = "date"
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO attributes(id, object_id, slug, name, type, multi, options) VALUES ($1, $2, $3, $3, $4, $5, $6)", id, f.object, slug, kind, slug == "categories", []string{"Existing"}); err != nil {
				t.Fatal(err)
			}
		}
		for i, fields := range []map[string][]string{
			{"description": {"Fabrication"}, "primary_activities": {"FABRICATION"}, "key_products": {"Brackets\nhttps://example.test/source"}, "categories": {"Existing"}, "manufacturer": {"Yes"}, "outreach_notes": {"Asked for a demo.\nSource: https://example.test/meeting"}, "fit_profile": {"Legacy fit"}, "last_activity_date": {"2026-01-01"}},
			{"primary_activities": {"Assembly"}, "manufacturer": {"No"}, "outreach_notes": {"Leadership search completed — 0 contacts found via Exa on 2026-04-22<br>Leadership re-search completed — 1 contact(s) found via Exa on 2026-04-23"}},
			{"description": {"Existing"}, "categories": {"Manufacturer"}, "manufacturer": {"Yes"}, "outreach_notes": {"Leadership search completed — 0 contacts found via Exa on 2026-04-22\nIntroduced by Ada."}},
		} {
			id := shortuuid.New()
			f.records = append(f.records, id)
			if _, err := db.ExecContext(ctx, "INSERT INTO records(id, workspace_id, object_id) VALUES ($1, $2, $3)", id, workspace, f.object); err != nil {
				t.Fatal(err)
			}
			fields["name"] = []string{"Company"}
			for slug, values := range fields {
				for _, value := range values {
					if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id, attribute_id, text, source, actor_id, active_from) VALUES ($1, $2, $3, 'user', $4, $5)", id, f.attrs[slug], value, f.user, at); err != nil {
						t.Fatal(err)
					}
				}
			}
			if i == 0 {
				if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id, attribute_id, text, source, active_from, active_until) VALUES ($1, $2, 'Earlier description', 'agent', $3, $4)", id, f.attrs["description"], at.Add(-time.Hour), at); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.ExecContext(ctx, "UPDATE records SET updated_at = $2 WHERE id = $1", id, at); err != nil {
				t.Fatal(err)
			}
		}
		fixtures = append(fixtures, f)
	}
	var before string
	if err := db.QueryRowContext(ctx, "SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id)::text FROM record_values v JOIN records r ON r.id = v.record_id WHERE r.workspace_id = $1", other).Scan(&before); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	crm := records.NewService(store)
	actor := auth.Actor{WorkspaceID: cas, UserID: fixtures[0].user}
	svc := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	for i, id := range fixtures[0].records {
		got, err := crm.Get(ctx, actor, id)
		if err != nil {
			t.Fatal(err)
		}
		if !got.UpdatedAt.Equal(at) {
			t.Fatalf("data relocation changed last updated: %s", got.UpdatedAt)
		}
		values := map[string][]string{}
		for _, field := range got.Fields {
			for _, v := range field.Values {
				values[field.Attribute] = append(values[field.Attribute], v.Text)
			}
		}
		for _, slug := range []string{"primary_activities", "key_products", "outreach_notes", "manufacturer", "fit_profile", "last_activity_date"} {
			if values[slug] != nil {
				t.Fatalf("retired property still visible: %s", slug)
			}
		}
		descriptions := []string{"Fabrication\n\nBrackets\nhttps://example.test/source", "Assembly", "Existing"}
		if !slices.Equal(values["description"], []string{descriptions[i]}) {
			t.Fatalf("description lost or repeated information: %+v", values)
		}
		if (i != 1) != slices.Contains(values["categories"], "Manufacturer") || (i == 0 && !slices.Contains(values["categories"], "Existing")) || (i == 2 && len(values["categories"]) != 1) {
			t.Fatalf("manufacturer/category conversion: %+v", values["categories"])
		}
		timeline, err := svc.Timeline(ctx, actor, id, []string{"note"}, "", false, 20)
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if len(timeline) != 0 {
				t.Fatalf("import bookkeeping became visible notes: %+v", timeline)
			}
		} else {
			if len(timeline) != 1 {
				t.Fatalf("outreach note missing or duplicated: %+v", timeline)
			}
			note, err := svc.Get(ctx, actor, timeline[0].ID)
			if err != nil || note.StartedAt != at.Format(time.RFC3339) || !strings.Contains(note.Text, []string{"Asked for a demo.\nSource: https://example.test/meeting", "", "Introduced by Ada."}[i]) {
				t.Fatalf("outreach note lost its content/date: %+v %v", note, err)
			}
		}
	}
	activity, err := svc.Activities(ctx, actor, fixtures[0].records)
	if err != nil || len(activity) != 0 {
		t.Fatalf("relocated notes counted as contact: %+v %v", activity, err)
	}
	var after string
	if err := db.QueryRowContext(ctx, "SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id)::text FROM record_values v JOIN records r ON r.id = v.record_id WHERE r.workspace_id = $1", other).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("migration changed another workspace's values")
	}
	var archived int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM attributes WHERE object_id = $1 AND archived", fixtures[1].object).Scan(&archived); err != nil || archived != 0 {
		t.Fatalf("migration changed another workspace's schema: %d %v", archived, err)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: "companies", RecordID: fixtures[0].records[0], Set: map[string][]string{"manufacturer": {"No"}}}); err == nil {
		t.Fatal("agent can still populate an archived field")
	}
	if err := crm.EditAttribute(ctx, actor, "companies", "outreach_notes", "restore", ""); err != nil {
		t.Fatal(err)
	}
	restored, err := crm.Get(ctx, actor, fixtures[0].records[0])
	if err != nil {
		t.Fatal(err)
	}
	found := slices.ContainsFunc(restored.Fields, func(f records.Field) bool {
		return f.Attribute == "outreach_notes" && len(f.Values) == 1 && f.Values[0].Text == "Asked for a demo.\nSource: https://example.test/meeting"
	})
	if !found {
		t.Fatal("archived source values were lost")
	}
	var history int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM record_values WHERE record_id = $1 AND attribute_id = $2", fixtures[0].records[0], fixtures[0].attrs["description"]).Scan(&history); err != nil || history != 3 {
		t.Fatalf("description history lost: %d %v", history, err)
	}
}
