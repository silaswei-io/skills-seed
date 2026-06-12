package parser

import (
	"os"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := i18n.Init("zh-CN"); err != nil {
		_ = err
	}
	os.Exit(m.Run())
}

// ==================== ExtractJSON 测试 ====================

func TestExtractJSON_PlainJSON(t *testing.T) {
	input := `{"key": "value"}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": "value"}`, result)
}

func TestExtractJSON_JSONInCodeBlock(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": "value"}`, result)
}

func TestExtractJSON_EmptyOutput(t *testing.T) {
	result, err := ExtractJSON("")
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_WhitespaceOnly(t *testing.T) {
	result, err := ExtractJSON("   \n\t  ")
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_NoBraces(t *testing.T) {
	result, err := ExtractJSON("no json here")
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_UnmatchedBraces(t *testing.T) {
	input := `{"key": "value",`
	result, err := ExtractJSON(input)
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_InvalidJSON(t *testing.T) {
	input := `{not valid}`
	result, err := ExtractJSON(input)
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_NestedJSON(t *testing.T) {
	input := `{"outer": {"inner": 1}}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"outer": {"inner": 1}}`, result)
}

func TestExtractJSON_JSONWithTextBefore(t *testing.T) {
	input := `some text {"key": 1}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": 1}`, result)
}

func TestExtractJSON_JSONWithTextAfter(t *testing.T) {
	input := `{"key": 1} some text`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": 1}`, result)
}

func TestExtractJSON_EscapedStrings(t *testing.T) {
	input := `{"msg": "hello \"world\""}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"msg": "hello \"world\""}`, result)
}

func TestExtractJSON_RepairsInvalidBackslashEscapesInStrings(t *testing.T) {
	input := `{"good_example": "const path = \"src\ pages\"\nconst re = /\s+/"}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"good_example": "const path = \"src\\ pages\"\nconst re = /\\s+/"}`, result)
}

func TestExtractJSON_RepairsUnescapedQuotesInsideCodeStrings(t *testing.T) {
	input := `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf("%s", "admin")","bad_example":""}]}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s\", \"admin\")","bad_example":""}]}`, result)
}

func TestExtractJSON_RepairsPartiallyEscapedQuotesInsideCodeStrings(t *testing.T) {
	input := `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】", resp.Extra.Object, req.Username)","bad_example":""}]}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】\", resp.Extra.Object, req.Username)","bad_example":""}]}`, result)
}

