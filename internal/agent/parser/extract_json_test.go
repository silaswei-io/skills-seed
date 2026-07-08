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

func TestParseAnalyzeCurrentCodebaseBatchResultKeepsTopLevelUnits(t *testing.T) {
	output := `{
  "units": [
    {
      "unit_id": "auth-login-flow",
      "unit_name": "认证登录流程",
      "patterns": [
        {
          "id": "login-failure-lock-mechanism",
          "name": "登录失败锁定机制",
          "category": "business",
          "description": "登录失败后锁定账号",
          "good_example": "func loginFailed() error {\n  return nil\n}",
          "bad_example": "",
          "rule": "登录失败达到阈值时锁定账号",
          "confidence": 0.9,
          "frequency": 1,
          "analysis_unit_id": "auth-login-flow",
          "analysis_unit_name": "认证登录流程"
        }
      ],
      "profile_delta": {
        "layers": [
          {
            "name": "服务层",
            "description": "核心登录业务逻辑",
            "responsibilities": ["密码验证"],
            "files": ["internal/service/system/admin/login.go"]
          }
        ]
      },
      "profile_refresh_recommended": {"needed": false, "reason": ""}
    }
  ]
}`

	result, err := ParseAnalyzeCurrentCodebaseBatchResult(output)

	require.NoError(t, err)
	require.Len(t, result.Units, 1)
	assert.Equal(t, "auth-login-flow", result.Units[0].UnitID)
	assert.Equal(t, "认证登录流程", result.Units[0].UnitName)
	require.Len(t, result.Units[0].Patterns, 1)
	assert.Equal(t, "login-failure-lock-mechanism", result.Units[0].Patterns[0].ID)
	require.Len(t, result.Units[0].ProfileDelta.Layers, 1)
	assert.Equal(t, "服务层", result.Units[0].ProfileDelta.Layers[0].Name)
}

func TestParseWorkspaceSpecParsesStringChangeOrder(t *testing.T) {
	output := `{
	  "name": "hsm-workspace",
	  "root_path": "/workspace",
	  "routing": [],
	  "rules": [],
	  "change_order": [
	    "1. 确认契约或共享接口稳定：修改 proto、API、SDK 前先确认兼容性。",
	    "2. 更新消费方：同步适配依赖方。"
	  ]
	}`

	result, err := ParseWorkspaceSpec(output)
	require.NoError(t, err)
	require.Equal(t, []string{
		"1. 确认契约或共享接口稳定：修改 proto、API、SDK 前先确认兼容性。",
		"2. 更新消费方：同步适配依赖方。",
	}, result.ChangeOrder)
}

func TestParseWorkspaceSpecRejectsObjectChangeOrder(t *testing.T) {
	output := `{
	  "name": "hsm-workspace",
	  "root_path": "/workspace",
	  "routing": [],
	  "rules": [],
	  "change_order": [
	    {"step": 1, "action": "确认契约或共享接口稳定", "details": "修改 proto、API、SDK 前先确认兼容性。"}
	  ]
	}`

	result, err := ParseWorkspaceSpec(output)

	require.Error(t, err)
	require.Nil(t, result)
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

func TestParseAnalyzeProjectResult_RepairsNonstandardJSON(t *testing.T) {
	output := `{
  // project profile returned by model
  project_name: 'demo',
  language: 'go',
  frameworks: ['cobra',],
  architecture: 'layered',
  layers: [],
  dependency_graph: 'command -> service',
  data_flow: 'request -> response',
  framework_patterns: [],
  structure: 'internal/',
  key_modules: [],
  business_methods: [],
  common_utils: [
    {name:'RawFieldNames', file:'core/stores/condition/adaptor.go', signature:'func RawFieldNames(in any) []string', description:'从结构体提取 db tag 对应的字段名列表', usage:'将 Go 结构体字段转为数据库列名列表'},
  ],
  config_patterns: [],
  dependencies: [],
  summary: 'demo project',
}`

	result, err := ParseAnalyzeProjectResult(output)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "demo", result.ProjectName)
	assert.Equal(t, []string{"cobra"}, result.Frameworks)
	assert.Len(t, result.CommonUtils, 1)
	assert.Equal(t, "RawFieldNames", result.CommonUtils[0].Name)
}

