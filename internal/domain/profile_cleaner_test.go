package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanProjectProfileNormalizesBusinessMethodLocation(t *testing.T) {
	profile := &ProjectProfile{
		BusinessMethods: []BusinessMethod{
			{
				Name:         "CreateOrder",
				CodeLocation: CodeLocation{CurrentLocation: "internal/service/order.go:42"},
			},
		},
	}

	cleaned := CleanProjectProfile(profile)

	assert.Len(t, cleaned.BusinessMethods, 1)
	method := cleaned.BusinessMethods[0]
	assert.Equal(t, "internal/service/order.go:42", method.CodeLocation.HistoricalLocation)
	assert.Equal(t, "internal/service/order.go:42", method.CodeLocation.CurrentLocation)
	assert.Equal(t, CodeLocationStatusValid, method.CodeLocation.Status)
	assert.False(t, method.CodeLocation.CreatedAt.IsZero())
	assert.False(t, method.CodeLocation.UpdatedAt.IsZero())
}

func TestCleanProjectProfileCleansValidationCommands(t *testing.T) {
	profile := &ProjectProfile{
		ValidationCommands: []ValidationCommand{
			{Command: " task verify ", When: " after changes ", Source: " Taskfile.yml "},
			{Command: "task verify", When: "after changes", Source: "README.md"},
			{Command: ""},
			{Command: "TODO add command"},
			{Command: "待确认验证命令"},
		},
	}

	cleaned := CleanProjectProfile(profile)

	require.Len(t, cleaned.ValidationCommands, 1)
	assert.Equal(t, "task verify", cleaned.ValidationCommands[0].Command)
	assert.Equal(t, "after changes", cleaned.ValidationCommands[0].When)
	assert.Equal(t, "Taskfile.yml", cleaned.ValidationCommands[0].Source)
}

func TestCleanProjectProfileDeduplicatesModulesAndFiltersPlaceholders(t *testing.T) {
	profile := &ProjectProfile{
		KeyModules: []ModuleInfo{
			{
				Name:             " home ",
				Path:             "",
				Description:      "first",
				Responsibilities: []string{" route ", "TODO confirm"},
				KeyMethods:       []string{"List", "List"},
			},
			{
				Name:             "home",
				Path:             "",
				Description:      "second",
				Responsibilities: []string{"handler"},
				Dependencies:     []string{"svc", "svc"},
				KeyMethods:       []string{"Create"},
			},
			{
				Name: "",
				Path: "",
			},
		},
	}

	cleaned := CleanProjectProfile(profile)

	require.Len(t, cleaned.KeyModules, 1)
	module := cleaned.KeyModules[0]
	assert.Equal(t, "home", module.Name)
	assert.Equal(t, "first", module.Description)
	assert.Equal(t, []string{"route", "handler"}, module.Responsibilities)
	assert.Equal(t, []string{"svc"}, module.Dependencies)
	assert.Equal(t, []string{"List", "Create"}, module.KeyMethods)
}

func TestCleanProjectProfileCollapsesFrameworkAndConfigTemplatePatterns(t *testing.T) {
	profile := &ProjectProfile{
		FrameworkPatterns: []string{
			"Handler层使用go-zero的httpx.Parse解析请求到types结构",
			"Handler 层使用 httpx.Parse 解析请求，httpx.OkJsonCtx/httpx.ErrorCtx 返回响应",
			"Logic层使用logx.WithContext初始化日志",
			"Logic 层嵌入 logx.Logger 记录日志",
		},
		ConfigPatterns: []string{
			"使用 condition.Condition 构建数据库查询条件",
			"使用 jzero 框架的 condition 包构建数据库查询条件",
			"KMIP基础请求使用kmiprequest.BaseRequestStandard()",
			"所有 KMIP DTO 的 BaseReq 字段使用 kmiprequest.BaseRequestStandard()",
		},
	}

	cleaned := CleanProjectProfile(profile)

	assert.Len(t, cleaned.FrameworkPatterns, 2)
	assert.Len(t, cleaned.ConfigPatterns, 2)
}

