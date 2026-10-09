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
	hit := func(record map[string]any, title, text string) map[string]any {
		id := record["id"].(string)
		target := map[string]any{"type": "mcp_app_tool", "name": "show_crm", "arguments": map[string]any{"path": "/r/" + id}}
		return map[string]any{"id": id, "title": title, "url": "http://crm.test/r/" + id, "text": text, "_meta": map[string]any{"openai/preview": map[string]any{"target": target}}}
	}
	want := []map[string]any{hit(person, "Bob", "People"), hit(company, "Acme", "Companies")}
	for _, hit := range want {
		if !slices.ContainsFunc(results, func(r any) bool { return encode(r) == encode(hit) }) {
			t.Fatalf("missing %v in %v", hit, results)
		}
	}
	if len(results) != len(want) {
		t.Fatalf("results = %v", results)
	}
}
