package postgres_test

import (
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

// LinkedIn becomes Links in place, keeping values and history; X values join
// them; links then identify people and their chat handles.
func TestLinksMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 50)
	workspace, user, people, companies := shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New()
	linkedin, x, companyLinkedIn, jim, ann, acme := shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New()
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces(id, name) VALUES ($1, 'CAS')`, []any{workspace}},
		{`INSERT INTO users(id, workspace_id, name, email) VALUES ($1, $2, 'Owner', 'owner@cas.dev')`, []any{user, workspace}},
		{`INSERT INTO objects(id, workspace_id, slug, name) VALUES ($1, $3, 'people', 'People'), ($2, $3, 'companies', 'Companies')`, []any{people, companies, workspace}},
		{`INSERT INTO attributes(id, object_id, slug, name, type, is_unique) VALUES ($1, $4, 'linkedin_url', 'LinkedIn', 'url', true), ($2, $4, 'x_url', 'X', 'url', false), ($3, $5, 'linkedin_url', 'LinkedIn', 'url', false)`, []any{linkedin, x, companyLinkedIn, people, companies}},
		{`INSERT INTO records(id, workspace_id, object_id) VALUES ($1, $4, $5), ($2, $4, $5), ($3, $4, $6)`, []any{jim, ann, acme, workspace, people, companies}},
		{`INSERT INTO record_values(record_id, attribute_id, text, unique_key, source, active_from, active_until) VALUES
			($1, $2, 'https://linkedin.com/in/old-jim', 'https://linkedin.com/in/old-jim', 'user', now() - interval '2 days', now() - interval '1 day'),
			($1, $2, 'https://uk.linkedin.com/in/Jim/', 'https://uk.linkedin.com/in/jim/', 'user', now() - interval '1 day', NULL)`, []any{jim, linkedin}},
		{`INSERT INTO record_values(record_id, attribute_id, text, source) VALUES ($1, $2, 'https://twitter.com/Ann', 'agent'), ($3, $4, 'https://www.linkedin.com/company/acme', 'user')`, []any{ann, x, acme, companyLinkedIn}},
		{`INSERT INTO saved_filters(id, workspace_id, object_id, name, filters) VALUES ($1, $2, $3, 'On LinkedIn', '[{"attribute":"linkedin_url","operator":"is_not_empty"},{"attribute":"name","operator":"is_not_empty"}]')`, []any{shortuuid.New(), workspace, people}},
		{`INSERT INTO handles(id, workspace_id, kind, value, triage) VALUES ($1, $3, 'linkedin', 'linkedin.com/in/jim', 'pending'), ($2, $3, 'x', 'x.com/ann', 'pending')`, []any{shortuuid.New(), shortuuid.New(), workspace}},
		{`UPDATE records SET updated_at = '2026-01-01'`, nil},
	} {
		if _, err := db.ExecContext(ctx, seed.query, seed.args...); err != nil {
			t.Fatal(err)
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var touched int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM records WHERE updated_at <> '2026-01-01'`).Scan(&touched); err != nil || touched != 0 {
		t.Fatalf("moving links changed %d records' last update: %v", touched, err)
	}
	var slugs []string
	rows, err := db.QueryContext(ctx, `SELECT a.id || ':' || o.slug || '.' || a.slug || ':' || a.name || ':' || a.multi || ':' || a.is_unique FROM attributes a JOIN objects o ON o.id = a.object_id ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			t.Fatal(err)
		}
		slugs = append(slugs, slug)
	}
	want := []string{companyLinkedIn + ":companies.links:Links:true:true", linkedin + ":people.links:Links:true:true"}
	slices.Sort(want)
	if !slices.Equal(slugs, want) {
		t.Fatalf("LinkedIn fields become Links in place and X goes: %v", slugs)
	}
	var history int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM record_values WHERE record_id = $1 AND attribute_id = $2`, jim, linkedin).Scan(&history); err != nil || history != 2 {
		t.Fatalf("Jim's LinkedIn history: %d %v", history, err)
	}
	var filters string
	if err := db.QueryRowContext(ctx, `SELECT filters::text FROM saved_filters WHERE object_id = $1`, people).Scan(&filters); err != nil || filters != `[{"operator": "is_not_empty", "attribute": "links"}, {"operator": "is_not_empty", "attribute": "name"}]` {
		t.Fatalf("saved filter: %s %v", filters, err)
	}
	crm := records.NewService(store)
	actor := auth.Actor{UserID: user, WorkspaceID: workspace}
	for variant, id := range map[string]string{"linkedin.com/in/jim": jim, "https://x.com/ann/": ann, "linkedin.com/company/acme": acme} {
		object := "people"
		if id == acme {
			object = "companies"
		}
		found, _, err := crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: object, Set: map[string][]string{"links": {variant}}})
		if err != nil || found.ID != id {
			t.Fatalf("%s must find its record %s: %+v %v", variant, id, found.ID, err)
		}
	}
	owners, err := store.HandlesOnRecords(ctx, workspace)
	if err != nil {
		t.Fatal(err)
	}
	var paired []string
	for _, o := range owners {
		paired = append(paired, o.RecordID)
	}
	slices.Sort(paired)
	expected := []string{jim, ann}
	slices.Sort(expected)
	if !slices.Equal(paired, expected) {
		t.Fatalf("profile handles pair with the people whose links hold them: %v", owners)
	}
	var kinds []string
	rows, err = db.QueryContext(ctx, `SELECT DISTINCT kind FROM handles`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, kind)
	}
	if !slices.Equal(kinds, []string{"link"}) {
		t.Fatalf("handle kinds: %v", kinds)
	}
}
