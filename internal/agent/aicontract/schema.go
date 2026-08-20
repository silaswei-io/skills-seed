package aicontract

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/invopop/jsonschema"
)

// AI 输出契约名称统一绑定 DTO，供提示词和 Agent CLI 复用同一份 Schema。
const (
	ContractUserDefinePattern           = "UserDefinePatternOutput"
	ContractProjectProfile              = "ProjectProfileOutput"
	ContractAuthorityExtraction         = "AuthorityExtractionOutput"
	ContractAnalyzeCurrentCodebaseBatch = "AnalyzeCurrentCodebaseBatchOutput"
	ContractAnalyzeCurrentDeltaBatch    = "AnalyzeCurrentDeltaBatchOutput"
	ContractPlanLearningAgenda          = "PlanLearningAgendaOutput"
	ContractReviewKnowledge             = "ReviewKnowledgeOutput"
	ContractNormalizePatterns           = "NormalizePatternsOutput"
	ContractWorkspaceProfile            = "WorkspaceProfileOutput"
	ContractWorkspaceSpec               = "WorkspaceSpecOutput"
	ContractOptimizeWorkflow            = "OptimizeWorkflowOutput"
	ContractOptimizeRule                = "OptimizeRuleOutput"
)

var outputTypes = map[string]reflect.Type{
	ContractUserDefinePattern:           reflect.TypeOf(PatternOutput{}),
	ContractProjectProfile:              reflect.TypeOf(ProjectProfileOutput{}),
	ContractAuthorityExtraction:         reflect.TypeOf(AuthorityExtractionOutput{}),
	ContractAnalyzeCurrentCodebaseBatch: reflect.TypeOf(AnalyzeCurrentCodebaseBatchOutput{}),
	ContractAnalyzeCurrentDeltaBatch:    reflect.TypeOf(AnalyzeCurrentDeltaBatchOutput{}),
	ContractPlanLearningAgenda:          reflect.TypeOf(PlanLearningAgendaOutput{}),
	ContractReviewKnowledge:             reflect.TypeOf(ReviewKnowledgeOutput{}),
	ContractNormalizePatterns:           reflect.TypeOf(NormalizePatternsOutput{}),
	ContractWorkspaceProfile:            reflect.TypeOf(WorkspaceProfileOutput{}),
	ContractWorkspaceSpec:               reflect.TypeOf(WorkspaceSpecOutput{}),
	ContractOptimizeWorkflow:            reflect.TypeOf(OptimizedContentOutput{}),
	ContractOptimizeRule:                reflect.TypeOf(OptimizedContentOutput{}),
}

// StructuredOutputOptions 表示一次 Agent 调用可动态收窄的输出契约。
type StructuredOutputOptions struct {
	// ProjectIDs 是工作区模式下配置声明的唯一合法子项目 ID。
	ProjectIDs []string
	// CandidateIDs 是知识审查本次调用允许返回的候选 ID。
	// 非 nil 时会将审查结果收窄为一对一的候选回执。
	CandidateIDs []string
}

// JSONSchema 返回指定 AI 输出 DTO 的 JSON Schema。
func JSONSchema(name string) (string, error) {
	schema, err := reflectSchema(name)
	if err != nil {
		return "", err
	}
	return marshalSchema(schema)
}

// StructuredOutputSchema 返回 Agent CLI 可直接校验的 DTO Schema。
// CLI 自带校验器不一定加载 Draft 2020-12 meta-schema，因此不传递版本声明。
func StructuredOutputSchema(name string) (string, error) {
	return StructuredOutputSchemaWithOptions(name, StructuredOutputOptions{})
}

// StrictStructuredOutputSchema 返回兼容严格结构化输出提供方的 DTO Schema。
// 严格提供方要求每个对象属性均显式出现在 required 中；原本可省略的字段
// 会保留为可空值或空集合，避免改变 DTO 的领域语义。
func StrictStructuredOutputSchema(name string) (string, error) {
	return StrictStructuredOutputSchemaWithOptions(name, StructuredOutputOptions{})
}

// StructuredOutputSchemaWithOptions 返回按本次调用上下文收窄后的 Agent 输出 Schema。
func StructuredOutputSchemaWithOptions(name string, opts StructuredOutputOptions) (string, error) {
	schema, err := reflectSchema(name)
	if err != nil {
		return "", err
	}
	schema.Version = ""
	data, err := marshalSchema(schema)
	if err != nil {
		return "", err
	}
	return constrainStructuredOutputSchema(name, data, opts)
}

// StrictStructuredOutputSchemaWithOptions 返回满足严格提供方 required 约束的输出 Schema。
func StrictStructuredOutputSchemaWithOptions(name string, opts StructuredOutputOptions) (string, error) {
	data, err := StructuredOutputSchemaWithOptions(name, opts)
	if err != nil {
		return "", err
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(data), &schema); err != nil {
		return "", err
	}
	makeSchemaStrict(schema)
	strict, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(strict), nil
}

