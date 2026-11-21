package mcpapi

import (
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

func inputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[map[string]any](): {Type: "object"},
	}})
	if err != nil {
		panic(err)
	}
	concreteTypes(schema)
	return schema
}

// Antigravity requires concrete types; optional inputs remain outside required.
func concreteTypes(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	if len(schema.Types) == 2 && schema.Types[0] == "null" {
		schema.Type = schema.Types[1]
		schema.Types = nil
	}
	for _, property := range schema.Properties {
		concreteTypes(property)
	}
	concreteTypes(schema.Items)
	concreteTypes(schema.AdditionalProperties)
}
