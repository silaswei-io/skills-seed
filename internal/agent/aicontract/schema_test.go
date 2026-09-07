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

func TestStrictStructuredOutputSchemaRequiresEveryObjectProperty(t *testing.T) {
	for name := range outputTypes {
		data, err := StrictStructuredOutputSchema(name)

		require.NoError(t, err, name)
		var schema map[string]any
		require.NoError(t, json.Unmarshal([]byte(data), &schema), name)
		requireStrictRequiredProperties(t, schema, name)
	}
}

func TestStrictStructuredOutputSchemaKeepsOptionalFocusFieldsRepresentable(t *testing.T) {
	plan := decodeStrictSchema(t, ContractPlanLearningAgenda)
	focus := mustFindSchemaContainer(t, plan, "attributes")
	requireRequiredFields(t, focus, "id", "name", "route_terms", "attributes", "risk_signals", "analysis_depth", "entry_paths", "related_paths", "scope_reason")

	attributes, _, ok := findSchemaPropertyWithContainer(plan, "attributes")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"array", "null"}, schemaStringList(attributes["type"]))
	require.Contains(t, attributes["description"], "Return an empty array when no value applies")

	scopeReason, _, ok := findSchemaPropertyWithContainer(plan, "scope_reason")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"string", "null"}, schemaStringList(scopeReason["type"]))
	require.Contains(t, scopeReason["description"], "Return null when no value applies")
}

func TestStrictStructuredOutputSchemaAllowsNullForOptionalEnums(t *testing.T) {
	authority := decodeStrictSchema(t, ContractAuthorityExtraction)
	commandPolicy, container, ok := findSchemaPropertyWithContainer(authority, "command_policy")
	require.True(t, ok)
	require.Contains(t, schemaStringList(container["required"]), "command_policy")
	require.ElementsMatch(t, []string{"string", "null"}, schemaStringList(commandPolicy["type"]))
	require.Contains(t, commandPolicy["enum"].([]any), nil)
}

func TestStructuredOutputSchemaAllowsExplicitNullForOptionalFields(t *testing.T) {
	data, err := StructuredOutputSchema(ContractAnalyzeCurrentDeltaBatch)
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	changes := schema["properties"].(map[string]any)["knowledge_changes"].(map[string]any)
	items := changes["items"].(map[string]any)
	proposal := items["properties"].(map[string]any)["proposal"].(map[string]any)
	require.ElementsMatch(t, []any{"object", "null"}, proposal["type"])
}

