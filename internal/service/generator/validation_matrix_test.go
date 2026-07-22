package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationMatrixChoosesCommandByEvidencePath(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{Command: "go test ./internal/handler/home/...", When: "接口变更后运行"},
			{Command: "go test ./plugins/key_manage/...", When: "插件变更后运行"},
			{Command: "go build ./..."},
		},
	}
	pattern := domain.NewPattern("key-api", "Key Manage Interface", domain.CategoryAPI)
	pattern.Confidence = 0.95
	pattern.Frequency = 2
	pattern.SetRule("Parse key manage request")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "plugins/key_manage/internal/handler/key_manage/create.go",
		Line:   12,
		Symbol: "Create",
	}}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	require.NotEmpty(t, matrix)
	assert.Equal(t, "go test ./plugins/key_manage/...", matrix[0].Command)
	assert.NotContains(t, matrix[0].When, "未找到覆盖该范围的专用验证命令")
}

func TestValidationMatrixDoesNotAssociateUnscopedGeneratorWithBusinessEvidence(t *testing.T) {
	profile := &domain.ProjectProfile{ValidationCommands: []domain.ValidationCommand{{
		Command:  "jzero gen",
		When:     "修改 API 描述后重新生成代码。",
		Evidence: []string{"Taskfile.yml"},
		Type:     "generate",
	}}}
	pattern := domain.NewPattern("operate-log", "操作日志 API 处理", domain.CategoryAPI)
	pattern.Confidence = 0.95
	pattern.SetDescription("操作日志导出与删除流程。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "plugins/logger_cipher_operate/internal/logic/operate_log/trigger_export_operate_log.go", Line: 34},
		{Path: "plugins/logger_cipher_operate/internal/service/operate_log/export_service.go", Line: 37},
		{Path: "plugins/logger_cipher_operate/internal/service/operate_log/delete_task_processor.go", Line: 34},
	}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	require.Empty(t, matrix)
}

func TestValidationMatrixDisplaysCommandOwnedEvidence(t *testing.T) {
	profile := &domain.ProjectProfile{ValidationCommands: []domain.ValidationCommand{{
		Command:    "go test ./plugins/logger_cipher_operate/...",
		ScopePaths: []string{"plugins/logger_cipher_operate"},
		Evidence:   []string{"Taskfile.yml:42"},
		Type:       "test",
	}}}
	pattern := domain.NewPattern("operate-log", "操作日志业务流程", domain.CategoryBusiness)
	pattern.Confidence = 0.95
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path: "plugins/logger_cipher_operate/internal/service/operate_log/export_service.go", Line: 37,
	}}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	require.Len(t, matrix, 1)
	require.Contains(t, matrix[0].Evidence, "Taskfile.yml:42")
	require.NotContains(t, matrix[0].Evidence, "plugins/logger_cipher_operate/internal/service/operate_log/export_service.go:37")
}

func TestValidationMatrixDoesNotUsePluginCommandForInternalEvidence(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{Command: "go test ./plugins/key_manage/...", When: "插件变更后运行"},
			{Command: "go test ./internal/handler/home/...", When: "接口变更后运行"},
			{Command: "go build ./..."},
		},
	}
	pattern := domain.NewPattern("home-api", "Home Interface", domain.CategoryAPI)
	pattern.Confidence = 0.95
	pattern.Frequency = 2
	pattern.SetRule("Parse home request")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "internal/handler/home/home.go",
		Line:   12,
		Symbol: "Home",
	}}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	require.NotEmpty(t, matrix)
	assert.Equal(t, "go test ./internal/handler/home/...", matrix[0].Command)
}

func TestValidationMatrixPrefersPatternEvidenceOverBroadModuleEvidence(t *testing.T) {
	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{{
			Name:             "custom",
			Path:             "internal/custom",
			Responsibilities: []string{"生成启动配置"},
		}},
		ValidationCommands: []domain.ValidationCommand{
			{Command: "go test ./internal/custom/..."},
			{Command: "go test ./plugins/key_manage/..."},
			{Command: "go build ./..."},
		},
	}
	pattern := domain.NewPattern("key-api", "Key Manage API", domain.CategoryAPI)
	pattern.Confidence = 0.95
	pattern.Frequency = 2
	pattern.SetRule("Parse key manage request")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "plugins/key_manage/internal/handler/key_manage/create.go",
		Line:   12,
		Symbol: "Create",
	}}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	require.NotEmpty(t, matrix)
	assert.Equal(t, "go test ./plugins/key_manage/...", matrix[0].Command)
	assert.Contains(t, matrix[0].Evidence, "plugins/key_manage")
	assert.NotContains(t, matrix[0].Evidence, "plugins/key_manage/internal/handler/key_manage/create.go:12")
}

