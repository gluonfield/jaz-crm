package postgres_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

func TestTextIDsPreserveExistingRecords(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 40)
	if _, err := db.ExecContext(ctx, `
INSERT INTO workspaces(name) VALUES('Legacy');
INSERT INTO users(workspace_id,name,email,admin) SELECT id,'Owner','owner@example.com',true FROM workspaces;
INSERT INTO objects(workspace_id,slug,name) SELECT id,'pages','Pages' FROM workspaces;
INSERT INTO attributes(object_id,slug,name,type) SELECT id,'name','Name','text' FROM objects;
INSERT INTO attributes(object_id,slug,name,type,target_object_id) SELECT id,'parent','Parent','reference',id FROM objects;
INSERT INTO records(workspace_id,object_id) SELECT workspace_id,id FROM objects;
INSERT INTO record_values(record_id,attribute_id,text,source,actor_id) SELECT records.id,attributes.id,'Legacy page','user',users.id FROM records,attributes,users WHERE attributes.slug='name';
INSERT INTO workspace_knowledge_pages(workspace_id,page_id,position) SELECT workspace_id,id,1 FROM records;
UPDATE workspaces SET company_page_id=(SELECT id FROM records);
INSERT INTO connections(workspace_id,user_id,provider,account,refresh_token) SELECT workspace_id,id,'google',email,'legacy'::bytea FROM users;
INSERT INTO interactions(workspace_id,connection_id,user_id,kind,source,external_id,started_at) SELECT workspace_id,id,user_id,'email','gmail','legacy-thread',now() FROM connections;
INSERT INTO links(interaction_id,record_id,source) SELECT interactions.id,records.id,'user' FROM interactions,records;
INSERT INTO sessions(token_hash,user_id,expires_at) SELECT 'legacy'::bytea,id,now()+interval '1 day' FROM users;
`); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		t.Helper()
		var rows []string
		for _, table := range []string{"workspaces", "users", "objects", "attributes", "records", "record_values", "workspace_knowledge_pages", "connections", "interactions", "links", "sessions"} {
			var row string
			value := "to_jsonb(t)"
			if table == "records" {
				value += " - 'updated_at'"
			}
			if err := db.QueryRowContext(ctx, "SELECT jsonb_agg("+value+" ORDER BY to_jsonb(t)::text)::text FROM "+table+" t").Scan(&row); err != nil {
				t.Fatal(err)
			}
			rows = append(rows, row)
		}
		return strings.Join(rows, "\n")
	}
	before := snapshot()
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if after := snapshot(); after != before {
		t.Fatalf("migration changed existing records:\nbefore %s\nafter %s", before, after)
	}
	var uuidColumns int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name <> 'domain_logos' AND udt_name IN ('uuid','_uuid')").Scan(&uuidColumns); err != nil || uuidColumns != 0 {
		t.Fatalf("remaining UUID columns: %d, %v", uuidColumns, err)
	}
	var actor auth.Actor
	if err := db.QueryRowContext(ctx, "SELECT id,workspace_id FROM users").Scan(&actor.UserID, &actor.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	var parent string
	if err := db.QueryRowContext(ctx, "SELECT id FROM records").Scan(&parent); err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	child, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "pages", Set: map[string][]string{"name": {"New page"}, "parent": {parent}}})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := shortuuid.DefaultEncoder.Decode(child.ID)
	if err != nil || shortuuid.DefaultEncoder.Encode(decoded) != child.ID {
		t.Fatalf("new record ID: %q, %v", child.ID, err)
	}
	for _, id := range []string{parent, child.ID} {
		if got, err := crm.Get(ctx, actor, id); err != nil || got.ID != id {
			t.Fatalf("lookup %q: %+v, %v", id, got, err)
		}
	}
	if len(child.Fields) != 2 || child.Fields[1].Attribute != "parent" || child.Fields[1].Values[0].RecordID != parent {
		t.Fatalf("mixed reference: %+v", child)
	}
	workspace, err := store.Workspace(ctx, actor.WorkspaceID)
	if err != nil || !slices.Equal(workspace.CompanyPageIDs, []string{parent}) {
		t.Fatalf("legacy knowledge pages: %+v, %v", workspace, err)
	}
	if _, err := crm.Get(ctx, actor, strings.ToLower(child.ID)); err == nil {
		t.Fatal("ID lookup must be case-sensitive")
	}
}
