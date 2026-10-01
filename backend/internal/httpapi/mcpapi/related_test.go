package mcpapi_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func TestSearchIncludesRelatedRecords(t *testing.T) {
	e := serve(t)
	a := e.session(t, e.apiKey(t, "a@jaz.test"))
	b := e.session(t, e.apiKey(t, "b@jaz.test"))
	company := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme", "domains": "acme.com"}})["record"].(map[string]any)["id"]
	empty := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Empty"}})["record"].(map[string]any)["id"]
	other := mustCall(t, b, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Private"}})["record"].(map[string]any)["id"]
	var ids []any
	store := e.store.(storage.InteractionStore)
	workspace := mustCall(t, a, "get_workspace", nil)["id"].(string)
	for i := range 6 {
		person := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": fmt.Sprintf("Person %d", i), "company": company}})["record"].(map[string]any)
		ids = append(ids, person["id"])
		address := fmt.Sprintf("person%d@acme.com", i)
		handle, err := store.UpsertHandle(context.Background(), storage.NewHandle{WorkspaceID: workspace, Kind: "email", Value: address, Triage: "kept"})
		if err != nil {
			t.Fatal(err)
		}
		id := person["id"].(string)
		if err := store.SetTriage(context.Background(), storage.Verdict{WorkspaceID: workspace, ID: handle.ID, Triage: "kept", PersonID: &id}); err != nil {
			t.Fatal(err)
		}
		if err := store.SetPhotos(context.Background(), workspace, map[string]string{address: fmt.Sprintf("https://photos.test/%d.png", i)}); err != nil {
			t.Fatal(err)
		}
	}
	mustCall(t, b, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Private person", "company": other}})
	args := map[string]any{"object": "companies", "include": []any{map[string]any{"object": "people", "attribute": "company", "limit": 4}}}
	result := mustCall(t, a, "search_records", args)["records"].([]any)
	if len(result) != 2 {
		t.Fatalf("companies: %v", result)
	}
	for _, raw := range result {
		record := raw.(map[string]any)
		people := record["related"].(map[string]any)["people.company"].([]any)
		if record["id"] == empty {
			if len(people) != 0 {
				t.Fatalf("empty company gained people: %v", people)
			}
			continue
		}
		if len(people) != 4 {
			t.Fatalf("per-company limit: %v", people)
		}
		for i, raw := range people {
			person := raw.(map[string]any)
			if person["id"] != ids[5-i] || person["name"] != fmt.Sprintf("Person %d", 5-i) || person["photo"] != fmt.Sprintf("https://photos.test/%d.png", 5-i) {
				t.Fatalf("related person's order, name or picture: %v", person)
			}
		}
	}
	mustCall(t, a, "upsert_record", map[string]any{"object": "people", "record_id": ids[5], "values": map[string]any{"company": empty}})
	result = mustCall(t, a, "search_records", args)["records"].([]any)
	for _, raw := range result {
		record := raw.(map[string]any)
		people := record["related"].(map[string]any)["people.company"].([]any)
		if record["id"] == empty && (len(people) != 1 || people[0].(map[string]any)["id"] != ids[5]) {
			t.Fatalf("reassigned person missing: %v", people)
		}
		if record["id"] == company && people[0].(map[string]any)["id"] != ids[4] {
			t.Fatalf("closed relationship remained visible: %v", people)
		}
	}
	args["where"] = map[string]any{"name": "Acme"}
	if filtered := mustCall(t, a, "search_records", args)["records"].([]any); len(filtered) != 1 || filtered[0].(map[string]any)["id"] != company {
		t.Fatalf("filtered parent selection: %v", filtered)
	}
	args["where"] = map[string]any{"name": "Private"}
	if found := mustCall(t, a, "search_records", args)["records"].([]any); len(found) != 0 {
		t.Fatalf("private relationships crossed workspaces: %v", found)
	}
	unnamed := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"email_addresses": "unnamed@acme.com", "company": empty}})["record"].(map[string]any)["id"]
	args["where"] = map[string]any{"name": "Empty"}
	preview := mustCall(t, a, "search_records", args)["records"].([]any)[0].(map[string]any)["related"].(map[string]any)["people.company"].([]any)[0].(map[string]any)
	if preview["id"] != unnamed || preview["name"] != "unnamed@acme.com" {
		t.Fatalf("a person without a name lost their email label: %v", preview)
	}
	delete(args, "where")
	for _, include := range []any{
		[]any{map[string]any{"object": "people", "attribute": "name"}},
		[]any{map[string]any{"object": "people", "attribute": "company", "limit": 21}},
		[]any{map[string]any{"object": "deals", "attribute": "primary_contact"}},
	} {
		args["include"] = include
		if _, failure := call(t, a, "search_records", args); failure == "" {
			t.Fatalf("invalid relationship selection accepted: %v", include)
		}
	}
}
