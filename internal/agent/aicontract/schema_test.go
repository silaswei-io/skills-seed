package aicontract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONSchemaGeneratesDTOContract(t *testing.T) {
	schema, err := JSONSchema("AnalyzeCurrentCodebaseBatchOutput")

	require.NoError(t, err)
	require.Contains(t, schema, `"units"`)
	require.Contains(t, schema, `"profile_delta"`)
	require.Contains(t, schema, `"business_methods"`)
	require.Contains(t, schema, `"validation_commands"`)
	require.Contains(t, schema, `"current_location"`)
	require.Contains(t, schema, `packages/modules/event-bus-local/src/services/event-bus-local.ts:140`)
	require.Contains(t, schema, `never output code_location as a single string`)
	require.Contains(t, schema, `"additionalProperties": false`)

	var contract map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &contract))
	currentLocation, ok := findSchemaProperty(contract, "current_location")
	require.True(t, ok)
	require.Equal(t, "string", currentLocation["type"])
	require.Contains(t, currentLocation["description"], "repository-relative source file and 1-based line")
	require.Contains(t, currentLocation["examples"], "packages/modules/event-bus-local/src/services/event-bus-local.ts:140")
}

func TestJSONSchemaRejectsUnknownContract(t *testing.T) {
	schema, err := JSONSchema("MissingOutput")

	require.Error(t, err)
	require.Empty(t, schema)
}

func findSchemaProperty(schema map[string]any, name string) (map[string]any, bool) {
	if properties, ok := schema["properties"].(map[string]any); ok {
		if property, ok := properties[name].(map[string]any); ok {
			return property, true
		}
		for _, property := range properties {
			if nested, ok := property.(map[string]any); ok {
				if found, ok := findSchemaProperty(nested, name); ok {
					return found, true
				}
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		return findSchemaProperty(items, name)
	}
	return nil, false
}
