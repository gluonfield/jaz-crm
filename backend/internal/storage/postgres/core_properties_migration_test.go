package postgres_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

func TestCorePropertiesMigrationAdoptsExistingData(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 46)
	workspace, object, record := shortuuid.New(), shortuuid.New(), shortuuid.New()
	personObject, dealObject, user := shortuuid.New(), shortuuid.New(), shortuuid.New()
	city, industry, custom := shortuuid.New(), shortuuid.New(), shortuuid.New()
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces(id, name) VALUES ($1, 'CAS')`, []any{workspace}},
		{`INSERT INTO users(id, workspace_id, name, email) VALUES ($1, $2, 'Owner', 'owner@jaz.test')`, []any{user, workspace}},
		{`INSERT INTO objects(id, workspace_id, slug, name) VALUES ($1, $2, 'companies', 'Companies')`, []any{object, workspace}},
		{`INSERT INTO objects(id, workspace_id, slug, name) VALUES ($1, $2, 'people', 'People'), ($3, $2, 'deals', 'Deals')`, []any{personObject, workspace, dealObject}},
		{`INSERT INTO attributes(id, object_id, slug, name, type) VALUES ($1, $2, 'name', 'Name', 'text'), ($3, $4, 'name', 'Name', 'text')`, []any{shortuuid.New(), personObject, shortuuid.New(), dealObject}},
		{`INSERT INTO attributes(id, object_id, slug, name, type, archived) VALUES ($1, $2, 'hq_city', 'Team city', 'text', true)`, []any{city, object}},
		{`INSERT INTO attributes(id, object_id, slug, name, type, options) VALUES ($1, $2, 'industry', 'Sector', 'select', $3)`, []any{industry, object, []string{"Fabrication", "Software"}}},
		{`INSERT INTO attributes(id, object_id, slug, name, type, archived) VALUES ($1, $2, 'capacity', 'Capacity', 'number', true)`, []any{custom, object}},
		{`INSERT INTO records(id, workspace_id, object_id) VALUES ($1, $2, $3)`, []any{record, workspace, object}},
		{`INSERT INTO record_values(record_id, attribute_id, text, source, active_from, active_until) VALUES ($1, $2, 'Old city', 'user', now() - interval '2 hours', now() - interval '1 hour')`, []any{record, city}},
		{`INSERT INTO record_values(record_id, attribute_id, text, source) VALUES ($1, $2, 'Current city', 'agent'), ($1, $3, 'Fabrication', 'user')`, []any{record, city, industry}},
		{`INSERT INTO saved_filters(id, workspace_id, object_id, name, filters) VALUES ($1, $2, $3, 'City view', '[{"attribute":"hq_city","operator":"contains","value":"Current"}]')`, []any{shortuuid.New(), workspace, object}},
	} {
		if _, err := db.ExecContext(ctx, seed.query, seed.args...); err != nil {
			t.Fatal(err)
		}
	}
	var before string
	if err := db.QueryRowContext(ctx, `SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id)::text FROM record_values v`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var after, cityID, industryID, industryName string
	var rawOptions []byte
	var options []string
	var customArchived bool
	if err := db.QueryRowContext(ctx, `SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id)::text FROM record_values v`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("migration rewrote current values or history")
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM attributes WHERE object_id = $1 AND slug = 'hq_city' AND NOT archived`, object).Scan(&cityID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id, name, array_to_json(options) FROM attributes WHERE object_id = $1 AND slug = 'industry'`, object).Scan(&industryID, &industryName, &rawOptions); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawOptions, &options); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT archived FROM attributes WHERE id = $1`, custom).Scan(&customArchived); err != nil {
		t.Fatal(err)
	}
	if cityID != city || industryID != industry || industryName != "Sector" || !slices.Equal(options, []string{"Fabrication", "Software"}) || !customArchived {
		t.Fatalf("adoption changed identities/options/custom archive: %s %s %q %v %v", cityID, industryID, industryName, options, customArchived)
	}
	crm := records.NewService(store)
	actor := auth.Actor{WorkspaceID: workspace, UserID: user}
	got, err := crm.Get(ctx, actor, record)
	if err != nil {
		t.Fatal(err)
	}
	foundCity := false
	for _, field := range got.Fields {
		if field.Attribute == "owner" {
			t.Fatal("migration assigned an owner to an existing record")
		}
		if field.Attribute == "hq_city" {
			foundCity = len(field.Values) == 1 && field.Values[0].Text == "Current city"
		}
	}
	filters, err := crm.SavedFilters(ctx, actor, "companies")
	if err != nil || !foundCity || len(filters) != 1 || filters[0].Name != "City view" {
		t.Fatalf("adopted field is unreadable or lost its saved view: %v %+v %v", foundCity, filters, err)
	}
	if _, err := crm.AddOption(ctx, actor, "companies", "hq_country", "United Kingdom"); err != nil {
		t.Fatal(err)
	}
	for _, write := range []records.Write{
		{Object: "companies", RecordID: record, Set: map[string][]string{
			"website": {"https://acme.test"}, "links": {"https://linkedin.com/company/acme"}, "industry": {"Fabrication"},
			"hq_city": {"Cambridge"}, "hq_state": {"Cambridgeshire"}, "hq_country": {"United Kingdom"}, "employee_count": {"42"},
			"owner": {"owner@jaz.test"},
		}},
		{Object: "people", Set: map[string][]string{"name": {"Ada"}, "links": {"https://linkedin.com/in/ada"}, "owner": {"owner@jaz.test"}}},
		{Object: "deals", Set: map[string][]string{"name": {"Project"}, "expected_close_date": {"2026-11-30"}}},
	} {
		written, _, err := crm.Upsert(ctx, actor, records.SourceUser, write)
		if err != nil {
			t.Fatal(err)
		}
		for attribute, want := range write.Set {
			i := slices.IndexFunc(written.Fields, func(f records.Field) bool { return f.Attribute == attribute })
			if i < 0 || len(written.Fields[i].Values) != 1 || written.Fields[i].Values[0].Text != want[0] {
				t.Fatalf("migrated %s.%s could not be written: %+v", written.Object, attribute, written.Fields)
			}
		}
	}
	if err := crm.EditAttribute(ctx, actor, "companies", "hq_city", "archive", ""); err == nil {
		t.Fatal("adopted core property could still be archived")
	}
}

