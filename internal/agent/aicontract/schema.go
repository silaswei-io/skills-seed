package aicontract

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/invopop/jsonschema"
)

var outputTypes = map[string]reflect.Type{
	"AnalyzeCodeOutput":                 reflect.TypeOf(AnalyzeCodeOutput{}),
	"GenerateFixesOutput":               reflect.TypeOf(GenerateFixesOutput{}),
	"LearnPatternsOutput":               reflect.TypeOf(LearnPatternsOutput{}),
	"CuratePatternsOutput":              reflect.TypeOf(CuratePatternsOutput{}),
	"UserDefinePatternOutput":           reflect.TypeOf(PatternOutput{}),
	"ProjectProfileOutput":              reflect.TypeOf(ProjectProfileOutput{}),
	"AnalyzeCurrentCodebaseOutput":      reflect.TypeOf(AnalyzeCurrentCodebaseOutput{}),
	"AnalyzeCurrentCodebaseBatchOutput": reflect.TypeOf(AnalyzeCurrentCodebaseBatchOutput{}),
	"PlanAnalysisUnitsOutput":           reflect.TypeOf(PlanAnalysisUnitsOutput{}),
	"SelectFilesOutput":                 reflect.TypeOf(SelectFilesOutput{}),
	"WorkspaceProfileOutput":            reflect.TypeOf(WorkspaceProfileOutput{}),
	"WorkspaceSpecOutput":               reflect.TypeOf(WorkspaceSpecOutput{}),
	"OptimizeWorkflowOutput":            reflect.TypeOf(OptimizeWorkflowOutput{}),
}

// JSONSchema 返回指定 AI 输出 DTO 的 JSON Schema。
func JSONSchema(name string) (string, error) {
	t, ok := outputTypes[name]
	if !ok {
		return "", fmt.Errorf("unknown AI output contract %q", name)
	}
	reflector := jsonschema.Reflector{
		Anonymous:      true,
		DoNotReference: true,
		ExpandedStruct: true,
	}
	schema := reflector.ReflectFromType(t)
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