func TestFixAIJSON_FixesCommonAIJSONDefectsTogether(t *testing.T) {
	input := `{{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】", resp.Extra.Object, req.Username)","bad_example":"const path = \"src\ pages\""}]`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】\", resp.Extra.Object, req.Username)","bad_example":"const path = \"src\\ pages\""}]}`, result)
}

func TestExtractJSON_RepairsMissingClosingContainersAtEnd(t *testing.T) {
	input := `{"patterns":[{"id":"service","name":"Service"}],"category_summaries":{"structure":{"summary":"layers","patterns":["Service"]}}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"service","name":"Service"}],"category_summaries":{"structure":{"summary":"layers","patterns":["Service"]}}}`, result)
}

func TestExtractJSON_DoesNotRepairUnterminatedString(t *testing.T) {
	input := `{"patterns":[{"id":"service","name":"Service}]`
	result, err := ExtractJSON(input)
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_CodeBlockWithNestedJSON(t *testing.T) {
	input := "```json\n{\"outer\": {\"inner\": [1, 2, 3]}}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"outer": {"inner": [1, 2, 3]}}`, result)
}

func TestExtractJSON_ArrayInCodeBlock(t *testing.T) {
	input := "```json\n[1, 2, 3]\n```"
	_, err := ExtractJSON(input)
	assert.Error(t, err)
}

func TestExtractJSON_MultipleCodeBlocks(t *testing.T) {
	input := "```\nsome code\n```\n```json\n{\"key\": 1}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": 1}`, result)
}

func TestParseWorkspaceSpecAcceptsObjectChangeOrder(t *testing.T) {
	output := `{
	  "name": "hsm-workspace",
	  "root_path": "/workspace",
	  "routing": [],
	  "rules": [],
	  "change_order": [
	    {
	      "step": 1,
	      "action": "确认契约或共享接口稳定",
	      "details": "修改 proto、API、SDK 前先确认兼容性。"
	    },
	    {
	      "action": "更新消费方",
	      "details": "同步适配依赖方。"
	    }
	  ]
	}`

	result, err := ParseWorkspaceSpec(output)
	require.NoError(t, err)
	require.Equal(t, []string{
		"1. 确认契约或共享接口稳定：修改 proto、API、SDK 前先确认兼容性。",
		"更新消费方：同步适配依赖方。",
	}, result.ChangeOrder)
}

func TestParseAnalyzeProjectResult_FullSchema(t *testing.T) {
	output := `{
	  "project_name": "demo",
  "language": "go",
  "frameworks": ["cobra"],
  "architecture": "layered",
  "layers": [{"name":"service","description":"business","responsibilities":["orchestrate"],"files":["internal/service/demo.go"]}],
  "dependency_graph": "command -> service -> domain",
  "data_flow": "request -> service -> repository",
  "framework_patterns": ["cobra command wiring"],
  "structure": "internal/",
  "key_modules": [{"name":"service","path":"internal/service","description":"business layer","responsibilities":["orchestrate"],"dependencies":["domain"],"dependents":["command"],"key_methods":["Run()"]}],
  "business_methods": [{"name":"Run","code_location":{"current_location":"internal/service/demo.go:10"},"description":"runs demo","usage":"demo flow","type":"domain","function":"func Run() error","prerequisites":"config loaded","returns":"error"}],
  "common_utils": [{"name":"Ptr","file":"internal/utils/ptr.go","signature":"func Ptr[T any](v T) *T","description":"returns pointer","usage":"optional fields"}],
  "config_patterns": ["yaml config"],
  "dependencies": ["bbolt"],
  "validation_commands": [{"command":"task verify","when":"after changing project code","source":"Taskfile.yml"}],
  "summary": "demo project"
}`

	result, err := ParseAnalyzeProjectResult(output)
	assert.NoError(t, err)
	assert.Equal(t, "demo", result.ProjectName)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "command -> service -> domain", result.DependencyGraph)
	assert.Len(t, result.Layers, 1)
	assert.Len(t, result.KeyModules, 1)
	assert.Equal(t, []string{"domain"}, result.KeyModules[0].Dependencies)
	assert.Len(t, result.BusinessMethods, 1)
	assert.Equal(t, "internal/service/demo.go:10", result.BusinessMethods[0].DisplayLocation())
	assert.Equal(t, "func Ptr[T any](v T) *T", result.CommonUtils[0].Signature)
	require.Len(t, result.ValidationCommands, 1)
	assert.Equal(t, "task verify", result.ValidationCommands[0].Command)
	assert.Equal(t, "Taskfile.yml", result.ValidationCommands[0].Source)
}

func TestParseAnalyzeProjectResult_RepairsDuplicatedObjectStart(t *testing.T) {
	output := `{
  "project_name": "demo",
  "language": "go",
  "frameworks": ["cobra"],
  "architecture": "layered",
  "layers": [],
  "dependency_graph": "command -> service",
  "data_flow": "request -> response",
  "framework_patterns": [],
  "structure": "internal/",
  "key_modules": [],
  "business_methods": [],
  "common_utils": [
    {"name":"QuoteWithFlavor","file":"core/stores/condition/adaptor.go","signature":"func QuoteWithFlavor(flavor sqlbuilder.Flavor, str string) string","description":"按数据库方言对字段名加引号","usage":"生成 SQL 时确保字段名按方言正确引用"},
    {"{"name":"RawFieldNames","file":"core/stores/condition/adaptor.go","signature":"func RawFieldNames(in any) []string","description":"从结构体提取 db tag 对应的字段名列表","usage":"将 Go 结构体字段转为数据库列名列表"}
  ],
  "config_patterns": [],
  "dependencies": [],
  "summary": "demo project"
}`

	result, err := ParseAnalyzeProjectResult(output)
	if !assert.NoError(t, err) {
		return
	}
	assert.Len(t, result.CommonUtils, 2)
	assert.Equal(t, "RawFieldNames", result.CommonUtils[1].Name)
}

func TestParseAnalyzeCurrentCodebaseResult_WithBusinessMethod(t *testing.T) {
	output := `{
  "patterns": [{
    "id": "business-run",
    "name": "Business Run",
    "category": "business",
    "description": "runs a business workflow",
    "good_example": "func Run() error {\n  return nil\n}",
    "bad_example": "",
    "rule": "Use for business orchestration",
    "confidence": 0.9,
    "frequency": 1,
    "business_method": {
      "name": "Run",
      "code_location": {"current_location":"internal/service/demo.go:10"},
      "description": "runs demo workflow",
      "usage": "demo flow",
      "type": "domain",
      "function": "func Run() error",
      "prerequisites": "config loaded",
      "returns": "error"
    }
  }],
  "category_summaries": {
    "business": {"summary": "business workflows", "patterns": ["Business Run"], "priority": 5}
  },
  "business_rules": ["validate before run"],
  "best_practices": ["wrap errors"],
  "common_patterns": ["service orchestration"],
  "summary": "demo project"
}`

	result, err := ParseAnalyzeCurrentCodebaseResult(output)
	assert.NoError(t, err)
	assert.Len(t, result.Patterns, 1)
	assert.NotNil(t, result.Patterns[0].BusinessMethod)
	assert.Equal(t, "internal/service/demo.go:10", result.Patterns[0].BusinessMethod.DisplayLocation())
	assert.Equal(t, "config loaded", result.Patterns[0].BusinessMethod.Prerequisites)
	assert.Equal(t, "error", result.Patterns[0].BusinessMethod.Returns)
}

func TestParseAnalyzeResultReturnsErrorWhenJSONMissing(t *testing.T) {
	result, err := ParseAnalyzeResult("plain text without json")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestParseLearnResultReturnsErrorWhenJSONMissing(t *testing.T) {
	result, err := ParseLearnResult("plain text without json")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestParseBatchLearnResultReturnsErrorWhenJSONMissing(t *testing.T) {
	result, err := ParseBatchLearnResult("plain text without json")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// ==================== findMatchingBrace 测试 ====================

func TestFindMatchingBrace_Simple(t *testing.T) {
	input := `{"a":1}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 6, end, "should find matching brace at position 6")
}

func TestFindMatchingBrace_Nested(t *testing.T) {
	input := `{"a":{"b":2}}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 12, end, "should find matching brace at position 12")
}

func TestFindMatchingBrace_NoMatch(t *testing.T) {
	input := `{"a":1`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, -1, end, "should return -1 for unmatched brace")
}

func TestFindMatchingBrace_WithStrings(t *testing.T) {
	input := `{"a":"}{"}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 9, end, "braces inside strings should be ignored")
}

func TestFindMatchingBrace_EscapedQuoteInString(t *testing.T) {
	input := `{"a":"he said \"hello\""}`
	end := findMatchingBrace(input, 0)
	assert.NotEqual(t, -1, end, "should find matching brace with escaped quotes")
	extracted := input[0 : end+1]
	assert.JSONEq(t, `{"a":"he said \"hello\""}`, extracted)
}

func TestFindMatchingBrace_DeeplyNested(t *testing.T) {
	input := `{"a":{"b":{"c":1}}}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 18, end, "should find matching brace for deeply nested object")
}

func TestFindMatchingBrace_StartFromMiddle(t *testing.T) {
	input := `prefix {"inner": 1} suffix`
	start := 7
	end := findMatchingBrace(input, start)
	assert.Equal(t, 18, end, "should find matching brace when starting from middle")
}

func TestFindMatchingBrace_EmptyObject(t *testing.T) {
	input := `{}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 1, end, "should find matching brace for empty object")
}

func TestFindMatchingBrace_NoOpeningBrace(t *testing.T) {
	input := `no braces here`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, -1, end, "should return -1 when no opening brace at start position")
}

func TestFindMatchingBrace_WithArray(t *testing.T) {
	input := `{"arr": [1, 2, 3]}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 17, end, "should correctly skip array brackets")
}

func TestFindMatchingBrace_EscapedBackslash(t *testing.T) {
	input := `{"path": "C:\\Users\\"}`
	end := findMatchingBrace(input, 0)
	assert.NotEqual(t, -1, end, "should handle escaped backslashes correctly")
}
