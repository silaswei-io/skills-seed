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
	ContractAnalyzeCurrentCodebaseBatch = "AnalyzeCurrentCodebaseBatchOutput"
	ContractAnalyzeCurrentDeltaBatch    = "AnalyzeCurrentDeltaBatchOutput"
	ContractSelectLearningCandidates    = "SelectLearningCandidatesOutput"
	ContractLearningSessionAck          = "LearningSessionAckOutput"
	ContractPlanLearningAgenda          = "PlanLearningAgendaOutput"
	ContractWorkspaceProfile            = "WorkspaceProfileOutput"
	ContractWorkspaceSpec               = "WorkspaceSpecOutput"
	ContractOptimizeWorkflow            = "OptimizeWorkflowOutput"
)

var outputTypes = map[string]reflect.Type{
	ContractUserDefinePattern:           reflect.TypeOf(PatternOutput{}),
	ContractProjectProfile:              reflect.TypeOf(ProjectProfileOutput{}),
	ContractAnalyzeCurrentCodebaseBatch: reflect.TypeOf(AnalyzeCurrentCodebaseBatchOutput{}),
	ContractAnalyzeCurrentDeltaBatch:    reflect.TypeOf(AnalyzeCurrentDeltaBatchOutput{}),
	ContractSelectLearningCandidates:    reflect.TypeOf(SelectLearningCandidatesOutput{}),
	ContractLearningSessionAck:          reflect.TypeOf(LearningSessionAckOutput{}),
	ContractPlanLearningAgenda:          reflect.TypeOf(PlanLearningAgendaOutput{}),
	ContractWorkspaceProfile:            reflect.TypeOf(WorkspaceProfileOutput{}),
	ContractWorkspaceSpec:               reflect.TypeOf(WorkspaceSpecOutput{}),
	ContractOptimizeWorkflow:            reflect.TypeOf(OptimizeWorkflowOutput{}),
}

// StructuredOutputOptions 表示一次 Agent 调用可动态收窄的输出契约。
type StructuredOutputOptions struct {
	// ProjectIDs 是工作区模式下配置声明的唯一合法子项目 ID。
	ProjectIDs []string
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
	if len(projectIDs) == 0 || (name != ContractWorkspaceProfile && name != ContractWorkspaceSpec) {
		return data, nil
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(data), &schema); err != nil {
		return "", err
	}
	applyProjectIDEnum(schema, projectIDs)
	constrained, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(constrained), nil
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
