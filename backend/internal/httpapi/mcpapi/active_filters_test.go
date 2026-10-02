package mcpapi_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/gluonfield/jaz-tasks/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestActiveFiltersBelongToWorkspace(t *testing.T) {
	e := serve(t)
	ownerKey := e.apiKey(t, "owner@example.com")
	owner := e.session(t, ownerKey)
	mustCall(t, owner, "invite_member", map[string]any{"email": "member@example.com"})
	member, err := e.people.SignIn(context.Background(), signin.Identity{Issuer: "https://idp.test", Subject: "member", Email: "member@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	memberKey, _, err := e.keys.CreateKey(context.Background(), member.ID, "member", "")
	if err != nil {
		t.Fatal(err)
	}
	colleague := e.session(t, memberKey)
	other := e.session(t, e.apiKey(t, "other@example.com"))
	conditions := []any{map[string]any{"attribute": "name", "operator": "contains", "value": "Stone"}}
	preset := mustCall(t, owner, "save_filter", map[string]any{"object": "people", "name": "Stone family", "filters": conditions})
	args := map[string]any{"object": "people", "filters": conditions, "query": " Ada ", "saved_id": preset["id"]}
	want := mustCall(t, colleague, "set_active_filter", args)
	if want["query"] != " Ada " || !reflect.DeepEqual(want["filters"], conditions) || want["saved_id"] != preset["id"] {
		t.Fatalf("active filter: %v", want)
	}
	for _, session := range []*mcp.ClientSession{owner, e.session(t, ownerKey), e.session(t, memberKey)} {
		if got := mustCall(t, session, "get_active_filter", map[string]any{"object": "people"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("workspace members/devices disagree: got %v, want %v", got, want)
		}
	}
	for _, scope := range []struct {
		session *mcp.ClientSession
		object  string
	}{{other, "people"}, {owner, "companies"}} {
		got := mustCall(t, scope.session, "get_active_filter", map[string]any{"object": scope.object})
		if got["query"] != "" || len(got["filters"].([]any)) != 0 || got["saved_id"] != nil {
			t.Fatalf("active filter crossed a workspace/object boundary: %v", got)
		}
		if _, failure := call(t, scope.session, "set_active_filter", map[string]any{"object": scope.object, "saved_id": preset["id"]}); failure == "" {
			t.Fatal("linked a preset from another workspace/object")
		}
	}
	for _, invalid := range []map[string]any{
		{"object": "people", "filters": []any{map[string]any{"attribute": "missing", "operator": "is", "value": "x"}}},
		{"object": "people", "filters": []any{map[string]any{"attribute": "name", "operator": "invalid", "value": "x"}}},
		{"object": "missing"},
	} {
		if _, failure := call(t, owner, "set_active_filter", invalid); failure == "" {
			t.Fatalf("invalid filter accepted: %v", invalid)
		}
	}
	if got := mustCall(t, owner, "get_active_filter", map[string]any{"object": "people"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("failed writes changed the active filter: %v", got)
	}
	mustCall(t, colleague, "delete_saved_filter", map[string]any{"id": preset["id"]})
	delete(want, "saved_id")
	if got := mustCall(t, owner, "get_active_filter", map[string]any{"object": "people"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("deleting a preset must retain its active conditions: %v", got)
	}
	mustCall(t, owner, "set_active_filter", map[string]any{"object": "people"})
	if got := mustCall(t, colleague, "get_active_filter", map[string]any{"object": "people"}); got["query"] != "" || len(got["filters"].([]any)) != 0 {
		t.Fatalf("clear was not shared: %v", got)
	}
	args = map[string]any{"object": "follow_ups"}
	if got := mustCall(t, owner, "get_active_filter", args)["filters"].([]any); len(got) != 1 || got[0].(map[string]any)["value"] != "Open" {
		t.Fatalf("missing initial follow-up default: %v", got)
	}
	mustCall(t, colleague, "set_active_filter", args)
	if got := mustCall(t, e.session(t, ownerKey), "get_active_filter", args)["filters"].([]any); len(got) != 0 {
		t.Fatalf("cleared follow-up default returned: %v", got)
	}
}

func TestActiveFiltersFollowSchemaDeletion(t *testing.T) {
	e := serve(t)
	a := e.session(t, e.apiKey(t, "owner@example.com"))
	mustCall(t, a, "create_object", map[string]any{"slug": "suppliers", "name": "Suppliers"})
	mustCall(t, a, "create_attribute", map[string]any{"object": "people", "slug": "supplier", "name": "Supplier", "type": "reference", "target": "suppliers"})
	mustCall(t, a, "create_attribute", map[string]any{"object": "people", "slug": "code", "name": "Code", "type": "text"})
	conditions := []any{
		map[string]any{"attribute": "supplier", "operator": "is_not_empty"},
		map[string]any{"attribute": "code", "operator": "is", "value": "A1"},
		map[string]any{"attribute": "name", "operator": "contains", "value": "Ada"},
	}
	mustCall(t, a, "set_active_filter", map[string]any{"object": "people", "filters": conditions, "query": "Lovelace"})
	mustCall(t, a, "edit_object", map[string]any{"object": "suppliers", "action": "delete"})
	if got := mustCall(t, a, "get_active_filter", map[string]any{"object": "people"})["filters"]; !reflect.DeepEqual(got, conditions[1:]) {
		t.Fatalf("deleted reference condition remains: %v", got)
	}
	mustCall(t, a, "edit_attribute", map[string]any{"object": "people", "attribute": "code", "action": "delete"})
	got := mustCall(t, a, "get_active_filter", map[string]any{"object": "people"})
	if !reflect.DeepEqual(got["filters"], conditions[2:]) || got["query"] != "Lovelace" {
		t.Fatalf("attribute cleanup changed unrelated conditions: %v", got)
	}
}
