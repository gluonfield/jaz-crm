package postgres_test

import (
	"context"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
)

// A workspace from before pages gains them, and its own tables gain content
// while keeping their records.
func TestPagesMigration(t *testing.T) {
	ctx := context.Background()
	db, dsn := legacy(t, 22)
	var workspace, user, object, quote string
	if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('CAS') RETURNING id").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO users(workspace_id,name,email) VALUES($1,'August','august@example.com') RETURNING id", workspace).Scan(&user); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []string{"people", "quotes"} {
		if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,$2,$2) RETURNING id", workspace, seed).Scan(&object); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO attributes(object_id,slug,name,type) VALUES($1,'name','Name','text')", object); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id) VALUES($1,$2) RETURNING id", workspace, object).Scan(&quote); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	crm := records.NewService(store)
	actor := auth.Actor{UserID: user, WorkspaceID: workspace}
	objects, err := crm.Objects(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string][]string{}
	for _, o := range objects {
		for _, a := range o.Attributes {
			slugs[o.Slug] = append(slugs[o.Slug], a.Slug+":"+a.Type)
		}
	}
	if !slices.Equal(slugs["pages"], []string{"name:text", "parent:reference", "content:markdown"}) || !slices.Equal(slugs["quotes"], []string{"name:text", "content:markdown"}) || !slices.Equal(slugs["people"], []string{"name:text"}) {
		t.Fatalf("migrated schema: %v", slugs)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "quotes", RecordID: quote, Set: map[string][]string{"content": {"# Terms"}}}); err != nil {
		t.Fatalf("an existing table record takes content: %v", err)
	}
	page, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "pages", Set: map[string][]string{"name": {"Research"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "pages", Set: map[string][]string{"name": {"Painpoints"}, "parent": {page.ID}}}); err != nil {
		t.Fatalf("a migrated workspace nests pages: %v", err)
	}
}