func TestNewProjectSpecFromProfilePreservesValidationCommands(t *testing.T) {
	spec := NewProjectSpecFromProfile(&ProjectProfile{
		ProjectName: "demo",
		Language:    "unknown",
		ValidationCommands: []ValidationCommand{
			{Command: "task verify", When: "after changes", Source: "Taskfile.yml"},
		},
	}, nil, WorkspaceProjectOverride{})

	require.NotNil(t, spec)
	require.Len(t, spec.ValidationCommands, 1)
	assert.Equal(t, "task verify", spec.ValidationCommands[0].Command)
	assert.Equal(t, "Taskfile.yml", spec.ValidationCommands[0].Source)
}

func TestNewProjectSpecFromProfileDemotesScopedAndSingleEvidencePatterns(t *testing.T) {
	global := NewPattern("global", "Cross Module Error Wrapping", CategoryError)
	global.Confidence = 0.95
	global.Frequency = 3
	global.SetRule("Wrap domain errors with project context")
	global.EvidenceLocations = []PatternEvidenceLocation{
		{Path: "internal/service/a.go", Line: 10, Symbol: "A"},
		{Path: "plugins/key_manage/service/b.go", Line: 20, Symbol: "B"},
	}

	scoped := NewPattern("scoped", "Handler Double Parse", CategoryAPI)
	scoped.Confidence = 1
	scoped.Frequency = 6
	scoped.SetRule("All handlers must call fuzzy.DecodeRequest before httpx.Parse")
	scoped.EvidenceLocations = []PatternEvidenceLocation{
		{Path: "internal/handler/home/home_compact.go", Line: 1, Symbol: "Home"},
		{Path: "internal/handler/home/auth_compact.go", Line: 2, Symbol: "Auth"},
	}

	single := NewPattern("single", "One Off Business Rule", CategoryBusiness)
	single.Confidence = 0.95
	single.Frequency = 1
	single.SetRule("Use one-off flow")
	single.EvidenceLocations = []PatternEvidenceLocation{{Path: "internal/logic/one.go", Line: 1, Symbol: "One"}}

	spec := NewProjectSpecFromProfile(&ProjectProfile{ProjectName: "demo"}, []Pattern{*global, *scoped, *single}, WorkspaceProjectOverride{})

	require.NotNil(t, spec)
	require.Len(t, spec.PatternRules, 1)
	assert.Equal(t, "Cross Module Error Wrapping", spec.PatternRules[0].Name)
	require.Len(t, spec.PatternGuidance, 1)
	assert.Equal(t, "Handler Double Parse", spec.PatternGuidance[0].Name)
}

func TestNewProjectSpecFromProfileCollapsesDuplicatePatternRules(t *testing.T) {
	first := NewPattern("a", "Go Zero Handler Standard", CategoryAPI)
	first.Confidence = 0.95
	first.Frequency = 2
	first.SetRule("Handler should parse request, create logic, call method, and return JSON response")
	first.EvidenceLocations = []PatternEvidenceLocation{{Path: "internal/handler/a.go", Line: 1, Symbol: "A"}}

	second := NewPattern("b", "Handler Standard Flow", CategoryAPI)
	second.Confidence = 0.94
	second.Frequency = 3
	second.SetRule("Handler should parse request, create logic, call method, and return JSON response")
	second.EvidenceLocations = []PatternEvidenceLocation{{Path: "plugins/key_manage/handler/b.go", Line: 1, Symbol: "B"}}

	spec := NewProjectSpecFromProfile(&ProjectProfile{ProjectName: "demo"}, []Pattern{*first, *second}, WorkspaceProjectOverride{})

	require.NotNil(t, spec)
	total := len(spec.PatternRules) + len(spec.PatternGuidance)
	assert.Equal(t, 1, total)
	if len(spec.PatternRules) == 1 {
		assert.Equal(t, 5, spec.PatternRules[0].Frequency)
	} else {
		require.Len(t, spec.PatternGuidance, 1)
		assert.Equal(t, 5, spec.PatternGuidance[0].Frequency)
	}
}

