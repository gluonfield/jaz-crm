package mcpapi_test

import (
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// columns is an object's column order as list_objects reports it.
func columns(t *testing.T, session *mcp.ClientSession, slug string) []string {
	t.Helper()
	for _, o := range mustCall(t, session, "list_objects", nil)["objects"].([]any) {
		object := o.(map[string]any)
		if object["slug"] == slug {
			list, _ := object["columns"].([]any)
			var out []string
			for _, column := range list {
				out = append(out, column.(string))
			}
			return out
		}
	}
	t.Fatalf("no object %s", slug)
	return nil
}

func TestColumnOrder(t *testing.T) {
	e := serve(t)
	key := e.apiKey(t, "a@jaz.test")
	a := e.session(t, key)
	b := e.session(t, e.apiKey(t, "b@jaz.test"))
	mustCall(t, a, "create_object", map[string]any{"slug": "events", "name": "Events"})
	for _, slug := range []string{"date", "location", "cost"} {
		mustCall(t, a, "create_attribute", map[string]any{"object": "events", "slug": slug, "name": slug, "type": "text"})
	}
	order := func(session *mcp.ClientSession, object string, columns ...string) string {
		_, failure := call(t, session, "edit_object", map[string]any{"object": object, "action": "order", "columns": columns})
		return failure
	}
	if failure := order(a, "events", "location", "$updated_at", "date"); failure != "" {
		t.Fatal(failure)
	}
	if got := columns(t, e.session(t, key), "events"); !slices.Equal(got, []string{"location", "$updated_at", "date"}) {
		t.Fatalf("events columns %v", got)
	}

	mustCall(t, a, "edit_object", map[string]any{"object": "events", "action": "rename", "name": "Meetups"})
	mustCall(t, a, "edit_attribute", map[string]any{"object": "events", "attribute": "location", "action": "archive"})
	mustCall(t, a, "edit_attribute", map[string]any{"object": "events", "attribute": "location", "action": "restore"})
	if got := columns(t, a, "events"); !slices.Equal(got, []string{"location", "$updated_at", "date"}) {
		t.Fatalf("unrelated edits changed the order: %v", got)
	}
	mustCall(t, a, "create_attribute", map[string]any{"object": "events", "slug": "updated_at", "name": "Custom timestamp", "type": "text"})
	if failure := order(a, "events", "updated_at", "$updated_at", "location", "date"); failure != "" {
		t.Fatalf("attribute and computed columns collide: %s", failure)
	}
	if got := columns(t, a, "events"); !slices.Equal(got, []string{"updated_at", "$updated_at", "location", "date"}) {
		t.Fatalf("attribute and computed columns collide: %v", got)
	}
	if failure := order(a, "events", "location", "$updated_at", "date"); failure != "" {
		t.Fatal(failure)
	}

	if failure := order(a, "people", "company", "$last_contact", "tags"); failure != "" {
		t.Fatalf("a built-in table refused its column order: %s", failure)
	}
	if got := columns(t, a, "people"); !slices.Equal(got, []string{"company", "$last_contact", "tags"}) {
		t.Fatalf("people columns %v", got)
	}
	if got := columns(t, b, "people"); got != nil {
		t.Fatalf("another workspace's people show a's order: %v", got)
	}
	if _, failure := call(t, a, "edit_object", map[string]any{"object": "people", "action": "rename", "name": "Contacts"}); failure == "" {
		t.Fatal("a built-in table was renamed")
	}

	for _, bad := range [][]string{{"name", "date"}, {"date", "nope"}, {"date", "date"}, {"content"}, {"$people"}, {"$last_contact"}} {
		if order(a, "events", bad...) == "" {
			t.Fatalf("order %v was accepted", bad)
		}
	}
	mustCall(t, a, "edit_attribute", map[string]any{"object": "events", "attribute": "location", "action": "delete"})
	if got := columns(t, a, "events"); !slices.Equal(got, []string{"$updated_at", "date"}) {
		t.Fatalf("events columns after deleting location %v", got)
	}
	if failure := order(a, "events"); failure != "" {
		t.Fatal(failure)
	}
	if got := columns(t, a, "events"); got != nil {
		t.Fatalf("an empty order kept %v", got)
	}
}
