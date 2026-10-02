package postgres_test

import (
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
)

func TestEmailHTMLMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 27)
	var connection, interaction string
	var part int64
	if err := db.QueryRowContext(ctx, `
WITH workspace AS (
  INSERT INTO workspaces(name) VALUES('CAS') RETURNING id
), member AS (
  INSERT INTO users(workspace_id,name,email)
  SELECT id,'Owner','owner@cas.dev' FROM workspace RETURNING id,workspace_id
), connection AS (
  INSERT INTO connections(workspace_id,user_id,provider,account,refresh_token)
  SELECT workspace_id,id,'google','owner@cas.dev','sealed'::bytea FROM member RETURNING id,workspace_id
), object AS (
  INSERT INTO objects(workspace_id,slug,name)
  SELECT id,'people','People' FROM workspace RETURNING id,workspace_id
), record AS (
  INSERT INTO records(workspace_id,object_id)
  SELECT workspace_id,id FROM object RETURNING id
), interaction AS (
  INSERT INTO interactions(workspace_id,connection_id,kind,source,external_id,started_at)
  SELECT workspace_id,id,'email','gmail','thread',now() FROM connection RETURNING id,connection_id
), link AS (
  INSERT INTO links(interaction_id,record_id,source)
  SELECT interaction.id,record.id,'user' FROM interaction,record
), part AS (
  INSERT INTO parts(interaction_id,connection_id,kind,external_id,provider_id,at,content)
  SELECT id,connection_id,'message','message','m1',now(),'Previously fetched text' FROM interaction RETURNING id
)
SELECT connection.id,interaction.id,part.id FROM connection,interaction,part`).Scan(&connection, &interaction, &part); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	due, err := store.UnfetchedParts(ctx, connection, 50)
	if err != nil || len(due) != 1 || due[0].ID != part {
		t.Fatalf("existing email must refresh its HTML: %+v %v", due, err)
	}
	parts, err := store.Parts(ctx, []string{interaction})
	if err != nil || len(parts) != 1 || parts[0].Content == nil || *parts[0].Content != "Previously fetched text" || parts[0].HTML != nil {
		t.Fatalf("migration must retain the existing body: %+v %v", parts, err)
	}
	if err := store.SetPartContent(ctx, part, "Previously fetched text", ""); err != nil {
		t.Fatal(err)
	}
	if due, err := store.UnfetchedParts(ctx, connection, 50); err != nil || len(due) != 0 {
		t.Fatalf("plain-text email must finish refreshing: %+v %v", due, err)
	}
}
