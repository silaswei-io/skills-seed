package aicontract

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/invopop/jsonschema"
)

// AI 输出契约名称统一绑定 DTO，供提示词和 Agent CLI 复用同一份 Schema。
const (
	ContractAnalyzeCode                 = "AnalyzeCodeOutput"
	ContractGenerateFixes               = "GenerateFixesOutput"
	ContractLearnPatterns               = "LearnPatternsOutput"
	ContractCuratePatterns              = "CuratePatternsOutput"
	ContractUserDefinePattern           = "UserDefinePatternOutput"
	ContractProjectProfile              = "ProjectProfileOutput"
	ContractAnalyzeCurrentCodebase      = "AnalyzeCurrentCodebaseOutput"
	ContractAnalyzeCurrentCodebaseBatch = "AnalyzeCurrentCodebaseBatchOutput"
	ContractPlanAnalysisUnits           = "PlanAnalysisUnitsOutput"
	ContractSelectFiles                 = "SelectFilesOutput"
	ContractWorkspaceProfile            = "WorkspaceProfileOutput"
	ContractWorkspaceSpec               = "WorkspaceSpecOutput"
	ContractOptimizeWorkflow            = "OptimizeWorkflowOutput"
)

var outputTypes = map[string]reflect.Type{
	ContractAnalyzeCode:                 reflect.TypeOf(AnalyzeCodeOutput{}),
	ContractGenerateFixes:               reflect.TypeOf(GenerateFixesOutput{}),
	ContractLearnPatterns:               reflect.TypeOf(LearnPatternsOutput{}),
	ContractCuratePatterns:              reflect.TypeOf(CuratePatternsOutput{}),
	ContractUserDefinePattern:           reflect.TypeOf(PatternOutput{}),
	ContractProjectProfile:              reflect.TypeOf(ProjectProfileOutput{}),
	ContractAnalyzeCurrentCodebase:      reflect.TypeOf(AnalyzeCurrentCodebaseOutput{}),
	ContractAnalyzeCurrentCodebaseBatch: reflect.TypeOf(AnalyzeCurrentCodebaseBatchOutput{}),
	ContractPlanAnalysisUnits:           reflect.TypeOf(PlanAnalysisUnitsOutput{}),
	ContractSelectFiles:                 reflect.TypeOf(SelectFilesOutput{}),
	ContractWorkspaceProfile:            reflect.TypeOf(WorkspaceProfileOutput{}),
	ContractWorkspaceSpec:               reflect.TypeOf(WorkspaceSpecOutput{}),
	ContractOptimizeWorkflow:            reflect.TypeOf(OptimizeWorkflowOutput{}),
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
	schema, err := reflectSchema(name)
	if err != nil {
		return "", err
	}
	schema.Version = ""
	return marshalSchema(schema)
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
