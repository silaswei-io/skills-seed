package aicontract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONSchemaGeneratesDTOContract(t *testing.T) {
	schema, err := JSONSchema(ContractAnalyzeCurrentCodebaseBatch)

	require.NoError(t, err)
	require.Contains(t, schema, `"units"`)
	require.Contains(t, schema, `"business_method"`)
	require.Contains(t, schema, `"current_location"`)
	require.NotContains(t, schema, `"profile_delta"`)
	require.NotContains(t, schema, `"validation_commands"`)
	require.Contains(t, schema, `src/component/file.ext:140`)
	require.Contains(t, schema, `never output code_location as a single string`)
	require.Contains(t, schema, `"additionalProperties": false`)

	var contract map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &contract))
	currentLocation, ok := findSchemaProperty(contract, "current_location")
	require.True(t, ok)
	require.Equal(t, "string", currentLocation["type"])
	require.Contains(t, currentLocation["description"], "repository-relative source file and 1-based line")
	require.Contains(t, currentLocation["examples"], "src/component/file.ext:140")
}

func TestStructuredOutputSchemaOmitsUnsupportedMetaSchema(t *testing.T) {
	for name := range outputTypes {
		schema, err := StructuredOutputSchema(name)

		require.NoError(t, err, name)
		require.NotContains(t, schema, `"$schema"`, name)
		require.Contains(t, schema, `"additionalProperties": false`, name)
		var contract map[string]any
		require.NoError(t, json.Unmarshal([]byte(schema), &contract), name)
	}
}

func TestStructuredOutputSchemaEncodesDTOValueConstraints(t *testing.T) {
	batch := decodeSchema(t, ContractAnalyzeCurrentCodebaseBatch)
	currentLocation, container, ok := findSchemaPropertyWithContainer(batch, "current_location")
	require.True(t, ok)
	require.Equal(t, "string", currentLocation["type"])
	require.Contains(t, schemaStringList(container["required"]), "current_location")

	learning := decodeSchema(t, ContractLearnPatterns)
	category, _, ok := findSchemaPropertyWithContainer(learning, "category")
	require.True(t, ok)
	require.ElementsMatch(t, []string{
		"naming", "error", "structure", "concurrency", "testing", "business",
		"api", "database", "utils", "middleware", "config",
	}, schemaStringList(category["enum"]))
	confidence, _, ok := findSchemaPropertyWithContainer(learning, "confidence")
	require.True(t, ok)
	require.Equal(t, float64(0), confidence["minimum"])
	require.Equal(t, float64(1), confidence["maximum"])

	analyze := decodeSchema(t, ContractAnalyzeCode)
	severity, _, ok := findSchemaPropertyWithContainer(analyze, "severity")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"error", "warning", "info"}, schemaStringList(severity["enum"]))
	line, _, ok := findSchemaPropertyWithContainer(analyze, "line")
	require.True(t, ok)
	require.Equal(t, float64(1), line["minimum"])
}

func TestCurateSchemaContainsOnlyDecisionFields(t *testing.T) {
	schema, err := StructuredOutputSchema(ContractCuratePatterns)

	require.NoError(t, err)
	require.Contains(t, schema, `"source_ids"`)
	require.NotContains(t, schema, `"merged_from"`)
	require.NotContains(t, schema, `"good_example"`)
	require.NotContains(t, schema, `"evidence_locations"`)
	require.NotContains(t, schema, `"business_method"`)
	require.NotContains(t, schema, `"summary"`)
}

func TestWorkspaceContractsKeepIdentityOutOfAIOutput(t *testing.T) {
	profile := decodeSchema(t, ContractWorkspaceProfile)
	profileProperties := profile["properties"].(map[string]any)
	require.NotContains(t, profileProperties, "name")
	require.NotContains(t, profileProperties, "root_path")
	projectID, _, ok := findSchemaPropertyWithContainer(profile, "project_id")
	require.True(t, ok)
	require.Equal(t, "string", projectID["type"])

	spec := decodeSchema(t, ContractWorkspaceSpec)
	specProperties := spec["properties"].(map[string]any)
	require.NotContains(t, specProperties, "name")
	require.NotContains(t, specProperties, "root_path")
	require.NotContains(t, specProperties, "projects")
	kind, _, ok := findSchemaPropertyWithContainer(spec, "kind")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"project", "role", "path"}, schemaStringList(kind["enum"]))
	require.Contains(t, specProperties["change_order"].(map[string]any)["description"], "without numeric or list prefixes")
}

func TestJSONSchemaRejectsUnknownContract(t *testing.T) {
	schema, err := JSONSchema("MissingOutput")

	require.Error(t, err)
	require.Empty(t, schema)

	schema, err = StructuredOutputSchema("MissingOutput")
	require.Error(t, err)
	require.Empty(t, schema)
}

func findSchemaProperty(schema map[string]any, name string) (map[string]any, bool) {
	property, _, ok := findSchemaPropertyWithContainer(schema, name)
	return property, ok
}

func findSchemaPropertyWithContainer(schema map[string]any, name string) (map[string]any, map[string]any, bool) {
	if properties, ok := schema["properties"].(map[string]any); ok {
		if property, ok := properties[name].(map[string]any); ok {
			return property, schema, true
		}
		for _, property := range properties {
			if nested, ok := property.(map[string]any); ok {
				if found, container, ok := findSchemaPropertyWithContainer(nested, name); ok {
					return found, container, true
				}
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		return findSchemaPropertyWithContainer(items, name)
	}
	return nil, nil, false
}

func decodeSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := StructuredOutputSchema(name)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	return schema
}

func schemaStringList(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
