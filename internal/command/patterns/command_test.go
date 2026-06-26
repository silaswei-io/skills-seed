package patterns

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestPlanWorkspacePatternTargetsMatchesProjectMention(t *testing.T) {
	projects := []config.WorkspaceProjectConfig{
		{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend"},
		{ID: "cluster-manage", Path: "cluster_manage", Type: "backend"},
	}

	plan := planWorkspacePatternTargets("hsmwebapi 的 plugins 来自 plugins_custom.sh，改代码应修改源插件代码", projects)

	require.True(t, plan.Workspace)
	require.Equal(t, []string{"hsmwebapi"}, plan.Projects)
}

func TestAddCmdInWorkspaceRootDistributesPattern(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	childRoot := filepath.Join(workspaceRoot, "hsmwebapi")

	rootConfigRepo, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	rootCfg := rootConfigRepo.Get()
	rootCfg.Project.Name = "hsm-workspace"
	rootCfg.Project.Mode = domain.ModeWorkspace
	rootCfg.Project.RootPath = workspaceRoot
	rootCfg.Project.Language = "go"
	rootCfg.Agent.Engine = "mock"
	rootCfg.Agent.Commands = map[string]string{"mock": "mock"}
	rootCfg.Workspace.Projects = []config.WorkspaceProjectConfig{
		{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
	}
	require.NoError(t, rootConfigRepo.Update(rootCfg))

	childConfigRepo, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	childCfg := childConfigRepo.Get()
	childCfg.Project.Name = "hsmwebapi"
	childCfg.Project.Mode = domain.ModeProject
	childCfg.Project.RootPath = childRoot
	childCfg.Project.Language = "go"
	childCfg.Agent.Engine = "mock"
	childCfg.Agent.Commands = map[string]string{"mock": "mock"}
	require.NoError(t, childConfigRepo.Update(childCfg))

	rootPatternRepo, err := boltdb.NewPatternRepository(filepath.Join(workspaceRoot, ".skills-seed", "store", "project.db"))
	require.NoError(t, err)
	defer rootPatternRepo.Close()
	pattern := domain.NewPattern("plugin-source-editing-rule", "插件源码修改规范", domain.CategoryConfig)
	pattern.SetDescription("hsmwebapi plugins 应修改源插件代码")
	pattern.SetRule("当 hsmwebapi 的 plugins 由 plugins_custom.sh 拉取时，应该修改源插件代码")
	pattern.Confidence = 0.9
	mockAgent := &mocks.MockAgent{
		NameVal:      "mock",
		AvailableVal: true,
		UserDefinePatternFn: func(ctx context.Context, req *agent.UserDefinePatternRequest) (*agent.UserDefinePatternResult, error) {
			return &agent.UserDefinePatternResult{Pattern: pattern}, nil
		},
	}
	cont := &container.Container{
		SeedPath:    filepath.Join(workspaceRoot, ".skills-seed"),
		Config:      rootConfigRepo.Get(),
		ConfigRepo:  rootConfigRepo,
		PatternRepo: rootPatternRepo,
		Agent:       mockAgent,
		CuratorSvc:  curator.NewService(mockAgent, rootPatternRepo),
	}
	cmd := addCmd(cont)
	cmd.SetArgs([]string{"hsmwebapi 的 plugins 来自 plugins_custom.sh，改代码应修改源插件代码"})

	require.NoError(t, cmd.Execute())

	rootPattern, err := rootPatternRepo.Get(ctx, "plugin-source-editing-rule")
	require.NoError(t, err)
	require.Equal(t, "hsmwebapi", rootPattern.ProjectID)
	require.Equal(t, "hsmwebapi", rootPattern.ScopePath)

	childPatternRepo, err := boltdb.NewPatternRepository(filepath.Join(childRoot, ".skills-seed", "store", "project.db"))
	require.NoError(t, err)
	defer childPatternRepo.Close()
	childPattern, err := childPatternRepo.Get(ctx, "plugin-source-editing-rule")
	require.NoError(t, err)
	require.Empty(t, childPattern.ProjectID)
	require.Empty(t, childPattern.ScopePath)

}

func TestAddCmdRejectsContextFlag(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cmd := addCmd(&container.Container{})
	cmd.SetArgs([]string{"--context", "hsmwebapi 的 plugins 来自 plugins_custom.sh"})

	err := cmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown flag: --context")
}

func TestAddCmdRequiresDescription(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cmd := addCmd(&container.Container{})

	err := cmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "需要提供模式描述")
}

func TestDeleteCmdInProjectDeletesPattern(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	ctx := context.Background()
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")

	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.RootPath = projectRoot
	require.NoError(t, configRepo.Update(cfg))

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "store", "project.db"))
	require.NoError(t, err)
	defer patternRepo.Close()
	pattern := domain.NewPattern("plugin-source-editing-rule", "插件源码修改规范", domain.CategoryConfig)
	require.NoError(t, patternRepo.Save(ctx, pattern))

	cont := &container.Container{
		SeedPath:    seedPath,
		ConfigRepo:  configRepo,
		PatternRepo: patternRepo,
	}
	cmd := deleteCmd(cont)
	cmd.SetArgs([]string{"plugin-source-editing-rule"})

	require.NoError(t, cmd.Execute())

	deleted, err := patternRepo.Get(ctx, "plugin-source-editing-rule")
	require.Error(t, err)
	require.Nil(t, deleted)
}