func marshalSchema(schema *jsonschema.Schema) (string, error) {
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func reflectSchema(name string) (*jsonschema.Schema, error) {
	t, ok := outputTypes[name]
	if !ok {
		return nil, fmt.Errorf("unknown AI output contract %q", name)
	}
	reflector := jsonschema.Reflector{
		Anonymous:      true,
		DoNotReference: true,
		ExpandedStruct: true,
	}
	schema := reflector.ReflectFromType(t)
	return schema, nil
}

func constrainStructuredOutputSchema(name, data string, opts StructuredOutputOptions) (string, error) {
	projectIDs := cleanSchemaEnumValues(opts.ProjectIDs)
	var schema map[string]any
	if len(projectIDs) > 0 && (name == ContractWorkspaceProfile || name == ContractWorkspaceSpec) {
		if err := json.Unmarshal([]byte(data), &schema); err != nil {
			return "", err
		}
		applyProjectIDEnum(schema, projectIDs)
	}
	candidateIDs := cleanSchemaEnumValues(opts.CandidateIDs)
	if name == ContractReviewKnowledge && opts.CandidateIDs != nil {
		if schema == nil {
			if err := json.Unmarshal([]byte(data), &schema); err != nil {
				return "", err
			}
		}
		applyCandidateReviewConstraints(schema, candidateIDs)
	}
	if schema == nil {
		return data, nil
	}
	constrained, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(constrained), nil
}

func applyCandidateReviewConstraints(schema map[string]any, candidateIDs []string) {
	properties := schemaProperties(schema)
	decisions, ok := properties["decisions"].(map[string]any)
	if !ok {
		return
	}
	count := len(candidateIDs)
	decisions["minItems"] = count
	decisions["maxItems"] = count
	decisions["uniqueItems"] = true
	if count == 0 {
		return
	}
	items, ok := decisions["items"].(map[string]any)
	if !ok {
		return
	}
	decisionProperties := schemaProperties(items)
	candidateID, ok := decisionProperties["candidate_id"].(map[string]any)
	if !ok {
		return
	}
	candidateID["enum"] = schemaEnumValues(candidateIDs)
}

func applyProjectIDEnum(schema map[string]any, projectIDs []string) {
	for name, property := range schemaProperties(schema) {
		prop, ok := property.(map[string]any)
		if !ok {
			continue
		}
		switch name {
		case "project_id", "from_project_id":
			prop["enum"] = schemaEnumValues(projectIDs)
		case "consumers", "producers", "affected_projects", "project_ids":
			if items, ok := prop["items"].(map[string]any); ok {
				items["enum"] = schemaEnumValues(projectIDs)
			}
		}
		applyProjectIDEnum(prop, projectIDs)
	}
	if items, ok := schema["items"].(map[string]any); ok {
		applyProjectIDEnum(items, projectIDs)
	}
}

func schemaProperties(schema map[string]any) map[string]any {
	properties, _ := schema["properties"].(map[string]any)
	return properties
}

// makeSchemaStrict 将常规 JSON Schema 投影为严格结构化输出方言。
func makeSchemaStrict(schema map[string]any) {
	properties := schemaProperties(schema)
	if len(properties) > 0 {
		required := schemaRequiredFields(schema)
		names := make([]string, 0, len(properties))
		for name, property := range properties {
			names = append(names, name)
			propertySchema, ok := property.(map[string]any)
			if !ok {
				continue
			}
			makeSchemaStrict(propertySchema)
			if !required[name] {
				makeSchemaPropertyNullable(propertySchema)
			}
		}
		slices.Sort(names)
		schema["required"] = schemaEnumValues(names)
	}

	if items, ok := schema["items"].(map[string]any); ok {
		makeSchemaStrict(items)
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		variants, _ := schema[keyword].([]any)
		for _, variant := range variants {
			if variantSchema, ok := variant.(map[string]any); ok {
				makeSchemaStrict(variantSchema)
			}
		}
	}
}

func schemaRequiredFields(schema map[string]any) map[string]bool {
	required := make(map[string]bool)
	values, _ := schema["required"].([]any)
	for _, value := range values {
		name, ok := value.(string)
		if ok {
			required[name] = true
		}
	}
	return required
}

func makeSchemaPropertyNullable(schema map[string]any) {
	addSchemaNullType(schema)
	if isSchemaArray(schema) {
		appendSchemaDescription(schema, "Return an empty array when no value applies; do not omit this key.")
		return
	}
	appendSchemaDescription(schema, "Return null when no value applies; do not omit this key.")
}

func addSchemaNullType(schema map[string]any) {
	switch value := schema["type"].(type) {
	case string:
		schema["type"] = []any{value, "null"}
	case []any:
		hasNull := false
		for _, item := range value {
			if item == "null" {
				hasNull = true
				break
			}
		}
		if !hasNull {
			schema["type"] = append(value, "null")
		}
	default:
		if enum, ok := schema["enum"].([]any); ok {
			addSchemaNullEnum(schema, enum)
			return
		}
		variants, _ := schema["anyOf"].([]any)
		schema["anyOf"] = append(variants, map[string]any{"type": "null"})
	}
	if enum, ok := schema["enum"].([]any); ok {
		addSchemaNullEnum(schema, enum)
	}
}

func addSchemaNullEnum(schema map[string]any, enum []any) {
	for _, value := range enum {
		if value == nil {
			return
		}
	}
	schema["enum"] = append(enum, nil)
}

func isSchemaArray(schema map[string]any) bool {
	switch value := schema["type"].(type) {
	case string:
		return value == "array"
	case []any:
		for _, item := range value {
			if item == "array" {
				return true
			}
		}
	}
	return false
}

func appendSchemaDescription(schema map[string]any, instruction string) {
	description, _ := schema["description"].(string)
	if description == "" {
		schema["description"] = instruction
		return
	}
	schema["description"] = description + " " + instruction
}

func cleanSchemaEnumValues(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func schemaEnumValues(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}
