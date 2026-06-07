package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateWorkspaceSkills_RendersOnlyWorkspaceRoot(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetForProjectFn: func(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
			require.Fail(t, "workspace root generation should not read child profiles")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("codex", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "codex"},
	}
	svc := NewWorkspaceGenerator(mockPattern, mockProfile, loader, &mocks.MockAgent{NameVal: "codex", AvailableVal: true}, cfg, nil, nil)

	require.NoError(t, svc.GenerateWorkspaceSkills(context.Background()))

	require.FileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-workspace", "SKILL.md"))
	require.FileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-workspace", "references", "workspace-overview.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "skills-seed-skills", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "skills-seed-skills", "references", "project-spec.md"))
}

func TestGenerateWorkspaceSkillsSkipsWhenInputFingerprintUnchanged(t *testing.T) {
	ctx := context.Background()
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	dbPath := filepath.Join(t.TempDir(), "workspace.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()

	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(patternRepo, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)

	require.NoError(t, svc.GenerateWorkspaceSkills(ctx))

	outputPath := filepath.Join(projectRoot, ".claude", "skills", "demo-workspace")
	skillPath := filepath.Join(outputPath, "SKILL.md")
	oldTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, os.Chtimes(skillPath, oldTime, oldTime))

	require.NoError(t, svc.GenerateWorkspaceSkills(ctx))

	stat, err := os.Stat(skillPath)
	require.NoError(t, err)
	require.Equal(t, oldTime.UnixNano(), stat.ModTime().UnixNano())
}

func TestGenerateWorkspaceSkillsDoesNotSkipWhenReferenceOutputIsIncomplete(t *testing.T) {
	ctx := context.Background()
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	dbPath := filepath.Join(t.TempDir(), "workspace.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()

	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(patternRepo, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)

	require.NoError(t, svc.GenerateWorkspaceSkills(ctx))

	outputPath := filepath.Join(projectRoot, ".claude", "skills", "demo-workspace")
	missingPath := filepath.Join(outputPath, "references", "cross-project-rules.md")
	require.NoError(t, os.Remove(missingPath))

	require.NoError(t, svc.GenerateWorkspaceSkills(ctx))
	require.FileExists(t, missingPath)
}

func TestGenerateWorkspaceSkillsUsesPersistedWorkspaceArtifacts(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "claude", AvailableVal: true,
		AnalyzeWorkspaceProfileFn: func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
			require.Fail(t, "persisted workspace profile should be used before AI analysis")
			return nil, nil
		},
		AnalyzeWorkspaceSpecFn: func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
			require.Fail(t, "persisted workspace spec should be used before AI analysis")
			return nil, nil
		},
	}
	profileRepo := &memoryWorkspaceProfileRepo{profile: &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: projectRoot,
		Summary:  "学习阶段沉淀：backend 是私有化部署主后端，不是 SaaS。",
		Projects: []domain.WorkspaceProject{
			{
				ID:             "backend",
				Path:           "backend",
				Type:           "backend",
				Language:       "go",
				Responsibility: "学习阶段沉淀：负责离线安装包的管理 API。",
			},
		},
		Shared: []domain.WorkspacePath{
			{Path: "shared", Description: "学习阶段沉淀：离线包共享配置", Consumers: []string{"backend"}},
		},
	}}
	specRepo := &memoryWorkspaceSpecRepo{spec: &domain.WorkspaceSpec{
		Name:     "demo",
		RootPath: projectRoot,
		Rules: []domain.WorkspaceRule{
			{Title: "离线交付边界", Description: "学习阶段沉淀：变更 backend 时必须保留离线安装包验证。", AppliesTo: []string{"backend"}},
		},
		Routing: []domain.WorkspaceRoute{
			{PathPattern: "backend/**", ProjectIDs: []string{"backend"}, Reason: "学习阶段沉淀：backend 变更读取 backend skill"},
		},
	}}

	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg, profileRepo, specRepo)

	require.NoError(t, svc.GenerateWorkspaceSkills(context.Background()))

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "学习阶段沉淀：backend 是私有化部署主后端，不是 SaaS。")
	require.Contains(t, overview, "学习阶段沉淀：负责离线安装包的管理 API。")
	require.Contains(t, overview, "`shared` - 学习阶段沉淀：离线包共享配置")

	rules := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "cross-project-rules.md")
	require.Contains(t, rules, "离线交付边界")
	require.Contains(t, rules, "学习阶段沉淀：变更 backend 时必须保留离线安装包验证。")
}

