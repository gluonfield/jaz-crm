package postgres_test

import (
	"cmp"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"

	"github.com/gluonfield/jaz-crm/records"
	"github.com/lithammer/shortuuid/v4"
)

// Tables were listed above every page; their positions keep them there, in
// the order they were made, even below a page moved before the others.
func TestTablePositionsMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 56)
	workspace, user, pages, position, company, bot := shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New()
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces(id, name) VALUES ($1, 'CAS')`, []any{workspace}},
		{`INSERT INTO users(id, workspace_id, name, email) VALUES ($1, $2, 'Owner', 'owner@cas.dev')`, []any{user, workspace}},
		{`INSERT INTO objects(id, workspace_id, slug, name, created_at) VALUES ($1, $2, 'pages', 'Pages', '2026-09-01')`, []any{pages, workspace}},
		{`INSERT INTO attributes(id, object_id, slug, name, type) VALUES ($1, $2, 'position', 'Position', 'number')`, []any{position, pages}},
		{`INSERT INTO records(id, workspace_id, object_id, created_at) VALUES ($1, $3, $4, '2026-10-02'), ($2, $3, $4, '2026-10-05')`, []any{company, bot, workspace, pages}},
		{`INSERT INTO record_values(record_id, attribute_id, text, source) VALUES ($1, $2, '1759000000000', 'user')`, []any{bot, position}},
		{`INSERT INTO objects(id, workspace_id, slug, name, created_at, parent_id) VALUES
			($1, $4, 'events', 'Events', '2026-10-17', NULL),
			($2, $4, 'problems', 'Industry Problems', '2026-10-01', NULL),
			($3, $4, 'standups', 'Team Meetings', '2026-10-12', $5)`, []any{shortuuid.New(), shortuuid.New(), shortuuid.New(), workspace, company}},
	} {
		if _, err := db.ExecContext(ctx, seed.query, seed.args...); err != nil {
			t.Fatal(err)
		}
	}
	store := postgrestest.Open(t, dsn)
	objects, err := records.NewService(store).Objects(ctx, records.Actor{MemberID: user, WorkspaceID: workspace}, false)
	if err != nil {
		t.Fatal(err)
	}
	var tables []records.Object
	for _, o := range objects {
		if !o.Standard {
			tables = append(tables, o)
		}
	}
	slices.SortFunc(tables, func(a, b records.Object) int { return cmp.Compare(a.Position, b.Position) })
	var order []string
	for _, table := range tables {
		order = append(order, table.Slug)
	}
	if !slices.Equal(order, []string{"problems", "standups", "events"}) || tables[2].Position >= 1759000000000 {
		t.Fatalf("tables after migration: %v, last at %v", order, tables[2].Position)
	}
}