func TestValidationMatrixPrefersNarrowTestCommandOverGlobalBuild(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{
				Command: "GOOS=$GOOS GOARCH=$GOARCH go build -tags no_k8s -o cipher_machine main.go",
				When:    "本地打包主项目二进制。",
				Type:    "build",
			},
			{
				Command:    "go test ./plugins/kmip_cluster_manage/...",
				When:       "修改 KMIP 集群或热备逻辑后运行。",
				ScopePaths: []string{"plugins/kmip_cluster_manage"},
				Type:       "test",
			},
		},
	}
	pattern := domain.NewPattern("kmip-cluster", "KMIP 集群状态同步", domain.CategoryBusiness)
	pattern.Confidence = 0.93
	pattern.Frequency = 3
	pattern.SetRule("KMIP 集群变更后同步本地角色和 system_cluster。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "plugins/kmip_cluster_manage/internal/service/kmipcontrol/cluster_service.go",
		Line:   42,
		Symbol: "ClusterService",
	}}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	require.NotEmpty(t, matrix)
	assert.Equal(t, "go test ./plugins/kmip_cluster_manage/...", matrix[0].Command)
}

func TestValidationMatrixOmitsFallbackForBroadEvidence(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{
				Command: "project check",
				When:    "检查 API 契约或生成链路。",
				Type:    "check",
			},
			{
				Command: "GOOS=$GOOS GOARCH=$GOARCH go build -tags no_k8s -o cipher_machine main.go",
				When:    "本地打包主项目二进制。",
				Type:    "build",
			},
			{
				Command:    "codegen generate swagger",
				When:       "修改插件 API 契约后生成插件 swagger 文档。",
				Workdir:    "plugins/key_manage",
				ScopePaths: []string{"plugins/key_manage/desc/api", "plugins/key_manage/desc/swagger"},
				Type:       "generate",
			},
			{
				Command:    "go test ./plugins/kmip_cluster_manage/...",
				When:       "修改 KMIP 集群或热备逻辑后运行。",
				ScopePaths: []string{"plugins/kmip_cluster_manage"},
				Type:       "test",
			},
		},
	}
	pattern := domain.NewPattern("startup-flow", "启动配置与插件装配", domain.CategoryConfig)
	pattern.Confidence = 0.91
	pattern.Frequency = 4
	pattern.SetRule("启动流程跨配置、备份导出和插件装配。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/logic/system/config/backup/export.go", Line: 31, Symbol: "Export"},
		{Path: "internal/logic/system/config/backup/export.go", Line: 53, Symbol: "Export"},
		{Path: "plugins/kmip_cluster_manage/internal/logic/cluster_manage/cluster_outer/joincluster.go", Line: 41, Symbol: "JoinCluster"},
	}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	assert.Empty(t, matrix)
}

func TestValidationMatrixDoesNotUseFallbackCommandForMixedEvidence(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{
				Command:  "project check",
				When:     "检查项目配置、API 契约或生成链路。",
				Evidence: []string{"README.md"},
				Type:     "check",
			},
			{
				Command:    "go test ./plugins/kmip_cluster_manage/...",
				When:       "修改 KMIP 集群创建、状态推进、HA 配置或插件装配后运行。",
				ScopePaths: []string{"plugins/kmip_cluster_manage"},
				Type:       "test",
			},
		},
	}
	pattern := domain.NewPattern("business-flow", "业务流程与插件装配", domain.CategoryBusiness)
	pattern.Confidence = 0.93
	pattern.Frequency = 4
	pattern.SetRule("业务流程跨主应用配置和插件状态编排。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/logic/system/config/backup/export.go", Line: 31, Symbol: "Export"},
		{Path: "internal/logic/system/config/backup/export.go", Line: 53, Symbol: "Export"},
		{Path: "plugins/kmip_cluster_manage/internal/logic/cluster_manage/cluster_outer/createcluster.go", Line: 39, Symbol: "CreateCluster"},
	}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	assert.Empty(t, matrix)
}

