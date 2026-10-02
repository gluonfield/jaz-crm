package postgres_test

import (
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
)

func TestSentDraftMigrationPreservesUnsentTextAndHistory(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 28)
	var workspace string
	if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('Draft migration') RETURNING id").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"follow_ups", "custom"} {
		var object, draft, status string
		if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,$2,$2) RETURNING id", workspace, slug).Scan(&object); err != nil {
			t.Fatal(err)
		}
		for field, id := range map[string]*string{"draft": &draft, "draft_status": &status} {
			if err := db.QueryRowContext(ctx, "INSERT INTO attributes(object_id,slug,name,type) VALUES($1,$2,$2,'text') RETURNING id", object, field).Scan(id); err != nil {
				t.Fatal(err)
			}
		}
		for _, state := range []string{"Draft", "Approved", "Sending", "Sent"} {
			var record string
			if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id) VALUES($1,$2) RETURNING id", workspace, object).Scan(&record); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id,attribute_id,text,source) VALUES($1,$2,$3,'user'),($1,$4,$5,'user')", record, draft, slug+state, status, state); err != nil {
				t.Fatal(err)
			}
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	rows, err := db.QueryContext(ctx, "SELECT objects.slug, state.text, draft.text, draft.active_until IS NULL FROM record_values AS draft JOIN attributes AS field ON field.id = draft.attribute_id JOIN objects ON objects.id = field.object_id JOIN record_values AS state ON state.record_id = draft.record_id JOIN attributes AS status ON status.id = state.attribute_id WHERE field.slug = 'draft' AND status.slug = 'draft_status'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var slug, state, text string
		var active bool
		if err := rows.Scan(&slug, &state, &text, &active); err != nil {
			t.Fatal(err)
		}
		if text != slug+state || active != (slug != "follow_ups" || state != "Sent") {
			t.Fatalf("migration changed unsent text or history: %q %q %q %v", slug, state, text, active)
		}
		count++
	}
	if err := rows.Err(); err != nil || count != 8 {
		t.Fatalf("migration lost records: %d %v", count, err)
	}
}
