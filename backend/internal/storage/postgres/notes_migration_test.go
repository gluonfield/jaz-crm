package postgres_test

import (
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

func TestNotesMigrationArchivesNotesWithTheirValues(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 51)
	workspace, user, object, notes, record := shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New()
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces(id, name) VALUES ($1, 'CAS')`, []any{workspace}},
		{`INSERT INTO users(id, workspace_id, name, email) VALUES ($1, $2, 'Owner', 'owner@jaz.test')`, []any{user, workspace}},
		{`INSERT INTO objects(id, workspace_id, slug, name) VALUES ($1, $2, 'people', 'People')`, []any{object, workspace}},
		{`INSERT INTO attributes(id, object_id, slug, name, type) VALUES ($1, $2, 'name', 'Name', 'text'), ($3, $2, 'notes', 'Notes', 'text')`, []any{shortuuid.New(), object, notes}},
		{`INSERT INTO records(id, workspace_id, object_id) VALUES ($1, $2, $3)`, []any{record, workspace, object}},
		{`INSERT INTO record_values(record_id, attribute_id, text, source) VALUES ($1, $2, 'Connected by Lib', 'agent')`, []any{record, notes}},
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
	crm := records.NewService(store)
	actor := auth.Actor{WorkspaceID: workspace, UserID: user}
	note := func() []string {
		got, err := crm.Get(ctx, actor, record)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(got.Fields, func(f records.Field) bool { return f.Attribute == "notes" })
		if i < 0 {
			return nil
		}
		var texts []string
		for _, v := range got.Fields[i].Values {
			texts = append(texts, v.Text)
		}
		return texts
	}
	if got := note(); got != nil {
		t.Fatalf("archived notes still show on the record: %v", got)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: "people", RecordID: record, Set: map[string][]string{"notes": {"Sources: https://example.test"}}}); err == nil {
		t.Fatal("an agent could still write notes")
	}
	if err := crm.EditAttribute(ctx, actor, "people", "notes", "restore", ""); err != nil {
		t.Fatal(err)
	}
	if got := note(); !slices.Equal(got, []string{"Connected by Lib"}) {
		t.Fatalf("restored notes lost their values: %v", got)
	}
}
