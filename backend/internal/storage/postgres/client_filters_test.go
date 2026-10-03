package postgres_test

import (
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

func TestClientFilterMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 39)
	var workspace, object, record string
	if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('Existing') RETURNING id").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,'people','People') RETURNING id", workspace).Scan(&object); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id) VALUES($1,$2) RETURNING id", workspace, object).Scan(&record); err != nil {
		t.Fatal(err)
	}
	preset := shortuuid.New()
	if _, err := db.ExecContext(ctx, "INSERT INTO saved_filters(id,workspace_id,object_id,name,query) VALUES($1,$2,$3,'Shared preset','Customer')", preset, workspace, object); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO active_filters(object_id,query,saved_id) VALUES($1,'Someone else',$2)", object, preset); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	saved, err := store.SavedFilters(ctx, workspace, object)
	if err != nil || len(saved) != 1 || saved[0].ID != preset || saved[0].Name != "Shared preset" || saved[0].Query != "Customer" {
		t.Fatalf("shared preset changed: %+v %v", saved, err)
	}
	rows, err := store.Records(ctx, workspace, []string{record})
	if err != nil || len(rows) != 1 || rows[0].ID != record {
		t.Fatalf("shared records changed: %+v %v", rows, err)
	}
	var removed bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('active_filters') IS NULL").Scan(&removed); err != nil || !removed {
		t.Fatalf("workspace UI state must be removed: %v %v", removed, err)
	}
}