func TestGenerateWorkspaceSkillsRejectsChildOutputOutsideChildRoot(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend", ".skills-seed"), 0755))

	childConfig := strings.TrimSpace(`
project:
  name: backend
  mode: project
  root_path: ` + filepath.Join(projectRoot, "backend") + `
  language: go
agent:
  engine: codex
skills:
  target: codex
  paths:
    codex: "../escaped-skill"
`)
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "backend", ".skills-seed", "config.yaml"), []byte(childConfig), 0644))

	loader := skills.NewLoaderForAgent("codex", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "codex"},
	}
	svc := NewWorkspaceGenerator(&mocks.MockPatternRepository{}, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "codex", AvailableVal: true}, cfg, nil, nil)

	err := svc.GenerateWorkspaceSkills(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), i18n.GetWithParams("GenerateOutputPathOutsideProjectRoot", map[string]interface{}{
		"OutputPath":  "../escaped-skill",
		"ProjectRoot": filepath.Join(projectRoot, "backend"),
	}))
	require.NoFileExists(t, filepath.Join(projectRoot, "escaped-skill", "SKILL.md"))
}

func TestGenerateWorkspaceSkillsDoesNotPersistRuntimeContextInWorkspaceReferences(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "hsmwebapi"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "core-engine"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsm-workspace", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{
				{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
				{ID: "core-engine", Path: "core-engine", Type: "library", Language: "go"},
			},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)
	ctx := runtimecontext.WithUserContext(context.Background(), strings.TrimSpace(`
HSM 工作区用于管理密码设备、密钥服务、KMIP 接入和日志/网络组件。
hsmwebapi 是管理 API 入口，core-engine 是核心能力库。
产品是私有化部署，交付物是离线安装包；不要建议 SaaS 化。
常见任务：改管理端接口先进入 hsmwebapi，改核心密码能力先进入 core-engine。
验证管道：先运行 go test ./...，再按受影响模块补充集成验证。
`))

	require.NoError(t, svc.GenerateWorkspaceSkills(ctx))

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "hsm-workspace-workspace", "references", "workspace-overview.md")
	require.NotContains(t, overview, "HSM 工作区用于管理密码设备")
	require.NotContains(t, overview, "hsmwebapi 是管理 API 入口")
	require.NotContains(t, overview, "私有化部署")
	require.NotContains(t, overview, "go test ./...")
	require.NotContains(t, overview, "用户提供的工作区说明")
}

