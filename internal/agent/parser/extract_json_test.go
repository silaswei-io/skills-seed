package parser

import (
	"os"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
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

func TestParseAnalyzeCurrentCodebaseBatchResultKeepsTopLevelFocuses(t *testing.T) {
	output := `{
  "focuses": [
    {
      "focus_id": "auth-login-flow",
      "focus_name": "认证登录流程",
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
		  "knowledge_flags": ["operational_risk"]
        }
      ],
      "profile_refresh_recommended": {"needed": false, "reason": ""}
    }
  ]
}`

	result, err := ParseAnalyzeCurrentCodebaseBatchResult(output)

	require.NoError(t, err)
	require.Len(t, result.Focuses, 1)
	assert.Equal(t, "auth-login-flow", result.Focuses[0].FocusID)
	assert.Equal(t, "认证登录流程", result.Focuses[0].FocusName)
	require.Len(t, result.Focuses[0].Patterns, 1)
	assert.Equal(t, "login-failure-lock-mechanism", result.Focuses[0].Patterns[0].ID)
	assert.Equal(t, domain.SourceLearnedCurrent, result.Focuses[0].Patterns[0].Source)
	assert.Equal(t, []string{domain.KnowledgeFlagOperationalRisk}, result.Focuses[0].Patterns[0].KnowledgeFlags)
}

func TestParseOptimizeContentResultTrimsMarkdown(t *testing.T) {
	result, err := ParseOptimizeContentResult(`{"content":"  # Rule\n\nKeep the boundary.  "}`)

	require.NoError(t, err)
	require.Equal(t, "# Rule\n\nKeep the boundary.", result.Content)
}

func TestParsePlanLearningAgendaResultKeepsCoverageReceipt(t *testing.T) {
	result, err := ParsePlanLearningAgendaResult(`{
		"focuses":[{"id":"contract","name":"Contract","analysis_depth":"careful","entry_paths":["src/contract.ext"]}],
		"skipped_paths":[{"path":"src/generated.ext","reason":"Generated projection without source-of-truth value."}],
		"reason":"Keep source-owned decisions."
	}`)

	require.NoError(t, err)
	require.Len(t, result.Focuses, 1)
	require.Len(t, result.SkippedPaths, 1)
	require.Equal(t, "src/generated.ext", result.SkippedPaths[0].Path)
	require.Equal(t, "Keep source-owned decisions.", result.Reason)
}

func TestParseReviewKnowledgeResultKeepsRevision(t *testing.T) {
	result, err := ParseReviewKnowledgeResult(`{"decisions":[{
		"candidate_id":"bounded-behavior","verdict":"revise","reason_code":"overclaimed",
		"reason":"Narrow the claim.","business_method_verdict":"remove",
		"revision":{"name":"Bounded behavior","category":"business","description":"Observed locally.","rule":"Inspect before reuse.","confidence":0.86,"knowledge_flags":["operational_risk"]}
	}]}`)

	require.NoError(t, err)
	require.Len(t, result.Decisions, 1)
	require.NotNil(t, result.Decisions[0].Revision)
	require.Equal(t, 0.86, result.Decisions[0].Revision.Confidence)
	require.Equal(t, []string{domain.KnowledgeFlagOperationalRisk}, result.Decisions[0].Revision.KnowledgeFlags)
}

func TestParseReviewKnowledgeResultIgnoresInactiveConditionalFields(t *testing.T) {
	result, err := ParseReviewKnowledgeResult(`{"decisions":[{
		"candidate_id":"bounded-behavior","verdict":"accept","reason_code":"accepted",
		"reason":"The evidence supports the candidate.","business_method_verdict":"remove",
		"business_method":{"name":"Unused","code_location":{"current_location":"src/state.ext:24"},"description":"Unused.","usage":"Unused.","type":"domain","function":"Unused()","prerequisites":"None.","returns":"Nothing."},
		"revision":{"name":"Unused","category":"business","description":"Unused.","rule":"Unused.","confidence":0.5,"knowledge_flags":[]}
	}]}`)

	require.NoError(t, err)
	require.Len(t, result.Decisions, 1)
	require.Nil(t, result.Decisions[0].Revision)
	require.Nil(t, result.Decisions[0].BusinessMethod)
}

func TestParseReviewKnowledgeResultSetsBusinessMethod(t *testing.T) {
	result, err := ParseReviewKnowledgeResult(`{"decisions":[{
		"candidate_id":"state-transition","verdict":"accept","reason_code":"accepted",
		"reason":"The entry is directly evidenced.","business_method_verdict":"set",
		"business_method":{"name":"State.Transition","code_location":{"current_location":"src/state.ext:24"},"description":"Validates a transition.","usage":"Use for state changes.","type":"domain","function":"Transition(next State) error","prerequisites":"Allowed current and next states.","returns":"Nil or a validation error."}
	}]}`)

	require.NoError(t, err)
	require.Len(t, result.Decisions, 1)
	require.NotNil(t, result.Decisions[0].BusinessMethod)
	require.Equal(t, "State.Transition", result.Decisions[0].BusinessMethod.Name)
	require.Equal(t, "src/state.ext:24", result.Decisions[0].BusinessMethod.CodeLocation.CurrentLocation)
}

func TestParseWorkspaceSpecParsesStringChangeOrder(t *testing.T) {
	output := `{
		  "routing": [],
		  "rules": [],
		  "change_order": [
		    "确认契约或共享接口稳定：修改 proto、API、SDK 前先确认兼容性。",
		    "更新消费方：同步适配依赖方。"
	  ]
	}`

	result, err := ParseWorkspaceSpec(output)
	require.NoError(t, err)
	require.Equal(t, []string{
		"确认契约或共享接口稳定：修改 proto、API、SDK 前先确认兼容性。",
		"更新消费方：同步适配依赖方。",
	}, result.ChangeOrder)
}

func TestParseWorkspaceSpecRejectsObjectChangeOrder(t *testing.T) {
	output := `{
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
  "config_patterns": ["yaml config"],
  "dependencies": ["bbolt"],
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
}

func TestParseExtractAuthorityResult(t *testing.T) {
	result, err := ParseExtractAuthorityResult(`{
		"authority_sections": [{
			"section_id":"authority-contracts",
			"rules":[{"title":"Preserve contract","rule":"Keep public interfaces compatible.","applies_to":["public interfaces"]}]
		}]
	}`)

	require.NoError(t, err)
	require.Len(t, result.AuthoritySections, 1)
	assert.Equal(t, "authority-contracts", result.AuthoritySections[0].SectionID)
	require.Len(t, result.AuthoritySections[0].Rules, 1)
	assert.Equal(t, "Preserve contract", result.AuthoritySections[0].Rules[0].Title)
	assert.Empty(t, result.AuthoritySections[0].Rules[0].Source)
}

func TestParseAnalyzeProjectResultRepairsNonstandardJSON(t *testing.T) {
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
  config_patterns: [],
  dependencies: [],
  summary: 'demo project',
}`

	result, err := ParseAnalyzeProjectResult(output)

	require.NoError(t, err)
	require.Equal(t, "demo", result.ProjectName)
	require.Equal(t, []string{"cobra"}, result.Frameworks)
}

func TestParseAnalyzeProjectResultRejectsUnknownProfileField(t *testing.T) {
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
  "config_patterns": [],
  "dependencies": [],
  "unknown_field": ["unexpected"],
  "summary": "demo project"
}`

	result, err := ParseAnalyzeProjectResult(output)

	require.Error(t, err)
	require.Nil(t, result)
}

func TestParseAnalyzeCurrentCodebaseBatchResultRejectsStringCodeLocation(t *testing.T) {
	output := currentBatchOutput(`[
  {
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
  }
]`)

	result, err := ParseAnalyzeCurrentCodebaseBatchResult(output)

	require.Error(t, err)
	require.Nil(t, result)
}

func TestParseAnalyzeCurrentCodebaseBatchResultWithBusinessMethod(t *testing.T) {
	output := currentBatchOutput(`[
  {
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
  }
]`)

	result, err := ParseAnalyzeCurrentCodebaseBatchResult(output)
	assert.NoError(t, err)
	require.Len(t, result.Focuses, 1)
	require.Len(t, result.Focuses[0].Patterns, 1)
	pattern := result.Focuses[0].Patterns[0]
	assert.NotNil(t, pattern.BusinessMethod)
	assert.Equal(t, "internal/service/demo.go:10", pattern.BusinessMethod.DisplayLocation())
	assert.Equal(t, "config loaded", pattern.BusinessMethod.Prerequisites)
	assert.Equal(t, "error", pattern.BusinessMethod.Returns)
}

func TestParseAnalyzeCurrentCodebaseBatchResultWithEvidenceLocations(t *testing.T) {
	output := currentBatchOutput(`[
  {
    "id": "error-wrap",
    "name": "Error Wrap",
    "category": "error",
    "description": "wraps errors with operation context",
    "good_example": "return fmt.Errorf(\"load config: %w\", err)",
    "bad_example": "",
    "rule": "Wrap errors at module boundaries",
    "confidence": 0.9,
    "frequency": 1,
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
  }
]`)

	result, err := ParseAnalyzeCurrentCodebaseBatchResult(output)
	assert.NoError(t, err)
	require.Len(t, result.Focuses, 1)
	require.Len(t, result.Focuses[0].Patterns, 1)
	pattern := result.Focuses[0].Patterns[0]
	assert.Nil(t, pattern.BusinessMethod)
	assert.Len(t, pattern.EvidenceLocations, 1)
	assert.Equal(t, "internal/service/config.go", pattern.EvidenceLocations[0].Path)
	assert.Equal(t, 42, pattern.EvidenceLocations[0].Line)
	assert.Equal(t, "LoadConfig", pattern.EvidenceLocations[0].Symbol)
	assert.Equal(t, "internal/service/config.go:42", pattern.EvidenceLocations[0].DisplayLocation())
	assert.Equal(t, domain.SourceLearnedCurrent, pattern.Source)
}

func TestParseAnalyzeCurrentCodebaseBatchResultExtractsJSONFromText(t *testing.T) {
	output := `I now have a complete picture of this pack.

{
  "focuses": [
    {
      "focus_id": "auth",
      "focus_name": "Authentication",
      "patterns": [],
      "profile_refresh_recommended": {"needed": false}
    }
  ]
}`

	result, err := ParseAnalyzeCurrentCodebaseBatchResult(output)

	require.NoError(t, err)
	require.Len(t, result.Focuses, 1)
	assert.Equal(t, "auth", result.Focuses[0].FocusID)
}

func TestParseAnalyzeCurrentCodebaseBatchResultRejectsRemovedProfileDelta(t *testing.T) {
	result, err := ParseAnalyzeCurrentCodebaseBatchResult(`{
  "focuses": [
    {
      "focus_id": "auth",
      "focus_name": "Authentication",
      "patterns": [],
      "profile_delta": {},
      "profile_refresh_recommended": {"needed": false}
    }
  ]
}`)

	require.Error(t, err)
	require.Nil(t, result)
}

func TestParseAnalyzeCurrentCodebaseBatchResultRejectsMissingProfileRefreshRecommendation(t *testing.T) {
	result, err := ParseAnalyzeCurrentCodebaseBatchResult(`{
  "focuses": [
    {
      "focus_id": "auth",
      "focus_name": "Authentication",
      "patterns": []
    }
  ]
}`)

	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorContains(t, err, "focuses[].profile_refresh_recommended")
}

func TestParseAnalyzeCurrentDeltaBatchResultRejectsMissingProfileRefreshRecommendation(t *testing.T) {
	result, err := ParseAnalyzeCurrentDeltaBatchResult(`{
  "knowledge_changes": []
}`)

	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorContains(t, err, "profile_refresh_recommended")
}

func TestParseAnalyzeCurrentDeltaBatchResultRejectsMissingFocusID(t *testing.T) {
	result, err := ParseAnalyzeCurrentDeltaBatchResult(`{
  "knowledge_changes": [
    {
      "focus_action": "no_change",
      "pattern_action": "no_change",
      "anchors": [],
      "reason": "no reusable knowledge change"
    }
  ],
  "profile_refresh_recommended": {"needed": false}
}`)

	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorContains(t, err, "knowledge_changes[].focus_id")
}

func currentBatchOutput(patterns string) string {
	return `{
  "focuses": [
    {
      "focus_id": "auth",
      "focus_name": "Authentication",
      "patterns": ` + patterns + `,
      "profile_refresh_recommended": {"needed": false}
    }
  ]
}`
}
