package mcpapi_test

import (
	"context"
	"encoding/json"
	"testing"
)

func TestToolFunctionSchemas(t *testing.T) {
	e := serve(t)
	session := e.session(t, e.apiKey(t, "schemas@jaz.test"))
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) == 0 || listed.NextCursor != "" {
		t.Fatalf("incomplete tool catalogue: %d tools, cursor %q", len(listed.Tools), listed.NextCursor)
	}
	for _, tool := range listed.Tools {
		t.Run(tool.Name, func(t *testing.T) {
			raw, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatal(err)
			}
			if schema["type"] != "object" {
				t.Fatalf("tool parameters must be an object: %s", raw)
			}
			functionSchema(t, schema)
		})
	}
}

func functionSchema(t *testing.T, schema map[string]any) {
	t.Helper()
	switch schema["type"] {
	case "string", "integer", "number", "boolean", "object":
	case "array":
		if _, ok := schema["items"].(map[string]any); !ok {
			t.Fatalf("array has no item schema: %s", encode(schema))
		}
	default:
		t.Fatalf("function schema needs one concrete type: %s", encode(schema))
	}
	properties, _ := schema["properties"].(map[string]any)
	for name, property := range properties {
		t.Run(name, func(t *testing.T) {
			functionSchema(t, property.(map[string]any))
		})
	}
	for _, key := range []string{"items", "additionalProperties"} {
		if child, ok := schema[key].(map[string]any); ok {
			t.Run(key, func(t *testing.T) {
				functionSchema(t, child)
			})
		}
	}
}
