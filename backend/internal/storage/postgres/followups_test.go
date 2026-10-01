package postgres_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/jackc/pgx/v5"
	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

func TestFollowupUpgradeAndDeletedDefault(t *testing.T) {
	ctx := context.Background()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = postgrestest.DefaultURL
	}
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := "jazcrm_upgrade_" + shortuuid.New()
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		_ = admin.Close(ctx)
	})
	dsn, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	dsn.Path = "/" + name
	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true), goose.WithExcludeNames([]string{"0015_deal_followups.go"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var workspace string
	if err := db.QueryRowContext(ctx, "INSERT INTO workspaces (name) VALUES ('Existing') RETURNING id").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO objects (workspace_id, slug, name) VALUES ($1, 'deals', 'Deals');
`, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO attributes (object_id, slug, name, type, options)
SELECT id, 'stage', 'Stage', 'status', ARRAY['Lead', 'Review', 'Won', 'Lost'] FROM objects;
INSERT INTO attributes (object_id, slug, name, type) SELECT id, 'name', 'Name', 'text' FROM objects;
INSERT INTO records (workspace_id, object_id) SELECT workspace_id, id FROM objects;
INSERT INTO record_values (record_id, attribute_id, text, source)
SELECT records.id, attributes.id, CASE attributes.slug WHEN 'name' THEN 'Existing deal' ELSE 'Review' END, 'user'
FROM records JOIN attributes ON attributes.object_id = records.object_id;
`); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	svc := records.NewService(store)
	actor := auth.Actor{WorkspaceID: workspace}
	objects, err := svc.Objects(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, attr := range objects[0].Attributes {
		if attr.Slug == "stage" && !slices.Equal(attr.Options, []string{"Lead", "Review", "On hold", "Won", "Lost"}) {
			t.Fatalf("upgrade changed custom stages: %v", attr.Options)
		}
	}
	filters, err := svc.SavedFilters(ctx, actor, "deals")
	if err != nil || len(filters) != 1 {
		t.Fatalf("existing workspace did not get default: %v %v", filters, err)
	}
	found, err := svc.Search(ctx, actor, records.Search{Object: "deals"})
	if err != nil || len(found) != 1 {
		t.Fatalf("upgrade lost existing deal: %v %v", found, err)
	}
	if _, _, err := svc.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "deals", RecordID: found[0].ID, Set: map[string][]string{"stage": {"On hold"}, "next_follow_up_date": {"2000-01-01"}, "next_action": {"Call back"}}}); err != nil {
		t.Fatal(err)
	}
	if due, err := svc.Search(ctx, actor, records.Search{Object: "deals", Filters: filters[0].Filters}); err != nil || len(due) != 1 || due[0].ID != found[0].ID {
		t.Fatalf("migrated follow-up fields/filter unusable: %v %v", due, err)
	}
	if err := svc.DeleteFilter(ctx, actor, filters[0].ID); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = postgres.Open(ctx, dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	if filters, err := records.NewService(store).SavedFilters(ctx, actor, "deals"); err != nil || len(filters) != 0 {
		t.Fatalf("deleted default returned on restart: %v %v", filters, err)
	}
}
