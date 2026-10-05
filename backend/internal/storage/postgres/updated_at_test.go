package postgres_test

import (
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
)

func TestUpdatedAtMigration(t *testing.T) {
	db, dsn := legacy(t, 40)
	ctx := t.Context()
	var workspace, object, attribute string
	for _, seed := range []struct {
		query string
		args  []any
		id    *string
	}{
		{"INSERT INTO workspaces(name) VALUES('CAS') RETURNING id", nil, &workspace},
		{"INSERT INTO objects(workspace_id,slug,name) VALUES($1,'people','People') RETURNING id", []any{&workspace}, &object},
		{"INSERT INTO attributes(object_id,slug,name,type) VALUES($1,'name','Name','text') RETURNING id", []any{&object}, &attribute},
	} {
		if err := db.QueryRowContext(ctx, seed.query, seed.args...).Scan(seed.id); err != nil {
			t.Fatal(err)
		}
	}
	created := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	closed := created.Add(48 * time.Hour)
	var empty, removed string
	for _, id := range []*string{&empty, &removed} {
		if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id,created_at) VALUES($1,$2,$3) RETURNING id", workspace, object, created).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id,attribute_id,text,source,active_from,active_until) VALUES($1,$2,'Former name','user',$3,$4)", removed, attribute, created.Add(time.Hour), closed); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.Records(ctx, workspace, []string{empty, removed})
	if err != nil || len(rows) != 2 {
		t.Fatalf("migrated records: %+v %v", rows, err)
	}
	for _, row := range rows {
		want := created
		if row.ID == removed {
			want = closed
		}
		if !row.UpdatedAt.Equal(want) || !row.CreatedAt.Equal(created) {
			t.Fatalf("backfilled %s: updated %s created %s; want updated %s", row.ID, row.UpdatedAt, row.CreatedAt, want)
		}
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id,attribute_id,text,source) VALUES($1,$2,'New name','sync')", empty, attribute); err != nil {
		t.Fatal(err)
	}
	afterInsert, err := store.Records(ctx, workspace, []string{empty})
	if err != nil || !afterInsert[0].UpdatedAt.After(created) {
		t.Fatalf("direct sync insert failed to update: %+v %v", afterInsert, err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE record_values SET text = text WHERE record_id = $1", empty); err != nil {
		t.Fatal(err)
	}
	unchanged, err := store.Records(ctx, workspace, []string{empty})
	if err != nil || !unchanged[0].UpdatedAt.Equal(afterInsert[0].UpdatedAt) {
		t.Fatalf("identical database update changed the timestamp: %+v %v", unchanged, err)
	}
}