func TestParseAnalyzeProjectResultRejectsSemanticTypeMismatch(t *testing.T) {
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
  "common_utils": [],
  "config_patterns": [],
  "dependencies": [],
  "validation_commands": ["go test ./..."],
  "summary": "demo project"
}`

	result, err := ParseAnalyzeProjectResult(output)

	require.Error(t, err)
	require.Nil(t, result)
}

func TestParseAnalyzeCurrentCodebaseResultRejectsStringCodeLocation(t *testing.T) {
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
      "code_location": "internal/service/demo.go:10",
      "description": "runs demo workflow",
      "usage": "demo flow",
      "type": "domain",
      "function": "func Run() error"
    }
  }],
  "profile_delta": {},
  "profile_refresh_recommended": {"needed": false}
}`

	result, err := ParseAnalyzeCurrentCodebaseResult(output)

	require.Error(t, err)
	require.Nil(t, result)
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
  "profile_delta": {
    "summary": "demo project",
    "business_methods": [{
      "name": "Run",
      "code_location": {"current_location":"internal/service/demo.go:10"},
      "description": "runs demo workflow",
      "usage": "demo flow",
      "type": "domain",
      "function": "func Run() error",
      "prerequisites": "config loaded",
      "returns": "error"
    }]
  },
  "profile_refresh_recommended": {"needed": false}
}`

	result, err := ParseAnalyzeCurrentCodebaseResult(output)
	assert.NoError(t, err)
	assert.Len(t, result.Patterns, 1)
	assert.NotNil(t, result.Patterns[0].BusinessMethod)
	assert.Equal(t, "internal/service/demo.go:10", result.Patterns[0].BusinessMethod.DisplayLocation())
	assert.Equal(t, "config loaded", result.Patterns[0].BusinessMethod.Prerequisites)
	assert.Equal(t, "error", result.Patterns[0].BusinessMethod.Returns)
	assert.Equal(t, "demo project", result.ProfileDelta.Summary)
	assert.Len(t, result.ProfileDelta.BusinessMethods, 1)
}

func TestParseAnalyzeCurrentCodebaseResult_WithEvidenceLocations(t *testing.T) {
	output := `{
  "patterns": [{
    "id": "error-wrap",
    "name": "Error Wrap",
    "category": "error",
    "description": "wraps errors with operation context",
    "good_example": "return fmt.Errorf(\"load config: %w\", err)",
    "bad_example": "",
    "rule": "Wrap errors at module boundaries",
    "confidence": 0.9,
    "frequency": 1,
    "analysis_unit_id": "auth",
    "analysis_unit_name": "Authentication",
    "evidence_locations": [
      {
        "path": "internal/service/config.go",
        "line": 42,
        "symbol": "LoadConfig",
        "kind": "function",
        "description": "wraps config load error",
        "confidence": 0.88
      }
    ],
    "business_method": null
  }],
  "profile_delta": {},
  "profile_refresh_recommended": {"needed": false}
}`

	result, err := ParseAnalyzeCurrentCodebaseResult(output)
	assert.NoError(t, err)
	assert.Len(t, result.Patterns, 1)
	assert.Nil(t, result.Patterns[0].BusinessMethod)
	assert.Len(t, result.Patterns[0].EvidenceLocations, 1)
	assert.Equal(t, "auth", result.Patterns[0].AnalysisUnitID)
	assert.Equal(t, "Authentication", result.Patterns[0].AnalysisUnitName)
	assert.Equal(t, "internal/service/config.go", result.Patterns[0].EvidenceLocations[0].Path)
	assert.Equal(t, 42, result.Patterns[0].EvidenceLocations[0].Line)
	assert.Equal(t, "LoadConfig", result.Patterns[0].EvidenceLocations[0].Symbol)
	assert.Equal(t, "internal/service/config.go:42", result.Patterns[0].EvidenceLocations[0].DisplayLocation())
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
