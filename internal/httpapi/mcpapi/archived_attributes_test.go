package mcpapi_test

import (
	"strings"
	"testing"
)

func TestArchivedReferenceSchemaAndRelatedRecords(t *testing.T) {
	agent, _ := connect(t)
	mustCall(t, agent, "create_attribute", map[string]any{"object": "people", "slug": "legacy_company", "name": "Legacy company", "type": "reference", "target": "companies"})
	company := mustCall(t, agent, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme"}})["record"].(map[string]any)["id"]
	person := mustCall(t, agent, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Ada", "legacy_company": company}})["record"].(map[string]any)["id"]
	args := map[string]any{"object": "people", "attribute": "legacy_company", "action": "archive"}
	mustCall(t, agent, "edit_attribute", args)
	if schema := encode(mustCall(t, agent, "list_objects", nil)); strings.Contains(schema, "legacy_company") || !strings.Contains(schema, `"protected":true`) {
		t.Fatalf("active schema or built-in protection metadata: %s", schema)
	}
	if schema := encode(mustCall(t, agent, "list_objects", map[string]any{"include_archived": true})); !strings.Contains(schema, `"archived":true,"name":"Legacy company"`) {
		t.Fatalf("archived schema missing restore metadata: %s", schema)
	}
	if record := encode(mustCall(t, agent, "get_record", map[string]any{"record_id": person})); strings.Contains(record, "legacy_company") {
		t.Fatalf("archived property exposed: %s", record)
	}
	if record := encode(mustCall(t, agent, "get_record", map[string]any{"record_id": company})); strings.Contains(record, "people.legacy_company") {
		t.Fatalf("archived incoming reference exposed: %s", record)
	}
	args["action"] = "restore"
	mustCall(t, agent, "edit_attribute", args)
	if record := encode(mustCall(t, agent, "get_record", map[string]any{"record_id": company})); !strings.Contains(record, "people.legacy_company") || !strings.Contains(record, person.(string)) {
		t.Fatalf("restored reference lost: %s", record)
	}
}