func TestDeleteCmdRequiresPatternID(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cmd := deleteCmd(&container.Container{})

	err := cmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "需要提供 pattern ID")
}

func TestDeleteCmdInWorkspaceRootDeletesRootAndLinkedChildPattern(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	childRoot := filepath.Join(workspaceRoot, "hsmwebapi")

	rootConfigRepo, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	rootCfg := rootConfigRepo.Get()
	rootCfg.Project.Name = "hsm-workspace"
	rootCfg.Project.Mode = domain.ModeWorkspace
	rootCfg.Project.RootPath = workspaceRoot
	rootCfg.Workspace.Projects = []config.WorkspaceProjectConfig{
		{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
	}
	require.NoError(t, rootConfigRepo.Update(rootCfg))

	childConfigRepo, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	childCfg := childConfigRepo.Get()
	childCfg.Project.Name = "hsmwebapi"
	childCfg.Project.Mode = domain.ModeProject
	childCfg.Project.RootPath = childRoot
	require.NoError(t, childConfigRepo.Update(childCfg))

	rootPatternRepo, err := boltdb.NewPatternRepository(filepath.Join(workspaceRoot, ".skills-seed", "store", "project.db"))
	require.NoError(t, err)
	defer rootPatternRepo.Close()
	rootPattern := domain.NewPattern("plugin-source-editing-rule", "插件源码修改规范", domain.CategoryConfig)
	rootPattern.ProjectID = "hsmwebapi"
	rootPattern.ScopePath = "hsmwebapi"
	require.NoError(t, rootPatternRepo.Save(ctx, rootPattern))

	childPatternRepo, err := boltdb.NewPatternRepository(filepath.Join(childRoot, ".skills-seed", "store", "project.db"))
	require.NoError(t, err)
	childPattern := domain.NewPattern("plugin-source-editing-rule", "插件源码修改规范", domain.CategoryConfig)
	require.NoError(t, childPatternRepo.Save(ctx, childPattern))
	require.NoError(t, childPatternRepo.Close())

	cont := &container.Container{
		SeedPath:    filepath.Join(workspaceRoot, ".skills-seed"),
		ConfigRepo:  rootConfigRepo,
		PatternRepo: rootPatternRepo,
	}
	cmd := deleteCmd(cont)
	cmd.SetArgs([]string{"plugin-source-editing-rule"})

	require.NoError(t, cmd.Execute())

	deletedRoot, err := rootPatternRepo.Get(ctx, "plugin-source-editing-rule")
	require.Error(t, err)
	require.Nil(t, deletedRoot)
	childPatternRepo, err = boltdb.NewPatternRepository(filepath.Join(childRoot, ".skills-seed", "store", "project.db"))
	require.NoError(t, err)
	defer childPatternRepo.Close()
	deletedChild, err := childPatternRepo.Get(ctx, "plugin-source-editing-rule")
	require.Error(t, err)
	require.Nil(t, deletedChild)

}

func TestStatsCmdPrintsPatternMetricsAndHits(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	p := domain.NewPattern("domain-error-wrap", "领域错误包装", domain.CategoryError)
	p.Confidence = 0.8
	p.Metrics = domain.PatternMetrics{
		SpecificityScore: 0.72,
		GenericPenalty:   0.1,
		EffectiveScore:   0.66,
		EvidenceCount:    3,
	}
	lastHit := time.Date(2026, 5, 28, 10, 30, 0, 0, time.UTC)
	cont := &container.Container{
		PatternStats: &mocks.MockPatternStatsRepository{
			GetPatternHitStatsFn: func(ctx context.Context) ([]domain.PatternHitStats, error) {
				return []domain.PatternHitStats{
					{Pattern: *p, HitCount: 2, LastHitAt: lastHit},
				}, nil
			},
		},
	}
	cmd := statsCmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	text := out.String()
	require.Contains(t, text, "具体度")
	require.Contains(t, text, "命中数")
	require.Contains(t, text, "domain-error-wrap")
	require.Contains(t, text, "error")
	require.Contains(t, text, "0.72")
	require.Contains(t, text, "0.66")
	require.Contains(t, text, "2")
	require.Contains(t, text, "2026-05-28")
}

func TestShowCmdPrintsPatternDatabaseFields(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	createdAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 9, 10, 30, 0, 0, time.UTC)
	p := domain.NewPattern("business-create-order", "创建订单", domain.CategoryBusiness)
	p.Source = domain.SourceLearnedCurrent
	p.Confidence = 0.86
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	p.BusinessMethod = &domain.BusinessMethod{
		Name: "CreateOrder",
		CodeLocation: domain.CodeLocation{
			HistoricalLocation: "service/order.ts:10",
			CurrentLocation:    "service/order.ts:20",
			Status:             domain.CodeLocationStatusChanged,
			ChangeKinds:        []domain.CodeLocationChangeKind{domain.CodeLocationChangeMoved, domain.CodeLocationChangeInputsChanged},
			UpdatedAt:          updatedAt,
			Snapshot: &domain.SymbolSnapshot{
				Language:   "typescript",
				Kind:       "method",
				Name:       "createOrder",
				InputTypes: []string{"CreateOrderRequestV2"},
			},
		},
	}

	cont := &container.Container{
		PatternReader: &mocks.MockPatternRepository{
			GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
				return []domain.Pattern{*p}, nil
			},
		},
	}
	cmd := showCmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	text := out.String()
	require.Contains(t, text, "分类")
	require.Contains(t, text, "当前位置")
	require.Contains(t, text, "business-create-order")
	require.Contains(t, text, "business")
	require.Contains(t, text, "learned_current")
	require.Contains(t, text, "2026-06-01")
	require.Contains(t, text, "2026-06-09")
	require.Contains(t, text, "changed")
	require.Contains(t, text, "service/order.ts:20")
}