func TestValidationMatrixOmitsGenericCheckAndBroadBuildForMixedEvidence(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{
				Command: "project check",
				When:    "检查项目配置、API 契约或生成链路。",
				Type:    "check",
			},
			{
				Command:    "GOOS=$GOOS GOARCH=$GOARCH go build -gcflags=\"all=-N -l\" -tags no_k8s -o $BIN_NAME main.go",
				When:       "本地打包主项目二进制。",
				ScopePaths: []string{"main.go", "cmd", "internal", "pkg", "plugins"},
				Type:       "build",
			},
		},
	}
	pattern := domain.NewPattern("business-flow", "业务流程与插件装配", domain.CategoryBusiness)
	pattern.Confidence = 0.93
	pattern.Frequency = 4
	pattern.SetRule("业务流程跨主应用配置和插件状态编排。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/logic/system/config/backup/export.go", Line: 31, Symbol: "Export"},
		{Path: "plugins/kmip_cluster_manage/internal/logic/cluster_manage/cluster_outer/createcluster.go", Line: 39, Symbol: "CreateCluster"},
	}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	assert.Empty(t, matrix)
}

func TestValidationMatrixTreatsSingleKeywordGenericCommandAsFallback(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{
				Command: "project check",
				When:    "检查项目配置是否符合工具要求。",
				Type:    "check",
			},
		},
	}
	pattern := domain.NewPattern("startup-config", "配置启动链路", domain.CategoryConfig)
	pattern.Confidence = 0.91
	pattern.Frequency = 3
	pattern.SetRule("配置结构变化会影响启动链路。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/config/config.go", Line: 12, Symbol: "Config"},
		{Path: "internal/svc/servicecontext.go", Line: 22, Symbol: "NewServiceContext"},
	}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	assert.Empty(t, matrix)
}

func TestValidationMatrixMarksBroadBuildAsFallback(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{
				Command:    "GOOS=$GOOS GOARCH=$GOARCH go build -tags no_k8s -o app main.go",
				When:       "本地打包主项目二进制。",
				ScopePaths: []string{"main.go", "cmd", "internal", "pkg", "plugins"},
				Type:       "build",
			},
		},
	}
	pattern := domain.NewPattern("startup-config", "配置启动链路", domain.CategoryConfig)
	pattern.Confidence = 0.91
	pattern.Frequency = 3
	pattern.SetRule("配置结构变化会影响启动链路。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/config/config.go", Line: 12, Symbol: "Config"},
		{Path: "plugins/example/internal/setup.go", Line: 22, Symbol: "Setup"},
	}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	assert.Empty(t, matrix)
}

func TestValidationMatrixDoesNotUseBroadTestForBusinessFlow(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{
				Command:    "codegen generate api",
				When:       "修改 API 契约或生成链路后运行。",
				ScopePaths: []string{"desc/api"},
				Type:       "generate",
			},
			{
				Command: "go test ./...",
				When:    "修改业务逻辑后运行。",
				Type:    "test",
			},
		},
	}
	pattern := domain.NewPattern("order-state", "Order State Transition", domain.CategoryBusiness)
	pattern.Confidence = 0.94
	pattern.Frequency = 3
	pattern.SetRule("业务状态流转必须经过领域服务校验并持久化。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "internal/service/order/transition.go",
		Line:   42,
		Symbol: "ApplyTransition",
	}}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	assert.Empty(t, matrix)
}

func TestValidationMatrixOmitsGenericCheckAndBroadRaceTestFallback(t *testing.T) {
	profile := &domain.ProjectProfile{
		ValidationCommands: []domain.ValidationCommand{
			{Command: "jzero check", When: "开发过程中检查代码和配置", Source: "README.md", Evidence: []string{"README.md"}, Type: "check"},
			{Command: "go test -race ./...", When: "运行测试时检查竞态条件", Source: "user_context", Evidence: []string{"test"}, Type: "test"},
			{Command: "task build", When: "构建项目验证编译", Source: "Taskfile.yml", Evidence: []string{"Taskfile.yml"}, Type: "build"},
		},
	}
	pattern := domain.NewPattern("admin-flow", "管理员业务流程", domain.CategoryBusiness)
	pattern.Confidence = 0.91
	pattern.Frequency = 3
	pattern.SetRule("管理员变更需要保持加密和角色校验顺序。")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "internal/logic/system/admin/create.go",
		Line:   48,
		Symbol: "Create",
	}}

	matrix := validationMatrix(profile, []domain.Pattern{*pattern}, "zh-CN")

	assert.Empty(t, matrix)
}

func TestValidationCommandPaths(t *testing.T) {
	paths := validationCommandPaths("go test ./plugins/server_manage/internal/logic/service_manage/ ./internal/handler/home/...")

	assert.Equal(t, []string{
		"plugins/server_manage/internal/logic/service_manage",
		"internal/handler/home",
	}, paths)
}
