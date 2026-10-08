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
	objects, err := crm.Objects(ctx, actor, false)
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string][]string{}
	for _, o := range objects {
		for _, a := range o.Attributes {
			slugs[o.Slug] = append(slugs[o.Slug], a.Slug+":"+a.Type)
		}
	}
	if !slices.Equal(slugs["pages"], []string{"name:text", "parent:reference", "content:markdown", "icon:text"}) || !slices.Equal(slugs["quotes"], []string{"name:text", "content:markdown"}) || !slices.Equal(slugs["people"], []string{"name:text"}) {
		t.Fatalf("migrated schema: %v", slugs)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "quotes", RecordID: quote, Set: map[string][]string{"content": {"# Terms"}}}); err != nil {
		t.Fatalf("an existing table record takes content: %v", err)
	}
	page, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "pages", Set: map[string][]string{"name": {"Research"}, "icon": {"📚"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "pages", Set: map[string][]string{"name": {"Painpoints"}, "parent": {page.ID}}}); err != nil {
		t.Fatalf("a migrated workspace nests pages: %v", err)
	}
}

func TestCompanyKnowledgeMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 26)
	roots := map[string]string{}
	for _, count := range []int{0, 1, 2} {
		var workspace, object, name, parent string
		if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('CAS') RETURNING id").Scan(&workspace); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,'pages','Pages') RETURNING id", workspace).Scan(&object); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "INSERT INTO attributes(object_id,slug,name,type) VALUES($1,'name','Name','text') RETURNING id", object).Scan(&name); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "INSERT INTO attributes(object_id,slug,name,type,target_object_id) VALUES($1,'parent','Parent','reference',$1) RETURNING id", object).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		page := func(under string) string {
			t.Helper()
			var id string
			if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id) VALUES($1,$2) RETURNING id", workspace, object).Scan(&id); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id,attribute_id,text,source) VALUES($1,$2,'Company','user')", id, name); err != nil {
				t.Fatal(err)
			}
			if under != "" {
				if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id,attribute_id,ref_record_id,source) VALUES($1,$2,$3,'user')", id, parent, under); err != nil {
					t.Fatal(err)
				}
			}
			return id
		}
		roots[workspace] = ""
		for range count {
			root := page("")
			page(root)
			if count == 1 {
				roots[workspace] = root
			}
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for id, want := range roots {
		ws, err := store.Workspace(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		if len(ws.CompanyPageIDs) > 0 {
			got = ws.CompanyPageIDs[0]
		}
		if got != want {
			t.Fatalf("migrate only an unambiguous top-level Company page: got %q want %q", got, want)
		}
	}
}