func TestShowCmdUsesPatternEvidenceLocationWhenBusinessMethodMissing(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	p := domain.NewPattern("error-wrap", "错误包装", domain.CategoryError)
	p.Source = domain.SourceLearnedCurrent
	p.Confidence = 0.9
	p.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/config.go", Line: 42, Symbol: "LoadConfig", Kind: "function", Description: "包装配置加载错误", Confidence: 0.88},
	}

	cont := &container.Container{
		PatternReader: &mocks.MockPatternRepository{
			GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
				return []domain.Pattern{*p}, nil
			},
			GetFn: func(ctx context.Context, id string) (*domain.Pattern, error) {
				require.Equal(t, "error-wrap", id)
				return p, nil
			},
		},
	}

	listCmd := showCmd(cont)
	var listOut bytes.Buffer
	listCmd.SetOut(&listOut)
	listCmd.SetErr(&listOut)
	require.NoError(t, listCmd.Execute())
	listText := listOut.String()
	require.Contains(t, listText, "evidence")
	require.Contains(t, listText, "internal/service/config.go:42")

	detailCmd := showCmd(cont)
	detailCmd.SetArgs([]string{"error-wrap"})
	var detailOut bytes.Buffer
	detailCmd.SetOut(&detailOut)
	detailCmd.SetErr(&detailOut)
	require.NoError(t, detailCmd.Execute())
	detailText := detailOut.String()
	require.Contains(t, detailText, "证据位置")
	require.Contains(t, detailText, "internal/service/config.go:42 | function | LoadConfig | 0.88 | 包装配置加载错误")
}