func TestGenerateWorkspaceSkills_ContextUsesWorkspaceAIPromptsForRootSkill(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "hsmwebapi"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "kmip-go"), 0755))

	var profileReq *agent.AnalyzeWorkspaceProfileRequest
	var specReq *agent.AnalyzeWorkspaceSpecRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "claude", AvailableVal: true,
		AnalyzeWorkspaceProfileFn: func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
			profileReq = req
			workspaceInput := readFilePath(t, req.WorkspaceInputPath)
			require.Contains(t, workspaceInput, `"id": "hsmwebapi"`)
			require.Contains(t, workspaceInput, `"skill_path": "hsmwebapi/.claude/skills/skills-seed-skills/SKILL.md"`)
			return &domain.WorkspaceProfile{
				Name:     req.WorkspaceName,
				RootPath: req.WorkspaceRoot,
				Summary:  "HSM 工作区负责私有化密码设备管理，hsmwebapi 调用 kmip-go 暴露 KMIP 能力。",
				Projects: []domain.WorkspaceProject{
					{
						ID:             "hsmwebapi",
						Path:           "hsmwebapi",
						Type:           "backend",
						Language:       "go",
						Responsibility: "主后端，负责管理 API 和调用 KMIP 能力。",
					},
					{
						ID:             "kmip-go",
						Path:           "kmip-go",
						Type:           "backend",
						Language:       "go",
						Responsibility: "KMIP 协议能力实现。",
					},
				},
				Dependencies: []domain.WorkspaceDependency{
					{From: "hsmwebapi", To: "kmip-go", Reason: "hsmwebapi 调用 kmip-go 实现 KMIP 能力"},
				},
				Shared: []domain.WorkspacePath{
					{Path: "core-engine", Description: "核心能力库", Consumers: []string{"hsmwebapi", "kmip-go"}},
				},
				Contracts: []domain.WorkspacePath{
					{Path: "kmip-go/proto", Description: "KMIP 契约", Producers: []string{"kmip-go"}, Consumers: []string{"hsmwebapi"}},
				},
				Infra: []domain.WorkspacePath{
					{Path: "base-xengine", Description: "离线安装底座", AffectedProjects: []string{"hsmwebapi", "kmip-go"}},
				},
				ImpactRoutes: []domain.WorkspaceRoute{
					{PathPattern: "kmip-go/**", ProjectIDs: []string{"hsmwebapi", "kmip-go"}, Reason: "KMIP 能力变更需要同步检查调用方"},
				},
			}, nil
		},
		AnalyzeWorkspaceSpecFn: func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
			specReq = req
			workspaceInput := readFilePath(t, req.WorkspaceInputPath)
			profileInput := readFilePath(t, req.WorkspaceProfilePath)
			require.Contains(t, workspaceInput, `"id": "kmip-go"`)
			require.Contains(t, profileInput, "hsmwebapi 调用 kmip-go")
			return &domain.WorkspaceSpec{
				Name:     req.WorkspaceName,
				RootPath: req.WorkspaceRoot,
				Routing: []domain.WorkspaceRoute{
					{PathPattern: "hsmwebapi/**", ProjectIDs: []string{"hsmwebapi"}, Reason: "管理 API 变更先读取 hsmwebapi skill"},
					{PathPattern: "kmip-go/**", ProjectIDs: []string{"hsmwebapi", "kmip-go"}, Reason: "KMIP 能力变更需同步检查 hsmwebapi 调用"},
				},
				Rules: []domain.WorkspaceRule{
					{Title: "KMIP 能力同步", Description: "修改 kmip-go 后必须检查 hsmwebapi 的调用适配和集成验证。", AppliesTo: []string{"hsmwebapi", "kmip-go"}},
				},
				ChangeOrder: []string{
					"先确认 KMIP 契约和核心能力边界",
					"再更新 hsmwebapi 调用和验证",
				},
				LoadMultipleSkillsWhen: []domain.WorkspaceLoadMultipleSkill{
					{Condition: "修改 kmip-go/**", ProjectIDs: []string{"hsmwebapi", "kmip-go"}, Reason: "调用方和能力实现必须一起检查"},
				},
			}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsm-workspace", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{
				{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
				{ID: "kmip-go", Path: "kmip-go", Type: "backend", Language: "go"},
			},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg, nil, nil)
	ctx := runtimecontext.WithUserContext(context.Background(), "hsmwebapi 为主后端，它调用 kmip-go 实现 KMIP 的能力。")

	require.NoError(t, svc.GenerateWorkspaceSkills(ctx))

	require.NotNil(t, profileReq)
	require.NotEmpty(t, profileReq.WorkspaceInputPath)
	require.Contains(t, filepath.ToSlash(profileReq.WorkspaceInputPath), ".skills-seed/memory/runtime/")
	require.NotNil(t, specReq)
	require.NotEmpty(t, specReq.WorkspaceInputPath)
	require.NotEmpty(t, specReq.WorkspaceProfilePath)
	require.Contains(t, filepath.ToSlash(specReq.WorkspaceProfilePath), ".skills-seed/memory/runtime/")
	require.NoFileExists(t, profileReq.WorkspaceInputPath)
	require.NoFileExists(t, specReq.WorkspaceInputPath)

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "hsm-workspace-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "HSM 工作区负责私有化密码设备管理")
	require.Contains(t, overview, "主后端，负责管理 API")
	require.Contains(t, overview, "hsmwebapi 调用 kmip-go 实现 KMIP 能力")
	require.Contains(t, overview, "`kmip-go/proto` - KMIP 契约")
	require.Contains(t, overview, "`core-engine` - 核心能力库")
	require.Contains(t, overview, "`base-xengine` - 离线安装底座")
	require.Contains(t, overview, "KMIP 能力变更需要同步检查调用方")

	rules := readGeneratedFile(t, projectRoot, ".claude", "skills", "hsm-workspace-workspace", "references", "cross-project-rules.md")
	require.Contains(t, rules, "KMIP 能力同步")
	require.Contains(t, rules, "先确认 KMIP 契约和核心能力边界")
	require.Contains(t, rules, "修改 kmip-go/**")
	require.Contains(t, rules, "修改已识别契约路径时")
	require.Contains(t, rules, "修改已识别共享路径时")
	require.Contains(t, rules, "修改已识别基础设施路径时")
}

