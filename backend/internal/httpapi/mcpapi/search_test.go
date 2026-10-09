package mcpapi_test

import (
	"slices"
	"testing"
)

func TestSearchFollowsOpenAIConvention(t *testing.T) {
	a, b := connect(t)
	company := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme", "domains": "acme.com"}})["record"].(map[string]any)
	person := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Bob", "email_addresses": "bob@acme.com"}})["record"].(map[string]any)
	mustCall(t, b, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme Other"}})
	results := mustCall(t, a, "search", map[string]any{"query": "acme"})["results"].([]any)
	want := []map[string]any{
		{"id": person["id"], "title": "Bob", "url": "http://crm.test/r/" + person["id"].(string), "text": "People"},
		{"id": company["id"], "title": "Acme", "url": "http://crm.test/r/" + company["id"].(string), "text": "Companies"},
	}
	for _, hit := range want {
		if !slices.ContainsFunc(results, func(r any) bool { return encode(r) == encode(hit) }) {
			t.Fatalf("missing %v in %v", hit, results)
		}
	}
	if len(results) != len(want) {
		t.Fatalf("results = %v", results)
	}
}