func TestStructuredOutputSchemaEncodesDTOValueConstraints(t *testing.T) {
	profile := decodeSchema(t, ContractProjectProfile)
	profileProperties := profile["properties"].(map[string]any)
	require.NotContains(t, profileProperties, "authority_coverage")
	require.NotContains(t, profileProperties, "engineering_rules")
	require.NotContains(t, profileProperties, "authority_reviews")
	require.NotContains(t, profileProperties, "authority_sections")
	authority := decodeSchema(t, ContractAuthorityExtraction)
	requireRequiredFields(t, authority, "authority_sections")
	authorityContainer := mustFindSchemaContainer(t, authority, "section_id")
	requireRequiredFields(t, authorityContainer, "section_id", "rules")
	require.NotContains(t, schemaStringList(authorityContainer["required"]), "no_rule_reason")
	authorityRuleContainer := mustFindSchemaContainer(t, authority, "command_policy")
	requireRequiredFields(t, authorityRuleContainer, "title", "rule")
	require.NotContains(t, schemaStringList(authorityRuleContainer["required"]), "applies_to")
	require.NotContains(t, schemaStringList(authorityRuleContainer["required"]), "command_policy")
	authorityRuleProperties := authorityRuleContainer["properties"].(map[string]any)
	require.NotContains(t, authorityRuleProperties, "source")
	require.NotContains(t, authorityRuleProperties, "section")
	require.NotContains(t, authorityRuleProperties, "evidence")

	batch := decodeSchema(t, ContractAnalyzeCurrentCodebaseBatch)
	requireRequiredFields(t, batch, "focuses")
	requireRequiredFields(t, mustFindSchemaContainer(t, batch, "focus_id"), "focus_id", "focus_name", "patterns", "profile_refresh_recommended")
	requireRequiredFields(t, mustFindSchemaContainer(t, batch, "category"), "id", "name", "category", "description", "good_example", "bad_example", "rule", "confidence", "frequency", "knowledge_flags")
	flags, _, ok := findSchemaPropertyWithContainer(batch, "knowledge_flags")
	require.True(t, ok)
	require.Equal(t, "array", flags["type"])
	require.ElementsMatch(t, []string{"operational_risk"}, schemaStringList(flags["items"].(map[string]any)["enum"]))

	currentLocation, container, ok := findSchemaPropertyWithContainer(batch, "current_location")
	require.True(t, ok)
	require.Equal(t, "string", currentLocation["type"])
	require.Contains(t, schemaStringList(container["required"]), "current_location")

	learning := decodeSchema(t, ContractAnalyzeCurrentCodebaseBatch)
	category, _, ok := findSchemaPropertyWithContainer(learning, "category")
	require.True(t, ok)
	require.ElementsMatch(t, []string{
		"naming", "error", "structure", "concurrency", "business",
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
	require.Contains(t, schemaStringList(reasonCode["enum"]), "overfiltered_source_backed")
}

func TestPlanningSchemaKeepsCoverageCompletionDeterministic(t *testing.T) {
	plan := decodeSchema(t, ContractPlanLearningAgenda)
	requireRequiredFields(t, plan, "focuses", "reason")
	require.NotContains(t, schemaStringList(plan["required"]), "skipped_paths")
	requireRequiredFields(t, mustFindSchemaContainer(t, plan, "id"), "id", "name")
}

func TestKnowledgeReviewSchemaRequiresOneDecisionShape(t *testing.T) {
	review := decodeSchema(t, ContractReviewKnowledge)
	requireRequiredFields(t, review, "decisions")
	decision := mustFindSchemaContainer(t, review, "candidate_id")
	requireRequiredFields(t, decision, "candidate_id", "verdict", "reason_code", "reason")
	candidateID, _, ok := findSchemaPropertyWithContainer(review, "candidate_id")
	require.True(t, ok)
	require.Equal(t, float64(1), candidateID["minLength"])
	require.NotContains(t, schemaStringList(decision["required"]), "business_method")
	decisionProperties := decision["properties"].(map[string]any)
	require.NotContains(t, decisionProperties, "business_method_verdict")
	require.Contains(t, decisionProperties, "business_method")
	verdict, _, ok := findSchemaPropertyWithContainer(review, "verdict")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"accept", "revise", "reject"}, schemaStringList(verdict["enum"]))
	revision := mustFindSchemaContainer(t, review, "knowledge_flags")
	requireRequiredFields(t, revision, "name", "category", "description", "rule", "confidence", "knowledge_flags")
}

func TestKnowledgeReviewSchemaConstrainsCandidateIDsOneToOne(t *testing.T) {
	data, err := StructuredOutputSchemaWithOptions(ContractReviewKnowledge, StructuredOutputOptions{
		CandidateIDs: []string{"cert-b", "cert-a", "cert-a"},
	})
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	decisions := schema["properties"].(map[string]any)["decisions"].(map[string]any)
	require.Equal(t, float64(2), decisions["minItems"])
	require.Equal(t, float64(2), decisions["maxItems"])
	require.Equal(t, true, decisions["uniqueItems"])
	candidateID, _, ok := findSchemaPropertyWithContainer(schema, "candidate_id")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"cert-a", "cert-b"}, schemaStringList(candidateID["enum"]))
}

func TestDeltaSchemaConstrainsFocusIDsOneToOne(t *testing.T) {
	data, err := StructuredOutputSchemaWithOptions(ContractAnalyzeCurrentDeltaBatch, StructuredOutputOptions{
		FocusIDs: []string{"frontend-routing", "auth", "auth"},
	})
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	changes := schema["properties"].(map[string]any)["knowledge_changes"].(map[string]any)
	require.Equal(t, float64(2), changes["minItems"])
	require.Equal(t, float64(2), changes["maxItems"])
	require.Equal(t, true, changes["uniqueItems"])
	focusID, _, ok := findSchemaPropertyWithContainer(schema, "focus_id")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"auth", "frontend-routing"}, schemaStringList(focusID["enum"]))
}