func TestGenerateWorkspaceSkills_RootSkillStaysConciseAndRoutesViaOverview(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend", ".skills-seed"), 0755))
	childConfig := `
profile:
  name: "backend"
  mode: "project"
  language: "go"
  locale: "zh-CN"
  root_path: ""
agent:
  engine: "codex"
  commands:
    codex: "codex"
skills:
  target: "codex"
  paths:
    codex: ".agents/skills/custom-child-skill"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "backend", ".skills-seed", "config.yaml"), []byte(childConfig), 0644))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)

	require.NoError(t, svc.GenerateWorkspaceSkills(context.Background()))

	rootSkill := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md")
	require.NotContains(t, rootSkill, "## 工作区地图")
	require.NotContains(t, rootSkill, "## 路由规则")
	require.NotContains(t, rootSkill, "## 影响范围判断")
	require.NotContains(t, rootSkill, "## 跨项目执行顺序")
	require.NotContains(t, rootSkill, "## 并发边界")
	require.NotContains(t, rootSkill, "backend/.agents/skills/custom-child-skill/SKILL.md")
	require.Contains(t, rootSkill, "[工作区概览](./references/workspace-overview.md)")
	require.Contains(t, rootSkill, "[跨项目规则](./references/cross-project-rules.md)")

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "子项目配置：`backend/.skills-seed/config.yaml`")
	require.Contains(t, overview, "backend/.agents/skills/custom-child-skill/SKILL.md")
}

func TestGenerateWorkspaceSkills_RoutingTableHasNoBlankLines(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "frontend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{
				{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
				{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
			},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)

	require.NoError(t, svc.GenerateWorkspaceSkills(context.Background()))

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	assertMarkdownTableHasNoBlankLines(t, overview, "## 路由表")
}

func TestGenerateWorkspaceSkills_DoesNotPersistRuntimeUserContext(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)

	ctx := runtimecontext.WithUserContext(context.Background(), "本次运行的一次性原文不能进入 workspace skill")
	require.NoError(t, svc.GenerateWorkspaceSkills(ctx))

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.NotContains(t, overview, "本次运行的一次性原文不能进入 workspace skill")
}

func TestGenerateWorkspaceSkills_UsesConfiguredTargetOnly(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetForProjectFn: func(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
			require.Fail(t, "workspace root generation should not read child profiles")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
		SkillsCfg: config.SkillsConfig{Paths: map[string]string{
			"claude": ".claude/skills/skills-seed-skills",
			"codex":  ".agents/skills/skills-seed-skills",
		}},
	}
	svc := NewWorkspaceGenerator(mockPattern, mockProfile, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)

	require.NoError(t, svc.GenerateWorkspaceSkills(context.Background()))
	require.FileExists(t, filepath.Join(projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-workspace", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".claude", "skills", "skills-seed-skills", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "skills-seed-skills", "SKILL.md"))

	rootSkill := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md")
	require.Contains(t, rootSkill, "[工作区概览](./references/workspace-overview.md)")
	require.Contains(t, rootSkill, "[跨项目规则](./references/cross-project-rules.md)")
	require.NotContains(t, rootSkill, "backend/.claude/skills/skills-seed-skills/SKILL.md")

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "未分析出明确契约路径")
	require.Contains(t, overview, "默认写入边界")
}

func TestGenerateWorkspaceSkills_UsesChildProjectConfigPath(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend", ".skills-seed"), 0755))
	childConfig := `
profile:
  name: "backend"
  mode: "project"
  language: "go"
  locale: "zh-CN"
  root_path: ""
agent:
  engine: "codex"
  commands:
    codex: "codex"
skills:
  target: "codex"
  paths:
    codex: ".agents/skills/custom-child-skill"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "backend", ".skills-seed", "config.yaml"), []byte(childConfig), 0644))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetForProjectFn: func(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
			require.Fail(t, "workspace root generation should not read child profiles")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Engine: "claude"},
	}
	svc := NewWorkspaceGenerator(mockPattern, mockProfile, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg, nil, nil)

	require.NoError(t, svc.GenerateWorkspaceSkills(context.Background()))
	require.FileExists(t, filepath.Join(projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "custom-child-skill", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".claude", "skills", "skills-seed-skills", "SKILL.md"))

	rootSkill := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md")
	assert.NotContains(t, rootSkill, "子项目自带 skills-seed")
	assert.NotContains(t, rootSkill, "backend/.skills-seed/config.yaml")
	assert.NotContains(t, rootSkill, "backend/.agents/skills/custom-child-skill/SKILL.md")

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	assert.Contains(t, overview, "子项目配置：`backend/.skills-seed/config.yaml`")
	assert.Contains(t, overview, "backend/.agents/skills/custom-child-skill/SKILL.md")
}

