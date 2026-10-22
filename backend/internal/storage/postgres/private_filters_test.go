package postgres_test

import (
	"reflect"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/lithammer/shortuuid/v4"
)

func TestPrivateFollowUpFilterMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 59)
	workspace, object, first, second, preset := shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New()
	if _, err := db.ExecContext(ctx, "INSERT INTO workspaces(id,name) VALUES($1,'Existing')", workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO objects(id,workspace_id,slug,name) VALUES($1,$2,'follow_ups','Follow-ups')", object, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users(id,workspace_id,name,email,created_at) VALUES($1,$3,'First','first@example.com','2026-01-01'),($2,$3,'Second','second@example.com','2026-01-02')", first, second, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO saved_filters(id,workspace_id,object_id,name,query,filters) VALUES($1,$2,$3,'Everyone','Customer','[{"attribute":"channel","operator":"is","value":"Email"}]')`, preset, workspace, object); err != nil {
		t.Fatal(err)
	}
	store := postgrestest.Open(t, dsn)
	a, err := store.SavedFilters(ctx, workspace, object, first)
	if err != nil || len(a) != 1 || a[0].ID != preset || a[0].Name == "Everyone" || a[0].Query != "Customer" || a[0].UserID == nil || *a[0].UserID != first {
		t.Fatalf("original preset and ownership: %+v %v", a, err)
	}
	b, err := store.SavedFilters(ctx, workspace, object, second)
	if err != nil || len(b) != 1 || b[0].ID == preset || b[0].Name != a[0].Name || b[0].Query != a[0].Query || !reflect.DeepEqual(b[0].Filters, a[0].Filters) || b[0].UserID == nil || *b[0].UserID != second {
		t.Fatalf("private copy: %+v %v", b, err)
	}
	if _, err := shortuuid.DefaultEncoder.Decode(b[0].ID); err != nil {
		t.Fatalf("private copy must use a full shortuuid: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users(id,workspace_id,name,email) VALUES($1,$2,'Later','later@example.com')", shortuuid.New(), workspace); err != nil {
		t.Fatal(err)
	}
	var later string
	if err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE workspace_id=$1 AND email='later@example.com'", workspace).Scan(&later); err != nil {
		t.Fatal(err)
	}
	if filters, err := store.SavedFilters(ctx, workspace, object, later); err != nil || len(filters) != 0 {
		t.Fatalf("new member inherited private presets: %+v %v", filters, err)
	}
	if err := store.DeleteFilter(ctx, workspace, preset, first); err != nil {
		t.Fatal(err)
	}
	if filters, err := store.SavedFilters(ctx, workspace, object, second); err != nil || len(filters) != 1 || filters[0].ID != b[0].ID {
		t.Fatalf("deleting original affected private copy: %+v %v", filters, err)
	}
}
