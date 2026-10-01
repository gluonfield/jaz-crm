package mcpapi_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-tasks/auth"
)

func TestWorkspaceDeletion(t *testing.T) {
	e := serve(t)
	ctx := context.Background()
	store := e.store.(*postgres.Store)
	key := e.apiKey(t, "owner@example.com")
	owner := e.session(t, key)
	actor, err := e.keys.Authenticate(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	workspace := mustCall(t, owner, "get_workspace", nil)
	args := map[string]any{"workspace_id": workspace["id"], "name": workspace["name"]}
	other, err := e.people.Create(ctx, actor, "Personal")
	if err != nil {
		t.Fatal(err)
	}
	otherKey, _, err := e.keys.CreateKey(ctx, other.UserID, "other", "")
	if err != nil {
		t.Fatal(err)
	}
	otherSession := e.session(t, otherKey)
	otherRecord := mustCall(t, otherSession, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Keep me"}})["record"].(map[string]any)
	mustCall(t, owner, "invite_member", map[string]any{"email": "member@example.com"})
	member, err := e.people.SignIn(ctx, signin.Identity{Issuer: "https://idp.test", Subject: "member", Email: "member@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	memberKey, _, err := e.keys.CreateKey(ctx, member.ID, "member", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, failure := call(t, e.session(t, memberKey), "delete_workspace", args); failure == "" {
		t.Fatal("a non-admin deleted the workspace")
	}
	for _, invalid := range []map[string]any{
		{"workspace_id": other.WorkspaceID, "name": workspace["name"]},
		{"workspace_id": workspace["id"], "name": "wrong name"},
		{"name": workspace["name"]},
	} {
		if _, failure := call(t, owner, "delete_workspace", invalid); failure == "" {
			t.Fatalf("deleted without confirming the current workspace: %v", invalid)
		}
	}
	record := mustCall(t, owner, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Delete me"}})["record"].(map[string]any)
	mustCall(t, owner, "invite_member", map[string]any{"email": "pending@example.com"})
	connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: actor.WorkspaceID, UserID: actor.UserID, Provider: "google", Account: "mailbox@example.com", RefreshToken: []byte("encrypted")})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetCursor(ctx, connection.ID, "gmail_history", "123"); err != nil {
		t.Fatal(err)
	}
	interaction, err := store.UpsertInteraction(ctx, storage.NewInteraction{WorkspaceID: actor.WorkspaceID, Kind: "email", Source: "gmail", ExternalID: "thread", ConnectionID: &connection.ID, StartedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	content := "Private mail"
	at := time.Now()
	if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: interaction.ID, Kind: "message", ExternalID: "mail", ConnectionID: &connection.ID, At: &at, Content: &content}); err != nil {
		t.Fatal(err)
	}
	cookie, _, err := e.keys.CreateSession(ctx, actor.UserID)
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.UserByID(ctx, actor.UserID)
	if err != nil {
		t.Fatal(err)
	}
	token := e.oauth(t, user)
	mustCall(t, owner, "delete_workspace", args)
	for _, credential := range []string{key, memberKey, token} {
		if _, err := e.keys.Authenticate(ctx, credential); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("a deleted workspace's credential still works: %v", err)
		}
	}
	if _, err := e.keys.Session(ctx, cookie); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("a deleted workspace's browser session still works: %v", err)
	}
	if _, err := store.Workspace(ctx, actor.WorkspaceID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("workspace remains: %v", err)
	}
	if _, err := store.Connection(ctx, connection.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("connection remains: %v", err)
	}
	if _, err := store.Cursor(ctx, connection.ID, "gmail_history"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("sync cursor remains: %v", err)
	}
	for label, read := range map[string]func() (int, error){
		"members": func() (int, error) {
			rows, err := store.Users(ctx, actor.WorkspaceID)
			return len(rows), err
		},
		"invites": func() (int, error) {
			rows, err := store.Invites(ctx, actor.WorkspaceID)
			return len(rows), err
		},
		"objects": func() (int, error) {
			rows, err := store.Objects(ctx, actor.WorkspaceID)
			return len(rows), err
		},
		"records": func() (int, error) {
			rows, err := store.Records(ctx, actor.WorkspaceID, []string{record["id"].(string)})
			return len(rows), err
		},
		"conversations": func() (int, error) {
			rows, err := store.Interactions(ctx, actor.WorkspaceID, []string{interaction.ID})
			return len(rows), err
		},
		"content": func() (int, error) {
			rows, err := store.Parts(ctx, []string{interaction.ID})
			return len(rows), err
		},
	} {
		if count, err := read(); err != nil || count != 0 {
			t.Errorf("%s after deletion: %d, %v", label, count, err)
		}
	}
	if got := mustCall(t, otherSession, "get_record", map[string]any{"record_id": otherRecord["id"]}); got["values"].(map[string]any)["name"] != "Keep me" {
		t.Errorf("another workspace was changed: %v", got)
	}
	if got := mustCall(t, otherSession, "list_workspaces", nil)["workspaces"].([]any); len(got) != 1 {
		t.Errorf("deleted membership remains: %v", got)
	}
	mustCall(t, otherSession, "delete_workspace", map[string]any{"workspace_id": other.WorkspaceID, "name": "Personal"})
	if _, err := store.Workspace(ctx, other.WorkspaceID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("the last workspace could not be deleted: %v", err)
	}
}