// --- test helpers ---

func readGeneratedFile(t *testing.T, root string, parts ...string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	require.NoError(t, err)
	return string(content)
}

func readFilePath(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func assertMarkdownTableHasNoBlankLines(t *testing.T, content, heading string) {
	t.Helper()

	start := strings.Index(content, heading)
	require.NotEqual(t, -1, start)
	section := content[start+len(heading):]
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	lines := strings.Split(section, "\n")
	firstTableLine := -1
	lastTableLine := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			if firstTableLine < 0 {
				firstTableLine = i
			}
			lastTableLine = i
		}
	}
	require.GreaterOrEqual(t, firstTableLine, 0)
	require.GreaterOrEqual(t, lastTableLine, firstTableLine)
	for i := firstTableLine; i <= lastTableLine; i++ {
		require.NotEmpty(t, strings.TrimSpace(lines[i]), "blank line inside markdown table under %s", heading)
	}
}

var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

func assertNoBrokenMarkdownLinks(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		content, err := os.ReadFile(path)
		require.NoError(t, err)

		for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(content), -1) {
			target := strings.TrimSpace(match[1])
			if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if fragmentIndex := strings.Index(target, "#"); fragmentIndex >= 0 {
				target = target[:fragmentIndex]
			}
			if target == "" {
				continue
			}

			targetPath := filepath.Clean(filepath.Join(filepath.Dir(path), target))
			_, err := os.Stat(targetPath)
			assert.NoErrorf(t, err, "broken link in %s: %s", path, match[1])
		}
		return nil
	})
	require.NoError(t, err)
}

type memoryWorkspaceProfileRepo struct {
	profile *domain.WorkspaceProfile
}

func (r *memoryWorkspaceProfileRepo) Get(ctx context.Context) (*domain.WorkspaceProfile, error) {
	if r.profile == nil {
		return nil, errors.New("workspace profile not found")
	}
	return r.profile, nil
}

func (r *memoryWorkspaceProfileRepo) Save(ctx context.Context, profile *domain.WorkspaceProfile) error {
	r.profile = profile
	return nil
}

type memoryWorkspaceSpecRepo struct {
	spec *domain.WorkspaceSpec
}

func (r *memoryWorkspaceSpecRepo) Get(ctx context.Context) (*domain.WorkspaceSpec, error) {
	if r.spec == nil {
		return nil, errors.New("workspace spec not found")
	}
	return r.spec, nil
}

func (r *memoryWorkspaceSpecRepo) Save(ctx context.Context, spec *domain.WorkspaceSpec) error {
	r.spec = spec
	return nil
}
