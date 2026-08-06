package aicontract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONSchemaGeneratesDTOContract(t *testing.T) {
	schema, err := JSONSchema(ContractAnalyzeCurrentCodebaseBatch)

	require.NoError(t, err)
	require.Contains(t, schema, `"focuses"`)
	require.Contains(t, schema, `"business_method"`)
	require.Contains(t, schema, "canonical reusable capability entry")
	require.Contains(t, schema, "complete signature")
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
	requireRequiredFields(t, batch, "focuses")
	requireRequiredFields(t, mustFindSchemaContainer(t, batch, "focus_id"), "focus_id", "focus_name", "patterns", "profile_refresh_recommended")
	requireRequiredFields(t, mustFindSchemaContainer(t, batch, "category"), "id", "name", "category", "description", "good_example", "bad_example", "rule", "confidence", "frequency")

	currentLocation, container, ok := findSchemaPropertyWithContainer(batch, "current_location")
	require.True(t, ok)
	require.Equal(t, "string", currentLocation["type"])
	require.Contains(t, schemaStringList(container["required"]), "current_location")

	learning := decodeSchema(t, ContractAnalyzeCurrentCodebaseBatch)
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

	delta := decodeSchema(t, ContractAnalyzeCurrentDeltaBatch)
	requireRequiredFields(t, delta, "knowledge_changes", "profile_refresh_recommended")
	requireRequiredFields(t, mustFindSchemaContainer(t, delta, "focus_action"), "focus_action", "focus_id", "pattern_action", "anchors", "reason")
	requireRequiredFields(t, mustFindSchemaContainer(t, delta, "change_kind"), "path", "change_kind", "description")

	changeKind, _, ok := findSchemaPropertyWithContainer(delta, "change_kind")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"added", "modified", "deleted"}, schemaStringList(changeKind["enum"]))
	line, _, ok := findSchemaPropertyWithContainer(delta, "line")
	require.True(t, ok)
	require.Equal(t, float64(1), line["minimum"])

	normalize := decodeSchema(t, ContractNormalizePatterns)
	requireRequiredFields(t, normalize, "patterns", "dropped")
	requireRequiredFields(t, mustFindSchemaContainer(t, normalize, "source_ids"), "id", "name", "category", "description", "rule", "confidence", "source_ids")
	requireRequiredFields(t, mustFindSchemaContainer(t, normalize, "reason_code"), "id", "reason_code", "reason")
	sourceIDs, _, ok := findSchemaPropertyWithContainer(normalize, "source_ids")
	require.True(t, ok)
	require.Equal(t, "array", sourceIDs["type"])
	reasonCode, _, ok := findSchemaPropertyWithContainer(normalize, "reason_code")
	require.True(t, ok)
	require.Contains(t, schemaStringList(reasonCode["enum"]), "unsafe_guidance")
}

func TestPlanningAndSelectionSchemasRequireDecisionFields(t *testing.T) {
	selection := decodeSchema(t, ContractSelectLearningCandidates)
	requireRequiredFields(t, selection, "selected_paths", "skipped_paths", "reason")
	requireRequiredFields(t, mustFindSchemaContainer(t, selection, "path"), "path", "reason")
	selectedPaths, _, ok := findSchemaPropertyWithContainer(selection, "selected_paths")
	require.True(t, ok)
	require.Contains(t, selectedPaths["description"], "exact candidate file list")
	require.Contains(t, selectedPaths["description"], "required paths must be included")
	require.Contains(t, selectedPaths["description"], "absolute paths")
	skippedPaths, _, ok := findSchemaPropertyWithContainer(selection, "skipped_paths")
	require.True(t, ok)
	require.Contains(t, skippedPaths["description"], "must not duplicate selected_paths")

	plan := decodeSchema(t, ContractPlanLearningAgenda)
	requireRequiredFields(t, plan, "focuses")
	requireRequiredFields(t, mustFindSchemaContainer(t, plan, "id"), "id", "name")
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

func TestWorkspaceStructuredOutputSchemaConstrainsProjectIDs(t *testing.T) {
	data, err := StructuredOutputSchemaWithOptions(ContractWorkspaceProfile, StructuredOutputOptions{
		ProjectIDs: []string{"network", "agent", "agent"},
	})
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))

	projectID, _, ok := findSchemaPropertyWithContainer(schema, "project_id")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"agent", "network"}, schemaStringList(projectID["enum"]))
	fromProjectID, _, ok := findSchemaPropertyWithContainer(schema, "from_project_id")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"agent", "network"}, schemaStringList(fromProjectID["enum"]))
	for _, name := range []string{"consumers", "producers", "affected_projects", "project_ids"} {
		property, _, ok := findSchemaPropertyWithContainer(schema, name)
		require.True(t, ok, name)
		items := property["items"].(map[string]any)
		require.ElementsMatch(t, []string{"agent", "network"}, schemaStringList(items["enum"]), name)
	}
	require.NotContains(t, data, "ntls-workspace")
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

func mustFindSchemaContainer(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	_, container, ok := findSchemaPropertyWithContainer(schema, name)
	require.True(t, ok, "schema property %q not found", name)
	return container
}

func requireRequiredFields(t *testing.T, schema map[string]any, names ...string) {
	t.Helper()
	required := schemaStringList(schema["required"])
	for _, name := range names {
		require.Contains(t, required, name)
	}
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