func TestShowCmdPrintsSinglePatternDetails(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	p := domain.NewPattern("business-create-order", "创建订单", domain.CategoryBusiness)
	p.Description = "创建订单并写入业务流水"
	p.Rule = "订单创建必须经过领域服务"
	p.GoodExample = "domainService.CreateOrder(ctx, req)"
	p.BadExample = "db.Insert(order)"
	p.Frequency = 3
	p.Metrics = domain.PatternMetrics{
		SpecificityScore: 0.71,
		EvidenceCount:    4,
		GenericPenalty:   0.12,
		EffectiveScore:   0.82,
	}
	p.Merged = true
	p.MergedFrom = []string{"legacy-create-order", "current-create-order"}
	p.Generated = true
	p.ProjectID = "order-service"
	p.ScopePath = "services/order"
	p.WorkspaceRole = "backend"
	p.BusinessMethod = &domain.BusinessMethod{
		Name:          "CreateOrder",
		Description:   "封装订单创建业务流程",
		Usage:         "创建订单时复用",
		Type:          "domain",
		Function:      "func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequestV2) error",
		Prerequisites: "已初始化 OrderService",
		Returns:       "创建失败时返回业务错误",
		CodeLocation: domain.CodeLocation{
			HistoricalLocation: "service/order.ts:10",
			CurrentLocation:    "service/order.ts:20",
			Status:             domain.CodeLocationStatusChanged,
			ChangeKinds:        []domain.CodeLocationChangeKind{domain.CodeLocationChangeMoved, domain.CodeLocationChangeInputsChanged},
			History: []domain.CodeLocationHistory{
				{
					Location:    "service/order.ts:10",
					Status:      domain.CodeLocationStatusMoved,
					ChangeKinds: []domain.CodeLocationChangeKind{domain.CodeLocationChangeMoved},
					ChangedAt:   time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC),
					Note:        "移动到新的领域服务",
				},
			},
			Snapshot: &domain.SymbolSnapshot{
				Language:   "typescript",
				Kind:       "method",
				Name:       "createOrder",
				InputTypes: []string{"CreateOrderRequestV2"},
			},
		},
	}

	cont := &container.Container{
		PatternReader: &mocks.MockPatternRepository{
			GetFn: func(ctx context.Context, id string) (*domain.Pattern, error) {
				require.Equal(t, "business-create-order", id)
				return p, nil
			},
		},
	}
	cmd := showCmd(cont)
	cmd.SetArgs([]string{"business-create-order"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	text := out.String()
	require.Contains(t, text, "id")
	require.Contains(t, text, "business-create-order")
	require.Contains(t, text, "描述")
	require.Contains(t, text, "创建订单并写入业务流水")
	require.Contains(t, text, "正例")
	require.Contains(t, text, "domainService.CreateOrder(ctx, req)")
	require.Contains(t, text, "反例")
	require.Contains(t, text, "db.Insert(order)")
	require.Contains(t, text, "具体度")
	require.Contains(t, text, "0.71")
	require.Contains(t, text, "证据数")
	require.Contains(t, text, "4")
	require.Contains(t, text, "泛化惩罚")
	require.Contains(t, text, "0.12")
	require.Contains(t, text, "有效分")
	require.Contains(t, text, "0.82")
	require.Contains(t, text, "已合并")
	require.Contains(t, text, "true")
	require.Contains(t, text, "合并来源")
	require.Contains(t, text, "legacy-create-order,current-create-order")
	require.Contains(t, text, "已生成")
	require.Contains(t, text, "子项目")
	require.Contains(t, text, "order-service")
	require.Contains(t, text, "范围路径")
	require.Contains(t, text, "services/order")
	require.Contains(t, text, "工作区角色")
	require.Contains(t, text, "backend")
	require.Contains(t, text, "方法说明")
	require.Contains(t, text, "封装订单创建业务流程")
	require.Contains(t, text, "使用场景")
	require.Contains(t, text, "创建订单时复用")
	require.Contains(t, text, "方法类型")
	require.Contains(t, text, "domain")
	require.Contains(t, text, "函数签名")
	require.Contains(t, text, "func (s *OrderService) CreateOrder")
	require.Contains(t, text, "前置条件")
	require.Contains(t, text, "已初始化 OrderService")
	require.Contains(t, text, "返回值")
	require.Contains(t, text, "创建失败时返回业务错误")
	require.Contains(t, text, "当前位置")
	require.Contains(t, text, "service/order.ts:20")
	require.Contains(t, text, "历史位置")
	require.Contains(t, text, "service/order.ts:10")
	require.Contains(t, text, "变更类型")
	require.Contains(t, text, "moved,inputs_changed")
	require.Contains(t, text, "快照语言")
	require.Contains(t, text, "typescript")
	require.Contains(t, text, "输入类型")
	require.Contains(t, text, "CreateOrderRequestV2")
	require.Contains(t, text, "位置历史")
	require.Contains(t, text, "service/order.ts:10 | moved | moved | 2026-06-10 08:00:00 | 移动到新的领域服务")
}

func TestShowCmdPrintsJSON(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	p := domain.NewPattern("business-create-order", "创建订单", domain.CategoryBusiness)
	p.BusinessMethod = &domain.BusinessMethod{
		Name: "CreateOrder",
		CodeLocation: domain.CodeLocation{
			CurrentLocation: "service/order.ts:20",
			Status:          domain.CodeLocationStatusValid,
		},
	}

	cont := &container.Container{
		PatternReader: &mocks.MockPatternRepository{
			GetFn: func(ctx context.Context, id string) (*domain.Pattern, error) {
				return p, nil
			},
		},
	}
	cmd := showCmd(cont)
	cmd.SetArgs([]string{"business-create-order", "--format", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	var got domain.Pattern
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Equal(t, "business-create-order", got.ID)
	require.Equal(t, "service/order.ts:20", got.BusinessMethod.CodeLocation.CurrentLocation)
	require.Equal(t, domain.CodeLocationStatusValid, got.BusinessMethod.CodeLocation.Status)
}
