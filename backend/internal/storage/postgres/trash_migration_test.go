package postgres_test

import (
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
)

func TestTrashMigrationRetainsExistingValuesAndReferences(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 44)
	workspace := "workspace"
	for _, query := range []string{
		"INSERT INTO workspaces(id,name) VALUES('workspace','CAS')",
		"INSERT INTO objects(id,workspace_id,slug,name) VALUES('pages','workspace','pages','Pages')",
		"INSERT INTO attributes(id,object_id,slug,name,type) VALUES('name','pages','name','Name','text'),('content','pages','content','Content','markdown')",
		"INSERT INTO attributes(id,object_id,slug,name,type,target_object_id) VALUES('parent','pages','parent','Parent','reference','pages')",
		"INSERT INTO records(id,workspace_id,object_id) VALUES('root','workspace','pages'),('child','workspace','pages')",
		"INSERT INTO record_values(record_id,attribute_id,text,source) VALUES('root','name','Videos','user'),('root','content','Original links','user')",
		"INSERT INTO record_values(record_id,attribute_id,ref_record_id,source) VALUES('child','parent','root','user')",
		"INSERT INTO workspace_knowledge_pages(workspace_id,page_id,position) VALUES('workspace','root',0)",
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	crm := records.NewService(store)
	actor := auth.Actor{WorkspaceID: workspace}
	before, err := crm.Get(ctx, actor, "root")
	if err != nil {
		t.Fatal(err)
	}
	if err := crm.Delete(ctx, actor, "root"); err != nil {
		t.Fatal(err)
	}
	ws, err := store.Workspace(ctx, workspace)
	if err != nil || len(ws.CompanyPageIDs) != 0 {
		t.Fatalf("trashed knowledge root remains active: %v %v", ws.CompanyPageIDs, err)
	}
	if err := crm.Restore(ctx, actor, "root"); err != nil {
		t.Fatal(err)
	}
	after, err := crm.Get(ctx, actor, "root")
	if err != nil || !slices.EqualFunc(before.Fields, after.Fields, func(a, b records.Field) bool {
		return a.Attribute == b.Attribute && slices.Equal(a.Values, b.Values)
	}) {
		t.Fatalf("populated upgrade lost values: %+v %v", after, err)
	}
	ws, err = store.Workspace(ctx, workspace)
	if err != nil || !slices.Equal(ws.CompanyPageIDs, []string{"root"}) {
		t.Fatalf("restored knowledge root lost selection: %v %v", ws.CompanyPageIDs, err)
	}
	child, err := crm.Get(ctx, actor, "child")
	if err != nil || len(child.Fields) != 1 || child.Fields[0].Values[0].RecordID != "root" {
		t.Fatalf("populated upgrade lost the child: %+v %v", child, err)
	}
	_, _, err = crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: "pages", RecordID: "root", Set: map[string][]string{"content": {"Edited links"}}})
	if err != nil {
		t.Fatalf("the active-record view cannot be edited: %v", err)
	}
}