func TestCorePropertiesMigrationRollsBackIncompatibleData(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 46)
	workspace, object := shortuuid.New(), shortuuid.New()
	if _, err := db.ExecContext(ctx, `INSERT INTO workspaces(id, name) VALUES ($1, 'CAS')`, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO objects(id, workspace_id, slug, name) VALUES ($1, $2, 'people', 'People')`, object, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO attributes(id, object_id, slug, name, type, archived) VALUES ($1, $2, 'owner', 'Owner', 'number', true)`, shortuuid.New(), object); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err == nil {
		store.Close()
		t.Fatal("migration silently adopted an incompatible core property")
	}
	if !strings.Contains(err.Error(), "people.owner") || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("migration omitted the conflicting property: %v", err)
	}
	var count int
	var kind string
	var archived bool
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attributes WHERE object_id = $1`, object).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT type, archived FROM attributes WHERE object_id = $1 AND slug = 'owner'`, object).Scan(&kind, &archived); err != nil {
		t.Fatal(err)
	}
	if count != 1 || kind != "number" || !archived {
		t.Fatalf("failed migration left partial writes: %d %s %v", count, kind, archived)
	}
	if _, err := db.ExecContext(ctx, `UPDATE attributes SET type = 'member' WHERE object_id = $1 AND slug = 'owner'`, object); err != nil {
		t.Fatal(err)
	}
	store, err = postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("migration did not recover after explicit schema mapping: %v", err)
	}
	store.Close()
}