func TestCodebaseBatchSchemaConstrainsFocusIDsOneToOne(t *testing.T) {
	data, err := StructuredOutputSchemaWithOptions(ContractAnalyzeCurrentCodebaseBatch, StructuredOutputOptions{
		FocusIDs: []string{"frontend-routing", "auth", "auth"},
	})
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	focuses := schema["properties"].(map[string]any)["focuses"].(map[string]any)
	require.Equal(t, float64(2), focuses["minItems"])
	require.Equal(t, float64(2), focuses["maxItems"])
	require.Equal(t, true, focuses["uniqueItems"])
	focusID, _, ok := findSchemaPropertyWithContainer(schema, "focus_id")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"auth", "frontend-routing"}, schemaStringList(focusID["enum"]))
}

func TestKnowledgeReviewSchemaConstrainsEmptyCandidateSet(t *testing.T) {
	data, err := StrictStructuredOutputSchemaWithOptions(ContractReviewKnowledge, StructuredOutputOptions{
		CandidateIDs: []string{},
	})
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	decisions := schema["properties"].(map[string]any)["decisions"].(map[string]any)
	require.Equal(t, float64(0), decisions["minItems"])
	require.Equal(t, float64(0), decisions["maxItems"])
}

func TestAuthorityExtractionSchemaConstrainsSectionIDsOneToOne(t *testing.T) {
	data, err := StructuredOutputSchemaWithOptions(ContractAuthorityExtraction, StructuredOutputOptions{
		AuthoritySectionIDs: []string{"authority-b", "authority-a", "authority-a"},
	})
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	sections := schema["properties"].(map[string]any)["authority_sections"].(map[string]any)
	require.Equal(t, float64(2), sections["minItems"])
	require.Equal(t, float64(2), sections["maxItems"])
	require.Equal(t, true, sections["uniqueItems"])
	sectionID, _, ok := findSchemaPropertyWithContainer(schema, "section_id")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"authority-a", "authority-b"}, schemaStringList(sectionID["enum"]))
}

func TestAuthorityExtractionSchemaConstrainsEmptySectionSet(t *testing.T) {
	data, err := StrictStructuredOutputSchemaWithOptions(ContractAuthorityExtraction, StructuredOutputOptions{
		AuthoritySectionIDs: []string{},
	})
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	sections := schema["properties"].(map[string]any)["authority_sections"].(map[string]any)
	require.Equal(t, float64(0), sections["minItems"])
	require.Equal(t, float64(0), sections["maxItems"])
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

func TestResourceOptimizationSchemasRequireContentAndAllowRoutingMetadata(t *testing.T) {
	for _, contract := range []string{ContractOptimizeWorkflow, ContractOptimizeRule} {
		schema := decodeSchema(t, contract)
		properties := schema["properties"].(map[string]any)
		require.Len(t, properties, 3)
		require.Contains(t, properties, "content")
		require.Contains(t, properties, "summary")
		require.Contains(t, properties, "route_terms")
		require.Equal(t, []string{"content"}, schemaStringList(schema["required"]))
	}
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

func decodeStrictSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := StrictStructuredOutputSchema(name)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &schema))
	return schema
}

func requireStrictRequiredProperties(t *testing.T, schema map[string]any, name string) {
	t.Helper()
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) > 0 {
		names := make([]string, 0, len(properties))
		for propertyName := range properties {
			names = append(names, propertyName)
		}
		require.ElementsMatch(t, names, schemaStringList(schema["required"]), name)
		for _, property := range properties {
			propertySchema, ok := property.(map[string]any)
			require.True(t, ok, name)
			requireStrictRequiredProperties(t, propertySchema, name)
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		requireStrictRequiredProperties(t, items, name)
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		variants, _ := schema[keyword].([]any)
		for _, variant := range variants {
			if variantSchema, ok := variant.(map[string]any); ok {
				requireStrictRequiredProperties(t, variantSchema, name)
			}
		}
	}
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
