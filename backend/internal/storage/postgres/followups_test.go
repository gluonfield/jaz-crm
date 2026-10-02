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

func TestStandardSchemaUpgradeAndDeletedDefault(t *testing.T) {
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
	old, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true), goose.WithExcludeNames([]string{"0015_deal_followups.go", "0019_follow_ups.go"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.UpTo(ctx, 14); err != nil {
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
	if _, err := db.ExecContext(ctx, `
INSERT INTO workspaces (name) VALUES ('Missing Context'), ('Existing Context');
INSERT INTO objects (workspace_id, slug, name)
SELECT id, 'people', 'People' FROM workspaces WHERE name IN ('Missing Context', 'Existing Context');
INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'name', 'Name', 'text' FROM objects WHERE slug = 'people';
INSERT INTO attributes (object_id, slug, name, type)
SELECT objects.id, 'context', 'Context', 'text' FROM objects
JOIN workspaces ON workspaces.id = objects.workspace_id WHERE workspaces.name = 'Existing Context';
INSERT INTO records (workspace_id, object_id) SELECT workspace_id, id FROM objects WHERE slug = 'people';
INSERT INTO record_values (record_id, attribute_id, text, source)
SELECT records.id, attributes.id, CASE attributes.slug WHEN 'name' THEN 'Existing person' ELSE E'Met through a friend.\nDiscussing a pilot.' END, 'user'
FROM records JOIN attributes ON attributes.object_id = records.object_id
JOIN objects ON objects.id = records.object_id WHERE objects.slug = 'people';
INSERT INTO objects (workspace_id, slug, name)
SELECT id, 'companies', 'Companies' FROM workspaces WHERE name IN ('Missing Context', 'Existing Context');
INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'name', 'Name', 'text' FROM objects WHERE slug = 'companies';
INSERT INTO attributes (object_id, slug, name, type, options)
SELECT objects.id, fields.slug, fields.name, fields.type, fields.options
FROM objects JOIN workspaces ON workspaces.id = objects.workspace_id
CROSS JOIN (VALUES ('founded_year', 'Founded year', 'number', ARRAY[]::text[]),
  ('size', 'Size', 'select', ARRAY['11-50', '51-200', 'Custom band'])) AS fields(slug, name, type, options)
WHERE objects.slug = 'companies' AND workspaces.name = 'Existing Context';
INSERT INTO records (workspace_id, object_id) SELECT workspace_id, id FROM objects WHERE slug = 'companies';
INSERT INTO record_values (record_id, attribute_id, text, source)
SELECT records.id, attributes.id, CASE attributes.slug WHEN 'name' THEN 'Existing company' WHEN 'founded_year' THEN '1984' ELSE '51-200' END, 'user'
FROM records JOIN attributes ON attributes.object_id = records.object_id
JOIN objects ON objects.id = records.object_id WHERE objects.slug = 'companies';
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
	for _, name := range []string{"Missing Context", "Existing Context"} {
		var workspaceID string
		if err := db.QueryRowContext(ctx, "SELECT id FROM workspaces WHERE name = $1", name).Scan(&workspaceID); err != nil {
			t.Fatal(err)
		}
		personActor := auth.Actor{WorkspaceID: workspaceID}
		people, _, err := svc.Search(ctx, personActor, records.Search{Object: "people"})
		if err != nil || len(people) != 1 {
			t.Fatalf("upgrade lost %s person: %v %v", name, people, err)
		}
		if name == "Missing Context" {
			if _, _, err := svc.Upsert(ctx, personActor, records.SourceUser, records.Write{Object: "people", RecordID: people[0].ID, Set: map[string][]string{"context": {"Met through a friend.\nDiscussing a pilot."}}}); err != nil {
				t.Fatalf("migrated person has no usable Context: %v", err)
			}
		}
		person, err := svc.Get(ctx, personActor, people[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]string{}
		for _, field := range person.Fields {
			values[field.Attribute] = field.Values[0].Text
		}
		if values["name"] != "Existing person" || values["context"] != "Met through a friend.\nDiscussing a pilot." {
			t.Fatalf("%s upgrade changed existing values: %v", name, values)
		}
		companies, _, err := svc.Search(ctx, personActor, records.Search{Object: "companies"})
		if err != nil || len(companies) != 1 {
			t.Fatalf("upgrade lost %s company: %v %v", name, companies, err)
		}
		if name == "Missing Context" {
			if _, _, err := svc.Upsert(ctx, personActor, records.SourceUser, records.Write{Object: "companies", RecordID: companies[0].ID, Set: map[string][]string{"founded_year": {"1984"}, "size": {"51-200"}}}); err != nil {
				t.Fatalf("migrated company profile is unusable: %v", err)
			}
		}
		company, err := svc.Get(ctx, personActor, companies[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		values = map[string]string{}
		for _, field := range company.Fields {
			values[field.Attribute] = field.Values[0].Text
		}
		if values["name"] != "Existing company" || values["founded_year"] != "1984" || values["size"] != "51-200" {
			t.Fatalf("%s upgrade changed existing company values: %v", name, values)
		}
		if name == "Existing Context" {
			if _, _, err := svc.Upsert(ctx, personActor, records.SourceUser, records.Write{Object: "companies", RecordID: company.ID, Set: map[string][]string{"size": {"Custom band"}}}); err != nil {
				t.Fatalf("upgrade lost custom size options: %v", err)
			}
		}
	}
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
	found, _, err := svc.Search(ctx, actor, records.Search{Object: "deals"})
	if err != nil || len(found) != 1 {
		t.Fatalf("upgrade lost existing deal: %v %v", found, err)
	}
	if _, _, err := svc.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "deals", RecordID: found[0].ID, Set: map[string][]string{"stage": {"On hold"}, "next_follow_up_date": {"2000-01-01"}, "next_action": {"Call back"}}}); err != nil {
		t.Fatal(err)
	}
	if due, _, err := svc.Search(ctx, actor, records.Search{Object: "deals", Filters: filters[0].Filters}); err != nil || len(due) != 1 || due[0].ID != found[0].ID {
		t.Fatalf("migrated follow-up fields/filter unusable: %v %v", due, err)
	}
	followUpFilters, err := svc.SavedFilters(ctx, actor, records.FollowUps)
	if err != nil || len(followUpFilters) != 2 {
		t.Fatalf("existing workspace did not get follow-up filters: %v %v", followUpFilters, err)
	}
	followUp, _, err := svc.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: map[string][]string{"name": {"Send the quote"}, "deal": {found[0].ID}, "review_on": {"2000-01-01"}}})
	if err != nil {
		t.Fatalf("migrated follow-ups unusable: %v", err)
	}
	if due, _, err := svc.Search(ctx, actor, records.Search{Object: records.FollowUps, Filters: followUpFilters[0].Filters}); err != nil || len(due) != 1 || due[0].ID != followUp.ID {
		t.Fatalf("migrated Needs attention filter does not find a due follow-up: %v %v", due, err)
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