func TestNewProjectSpecFromProfileKeepsGuidanceAsNavigationHints(t *testing.T) {
	handler := NewPattern("handler", "Handler Double Parse", CategoryAPI)
	handler.Confidence = 1
	handler.Frequency = 6
	handler.SetRule("所有 Handler 函数必须先使用 fuzzy.DecodeRequest 解析请求，再使用 httpx.Parse 进行参数验证")
	handler.EvidenceLocations = []PatternEvidenceLocation{
		{Path: "internal/handler/home/home.go", Line: 1, Symbol: "Home"},
		{Path: "internal/handler/home/auth.go", Line: 2, Symbol: "Auth"},
	}

	oneOff := NewPattern("one-off", "One Off Flow", CategoryBusiness)
	oneOff.Confidence = 0.95
	oneOff.Frequency = 1
	oneOff.SetRule("必须按单个文件里的临时流程处理")
	oneOff.EvidenceLocations = []PatternEvidenceLocation{{Path: "internal/logic/one.go", Line: 1, Symbol: "One"}}

	spec := NewProjectSpecFromProfile(&ProjectProfile{ProjectName: "demo"}, []Pattern{*handler, *oneOff}, WorkspaceProjectOverride{})

	require.NotNil(t, spec)
	require.Len(t, spec.PatternGuidance, 1)
	assert.Equal(t, "Handler Double Parse", spec.PatternGuidance[0].Name)
	assert.Empty(t, spec.PatternGuidance[0].Rule)
	assert.NotContains(t, spec.PatternGuidance[0].Description, "必须")
	assert.NotContains(t, spec.PatternGuidance[0].Description, "所有")
}

func TestNewProjectSpecFromProfilePromotesStableAnalysisUnitRules(t *testing.T) {
	stable := NewPattern("kmip-success", "KMIP JSON 响应检查成功码", CategoryError)
	stable.Confidence = 0.93
	stable.Frequency = 6
	stable.AnalysisUnitID = "key-manage-lifecycle"
	stable.SetRule("KMIP JSON 响应解析后必须检查 Code 是否为 Success，失败时返回当前业务动作对应的错误。")
	stable.EvidenceLocations = []PatternEvidenceLocation{
		{Path: "plugins/key_manage/internal/logic/key_manage/create.go", Line: 10, Symbol: "Create"},
		{Path: "plugins/key_manage/internal/logic/key_manage/delete.go", Line: 20, Symbol: "Delete"},
		{Path: "plugins/key_manage/internal/logic/key_manage/update.go", Line: 30, Symbol: "Update"},
	}
	stable.RefreshMetrics()

	local := NewPattern("local-flow", "单文件初始化流程", CategoryBusiness)
	local.Confidence = 0.95
	local.Frequency = 2
	local.AnalysisUnitID = "system-init"
	local.SetRule("创建初始化管理员后写入 FirstInit 完成状态。")
	local.EvidenceLocations = []PatternEvidenceLocation{
		{Path: "internal/logic/system/initialization/create_admin.go", Line: 10, Symbol: "CreateAdmin"},
	}
	local.RefreshMetrics()

	spec := NewProjectSpecFromProfile(&ProjectProfile{ProjectName: "demo"}, []Pattern{*stable, *local}, WorkspaceProjectOverride{})

	require.NotNil(t, spec)
	require.Len(t, spec.PatternRules, 1)
	assert.Equal(t, "KMIP JSON 响应检查成功码", spec.PatternRules[0].Name)
	require.Len(t, spec.PatternGuidance, 1)
	assert.Equal(t, "单文件初始化流程", spec.PatternGuidance[0].Name)
}