func TestTriageSettingsAuthorization(t *testing.T) {
	e := serve(t)
	ctx := context.Background()
	key := e.apiKey(t, "owner@example.com")
	owner := e.session(t, key)
	settings := mustCall(t, owner, "get_triage_settings", nil)
	for flag, enabled := range settings {
		if enabled != false {
			t.Fatalf("new workspace auto-approves %s: %v", flag, enabled)
		}
	}
	mustCall(t, owner, "update_triage_settings", map[string]any{"auto_keep_email": true, "auto_keep_meetings": false, "auto_keep_records": false, "auto_keep_ai": false})
	if settings = mustCall(t, owner, "get_triage_settings", nil); settings["auto_keep_email"] != true || settings["auto_keep_meetings"] != false {
		t.Fatalf("settings not persisted independently: %v", settings)
	}
	if _, failure := call(t, owner, "update_triage_settings", map[string]any{"auto_keep_email": true, "auto_keep_meetings": false, "auto_keep_records": false, "auto_keep_ai": true}); failure == "" {
		t.Fatal("AI decisions enabled without criteria")
	}
	mustCall(t, owner, "update_workspace", map[string]any{"description": "Manufacturing customers and partners"})
	mustCall(t, owner, "update_triage_settings", map[string]any{"auto_keep_email": true, "auto_keep_meetings": false, "auto_keep_records": false, "auto_keep_ai": true})
	if settings = mustCall(t, owner, "get_triage_settings", nil); settings["auto_keep_ai"] != true {
		t.Fatalf("AI decisions not enabled with criteria: %v", settings)
	}
	mustCall(t, owner, "invite_member", map[string]any{"email": "triage-member@example.com"})
	member, err := e.people.SignIn(ctx, signin.Identity{Issuer: "https://idp.test", Subject: "triage-member", Email: "triage-member@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	memberKey, _, err := e.keys.CreateKey(ctx, member.ID, "member", "")
	if err != nil {
		t.Fatal(err)
	}
	session := e.session(t, memberKey)
	if _, failure := call(t, session, "update_triage_settings", map[string]any{"auto_keep_email": false, "auto_keep_meetings": true, "auto_keep_records": false, "auto_keep_ai": false}); failure == "" {
		t.Fatal("member changed workspace auto-approval policy")
	}
	if got := mustCall(t, session, "get_triage_settings", nil); got["auto_keep_email"] != true || got["auto_keep_meetings"] != false {
		t.Fatalf("member must see shared policy unchanged: %v", got)
	}
	otherKey := e.apiKey(t, "other@example.com")
	other := e.session(t, otherKey)
	if got := mustCall(t, other, "get_triage_settings", nil); got["auto_keep_email"] != false {
		t.Fatalf("another workspace inherited approvals: %v", got)
	}
}
