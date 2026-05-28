package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/utils"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

// GeneratorService 生成服务
type GeneratorService struct {
	patternRepo       domain.PatternRepository
	profileRepo       domain.ProjectProfileRepository
	scopedProfileRepo domain.ScopedProjectProfileRepository
	projectSpecRepo   domain.ProjectSpecRepository
	scopedSpecRepo    domain.ScopedProjectSpecRepository
	skillsLoader      *skills.Loader
	agent             agent.Agent // 添加AI Agent
	configRepo        config.Reader
}

type projectOverviewTemplateData struct {
	domain.ProjectProfile
	OverviewReferences []skills.ReferenceItem
}

type projectSpecTemplateData struct {
	domain.ProjectSpec
}

type categoryReferenceMeta struct {
	Group       string
	Title       string
	Description string
}

type workspaceSkillTemplateData struct {
	ProgramVersion      string
	SkillsTemplatesHash string
	SkillName           string
	ProjectName         string
	WorkspaceName       string
	WorkspaceFacts      string
	ProjectCount        int
	Projects            []workspaceProjectTemplateData
	UserContext         string
	HasUserContext      bool
	Shared              []domain.WorkspacePath
	Contracts           []domain.WorkspacePath
	Infra               []domain.WorkspacePath
	Dependencies        []domain.WorkspaceDependency
	ImpactRoutes        []domain.WorkspaceRoute
	Routing             []domain.WorkspaceRoute
	Rules               []domain.WorkspaceRule
	ChangeOrder         []string
	ParallelGuidance    []domain.WorkspaceParallelGuidance
	LoadMultipleWhen    []domain.WorkspaceLoadMultipleSkill
	HasWorkspaceFacts   bool
	HasShared           bool
	HasContracts        bool
	HasInfra            bool
	HasDependencies     bool
	HasImpactRoutes     bool
	HasRouting          bool
	HasRules            bool
	HasChangeOrder      bool
	HasParallelGuidance bool
	HasLoadMultipleWhen bool
}

type workspaceProjectTemplateData struct {
	config.WorkspaceProjectConfig
	SkillName             string
	SkillPath             string
	ProjectSpecPath       string
	SkillSummary          string
	Responsibility        string
	Frameworks            []string
	SelfManaged           bool
	SelfManagedConfigPath string
	UsesChildConfig       bool
	HasFrameworks         bool
}

type childSkillTarget struct {
	OutputPath      string
	ConfigRepo      config.Reader
	UsesChildConfig bool
	ConfigPath      string
}

type workspaceGenerateInputData struct {
	Name       string                          `json:"name"`
	RootPath   string                          `json:"root_path"`
	Projects   []workspaceGenerateInputProject `json:"projects"`
	Shared     []config.WorkspacePathConfig    `json:"shared,omitempty"`
	Contracts  []config.WorkspacePathConfig    `json:"contracts,omitempty"`
	Infra      []config.WorkspacePathConfig    `json:"infra,omitempty"`
	ConfigPath string                          `json:"config_path,omitempty"`
}

type workspaceGenerateInputProject struct {
	ID              string `json:"id"`
	Path            string `json:"path"`
	Type            string `json:"type"`
	Language        string `json:"language"`
	SkillPath       string `json:"skill_path,omitempty"`
	ProjectSpecPath string `json:"project_spec_path,omitempty"`
	ConfigPath      string `json:"config_path,omitempty"`
	SelfManaged     bool   `json:"self_managed,omitempty"`
}

// ManualSkillExistsError 表示目标目录已有非 skills-seed 生成的 SKILL.md
type ManualSkillExistsError struct {
	Path string
}

func (e *ManualSkillExistsError) Error() string {
	return i18n.GetWithParams("GenerateManualSkillExists", map[string]interface{}{"Path": e.Path})
}

// NewGeneratorService 创建生成服务
func NewGeneratorService(
	patternRepo domain.PatternRepository,
	profileRepo domain.ProjectProfileRepository,
	skillsLoader *skills.Loader,
	ag agent.Agent,
	configRepo config.Reader,
) *GeneratorService {
	scopedProfileRepo, _ := profileRepo.(domain.ScopedProjectProfileRepository)
	projectSpecRepo, _ := profileRepo.(domain.ProjectSpecRepository)
	scopedSpecRepo, _ := profileRepo.(domain.ScopedProjectSpecRepository)
	return &GeneratorService{
		patternRepo:       patternRepo,
		profileRepo:       profileRepo,
		scopedProfileRepo: scopedProfileRepo,
		projectSpecRepo:   projectSpecRepo,
		scopedSpecRepo:    scopedSpecRepo,
		skillsLoader:      skillsLoader,
		agent:             ag,
		configRepo:        configRepo,
	}
}

// GenerateSkills 生成 Skills 文件夹
func (s *GeneratorService) GenerateSkills(ctx context.Context, outputPath string) error {
	if s.configRepo != nil && s.configRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return s.generateWorkspaceSkills(ctx)
	}
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_skills",
		"output_path", outputPath,
	)

	resolvedOutputPath, err := s.resolveOutputPath(outputPath)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.resolve_output_path",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	// 1. 获取所有模式
	patterns, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.load_patterns",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return err
	}

	if len(patterns) == 0 {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.generate_skills",
			"duration", time.Since(startedAt),
			"resolved_output_path", resolvedOutputPath,
			"patterns_count", 0,
			"skipped", true,
		)
		return nil
	}

	summaryResult, err := s.generateSkillsSummary(ctx, patterns, resolvedOutputPath, startedAt)
	if err != nil {
		return err
	}

	// 6. 计算统计信息（用于 writeSkillsOutput）
	stats := s.calculateStats(patterns)

	profile, err := s.loadProjectProfile(ctx)
	if err != nil {
		return err
	}
	profile = cleanProjectProfile(profile)
	spec := s.projectSpecFromProfileAndPatterns(profile, patterns, config.WorkspaceProjectConfig{})
	if s.projectSpecRepo != nil {
		if err := s.projectSpecRepo.SaveSpec(ctx, spec); err != nil {
			return err
		}
	}
	if err := s.writeSkillsOutput(ctx, resolvedOutputPath, patterns, summaryResult, stats, profile, spec, generatedSkillName(s.configRepo.GetProjectConfig().Name)); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_skills",
			"duration", time.Since(startedAt),
			"patterns_count", len(patterns),
			"resolved_output_path", resolvedOutputPath,
			"error", err,
		)
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_skills",
		"duration", time.Since(startedAt),
		"patterns_count", len(patterns),
		"resolved_output_path", resolvedOutputPath,
		"categories_count", len(stats.ByCategory),
	)
	return nil
}

func (s *GeneratorService) generateSkillsSummary(ctx context.Context, patterns []domain.Pattern, resolvedOutputPath string, startedAt time.Time) (*agent.GenerateSkillsResult, error) {
	userContext := runtimecontext.UserContext(ctx)

	// 序列化模式摘要为 JSON，不把代码示例直接发送给 Agent
	patternsJSONBytes, err := json.MarshalIndent(summarizePatternsForAgent(patterns), "", "  ")
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.marshal_patterns",
			"duration", time.Since(startedAt),
			"patterns_count", len(patterns),
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("GeneratorMarshalPatternsFailed"), err)
	}
	patternsJSON := string(patternsJSONBytes)

	// 记录现有 SKILL.md 路径，由 Agent 按需自行读取
	existingSkillsPath := ""
	skillPath := filepath.Join(resolvedOutputPath, "SKILL.md")
	if _, err := os.Stat(skillPath); err == nil {
		existingSkillsPath = skillPath
	}

	summaryReq := &agent.GenerateSkillsRequest{
		PatternsJSON:       patternsJSON,
		PatternsCount:      len(patterns),
		ExistingSkillsPath: existingSkillsPath,
		ProjectName:        s.configRepo.GetProjectConfig().Name,
		Language:           s.configRepo.GetProjectConfig().Language,
		UserContext:        userContext,
	}

	summaryResult, err := s.agent.GenerateSkillsSummary(ctx, summaryReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_summary",
			"duration", time.Since(startedAt),
			"patterns_count", len(patterns),
			"patterns_json_length", len(patternsJSON),
			"existing_skills_path", existingSkillsPath,
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("GeneratorGenerateSummaryFailed"), err)
	}
	return summaryResult, nil
}

func patternImportance(confidence float64) string {
	switch {
	case confidence >= 0.85:
		return "high"
	case confidence >= 0.6:
		return "medium"
	default:
		return "low"
	}
}

// GenerateWorkspaceSkills 只生成工作区根 skill，子项目 skill 由各子仓自己生成
func (s *GeneratorService) GenerateWorkspaceSkills(ctx context.Context) error {
	return s.generateWorkspaceSkills(ctx)
}

func (s *GeneratorService) generateWorkspaceSkills(ctx context.Context) error {
	startedAt := time.Now()
	projectConfig := s.configRepo.GetProjectConfig()
	workspaceConfig := s.configRepo.GetWorkspaceConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}

	projectRoot := projectConfig.RootPath
	rootOutputPath, err := s.workspaceRootOutputPath(projectRoot, projectConfig.Name)
	if err != nil {
		return err
	}
	profile, spec, err := s.analyzeWorkspaceForGenerate(ctx, projectConfig, workspaceConfig)
	if err != nil {
		return err
	}
	if err := s.writeWorkspaceRootSkill(ctx, rootOutputPath, projectConfig, workspaceConfig, profile, spec); err != nil {
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_workspace_skills",
		"duration", time.Since(startedAt),
		"projects_count", len(workspaceConfig.Projects),
	)
	return nil
}

func (s *GeneratorService) analyzeWorkspaceForGenerate(ctx context.Context, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig) (*domain.WorkspaceProfile, *domain.WorkspaceSpec, error) {
	userContext := runtimecontext.UserContext(ctx)
	if s.agent == nil {
		return nil, nil, fmt.Errorf("%s", i18n.Get("GeneratorGenerateSummaryFailed"))
	}

	workspaceName := workspaceNameOrDefault(projectConfig.Name)
	projectRoot := projectConfig.RootPath
	workspaceInput, err := s.workspaceGenerateInput(projectConfig, workspaceConfig)
	if err != nil {
		return nil, nil, err
	}
	runtimeDir := filepath.Join(projectRoot, ".skills-seed", "memory", "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return nil, nil, err
	}
	tmpDir, err := os.MkdirTemp(runtimeDir, "skills-seed-workspace-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath, err := writeJSONFile(filepath.Join(tmpDir, "workspace-input.json"), workspaceInput)
	if err != nil {
		return nil, nil, err
	}
	userContextPath := ""
	if userContext != "" {
		userContextPath = filepath.Join(tmpDir, "user-context.md")
		if err := os.WriteFile(userContextPath, []byte(userContext+"\n"), 0600); err != nil {
			return nil, nil, err
		}
	}

	profile, err := s.agent.AnalyzeWorkspaceProfile(ctx, &agent.AnalyzeWorkspaceProfileRequest{
		WorkspaceName:      workspaceName,
		WorkspaceRoot:      projectRoot,
		WorkspaceInputPath: inputPath,
		UserContextPath:    userContextPath,
	})
	if err != nil {
		return nil, nil, err
	}
	profile = mergeWorkspaceProfile(workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig), profile)
	profilePath, err := writeJSONFile(filepath.Join(tmpDir, "workspace-profile.json"), profile)
	if err != nil {
		return nil, nil, err
	}

	spec, err := s.agent.AnalyzeWorkspaceSpec(ctx, &agent.AnalyzeWorkspaceSpecRequest{
		WorkspaceName:        workspaceName,
		WorkspaceRoot:        projectRoot,
		WorkspaceInputPath:   inputPath,
		WorkspaceProfilePath: profilePath,
		UserContextPath:      userContextPath,
	})
	if err != nil {
		return nil, nil, err
	}
	spec = mergeWorkspaceSpec(workspaceSpecFromProfile(profile), spec)
	return profile, spec, nil
}

func (s *GeneratorService) workspaceGenerateInput(projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig) (workspaceGenerateInputData, error) {
	projectRoot := projectConfig.RootPath
	input := workspaceGenerateInputData{
		Name:       workspaceNameOrDefault(projectConfig.Name),
		RootPath:   projectRoot,
		Projects:   make([]workspaceGenerateInputProject, 0, len(workspaceConfig.Projects)),
		Shared:     workspaceConfig.Shared,
		Contracts:  workspaceConfig.Contracts,
		Infra:      workspaceConfig.Infra,
		ConfigPath: filepath.Join(projectRoot, ".skills-seed", "config.yaml"),
	}
	for _, project := range workspaceConfig.Projects {
		target, err := s.childSkillTarget(projectRoot, project)
		if err != nil {
			return workspaceGenerateInputData{}, err
		}
		configPath := ""
		if target.UsesChildConfig {
			configPath = relativeWorkspacePath(projectRoot, target.ConfigPath)
		}
		input.Projects = append(input.Projects, workspaceGenerateInputProject{
			ID:              project.ID,
			Path:            project.Path,
			Type:            project.Type,
			Language:        project.Language,
			SkillPath:       filepath.ToSlash(filepath.Join(relativeWorkspacePath(projectRoot, target.OutputPath), "SKILL.md")),
			ProjectSpecPath: filepath.ToSlash(filepath.Join(relativeWorkspacePath(projectRoot, target.OutputPath), "references", "project-spec.md")),
			ConfigPath:      filepath.ToSlash(configPath),
			SelfManaged:     target.UsesChildConfig,
		})
	}
	return input, nil
}

func writeJSONFile(path string, value interface{}) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func workspaceNameOrDefault(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "workspace"
	}
	return name
}

func mergeWorkspaceProfile(base, analyzed *domain.WorkspaceProfile) *domain.WorkspaceProfile {
	if base == nil {
		base = &domain.WorkspaceProfile{}
	}
	if analyzed == nil {
		return base
	}
	if analyzed.Name != "" {
		base.Name = analyzed.Name
	}
	if analyzed.RootPath != "" {
		base.RootPath = analyzed.RootPath
	}
	base.Summary = analyzed.Summary
	base.Projects = mergeWorkspaceProjects(base.Projects, analyzed.Projects)
	base.Shared = chooseWorkspacePaths(base.Shared, analyzed.Shared)
	base.Contracts = chooseWorkspacePaths(base.Contracts, analyzed.Contracts)
	base.Infra = chooseWorkspacePaths(base.Infra, analyzed.Infra)
	base.Dependencies = analyzed.Dependencies
	base.ImpactRoutes = analyzed.ImpactRoutes
	base.GeneratedAt = analyzed.GeneratedAt
	return base
}

func mergeWorkspaceSpec(base, analyzed *domain.WorkspaceSpec) *domain.WorkspaceSpec {
	if base == nil {
		base = &domain.WorkspaceSpec{}
	}
	if analyzed == nil {
		return base
	}
	if analyzed.Name != "" {
		base.Name = analyzed.Name
	}
	if analyzed.RootPath != "" {
		base.RootPath = analyzed.RootPath
	}
	if len(analyzed.Projects) > 0 {
		base.Projects = analyzed.Projects
	}
	if len(analyzed.Routing) > 0 {
		base.Routing = analyzed.Routing
	}
	if len(analyzed.Rules) > 0 {
		base.Rules = analyzed.Rules
	}
	base.ChangeOrder = analyzed.ChangeOrder
	base.ParallelAgentGuidance = analyzed.ParallelAgentGuidance
	base.LoadMultipleSkillsWhen = analyzed.LoadMultipleSkillsWhen
	if analyzed.GeneratedAt != "" {
		base.GeneratedAt = analyzed.GeneratedAt
	}
	return base
}

func workspaceSpecFromProfile(profile *domain.WorkspaceProfile) *domain.WorkspaceSpec {
	if profile == nil {
		return &domain.WorkspaceSpec{}
	}
	routing := make([]domain.WorkspaceRoute, 0, len(profile.Projects)+len(profile.Shared)+len(profile.Contracts)+len(profile.Infra)+len(profile.ImpactRoutes))
	for _, project := range profile.Projects {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(project.Path, "**")),
			ProjectIDs:  []string{project.ID},
			Reason:      "子项目路径只路由到该子项目的独立 skill",
		})
	}
	for _, route := range profile.ImpactRoutes {
		routing = append(routing, route)
	}
	projectIDs := workspaceProfileProjectIDs(profile.Projects)
	for _, path := range profile.Contracts {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(append(append([]string{}, path.Producers...), path.Consumers...), projectIDs),
			Reason:      firstNonEmpty(path.Description, "契约路径变更需要检查生产者、消费者和生成物"),
		})
	}
	for _, path := range profile.Shared {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(path.Consumers, projectIDs),
			Reason:      firstNonEmpty(path.Description, "共享代码变更需要检查所有导入方或复用方"),
		})
	}
	for _, path := range profile.Infra {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(path.AffectedProjects, projectIDs),
			Reason:      firstNonEmpty(path.Description, "基础设施变更需要检查受部署或运行时配置影响的子项目"),
		})
	}
	return &domain.WorkspaceSpec{
		Name:     profile.Name,
		RootPath: profile.RootPath,
		Projects: profile.Projects,
		Routing:  routing,
		Rules: []domain.WorkspaceRule{
			{
				Title:       "跨项目改动先定边界",
				Description: "修改契约、共享代码或基础设施前，先确认受影响子项目并读取对应 skill。",
				AppliesTo:   projectIDs,
			},
		},
	}
}

func mergeWorkspaceProjects(base, analyzed []domain.WorkspaceProject) []domain.WorkspaceProject {
	if len(base) == 0 {
		return analyzed
	}
	byID := make(map[string]domain.WorkspaceProject, len(analyzed))
	for _, project := range analyzed {
		byID[project.ID] = project
	}
	result := make([]domain.WorkspaceProject, 0, len(base))
	for _, project := range base {
		if analyzedProject, ok := byID[project.ID]; ok {
			if analyzedProject.Path != "" {
				project.Path = analyzedProject.Path
			}
			if analyzedProject.Type != "" {
				project.Type = analyzedProject.Type
			}
			if analyzedProject.Language != "" {
				project.Language = analyzedProject.Language
			}
			project.Responsibility = analyzedProject.Responsibility
			project.Frameworks = analyzedProject.Frameworks
		}
		result = append(result, project)
	}
	return result
}

func chooseWorkspacePaths(base, analyzed []domain.WorkspacePath) []domain.WorkspacePath {
	if len(analyzed) > 0 {
		return analyzed
	}
	return base
}

func workspaceProfileProjectIDs(projects []domain.WorkspaceProject) []string {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project.ID != "" {
			ids = append(ids, project.ID)
		}
	}
	return ids
}

func nonEmptyStrings(values, fallback []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	if len(result) > 0 {
		return result
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *GeneratorService) workspaceChildConfig(projectRoot string, project config.WorkspaceProjectConfig) (config.Reader, bool, string, error) {
	configPath := filepath.Join(projectRoot, filepath.FromSlash(project.Path), ".skills-seed", "config.yaml")
	_, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, configPath, nil
		}
		return nil, false, configPath, err
	}
	locale := ""
	if s.skillsLoader != nil {
		locale = s.skillsLoader.GetLocale()
	}
	repo, err := config.NewRepository(filepath.Dir(configPath), locale)
	if err != nil {
		return nil, true, configPath, err
	}
	return repo, true, configPath, nil
}

func summarizePatternsForAgent(patterns []domain.Pattern) []map[string]interface{} {
	summary := make([]map[string]interface{}, 0, len(patterns))
	for _, p := range patterns {
		summary = append(summary, map[string]interface{}{
			"id":              p.ID,
			"name":            p.Name,
			"category":        string(p.Category),
			"description":     p.Description,
			"rule":            p.Rule,
			"confidence":      p.Confidence,
			"frequency":       p.Frequency,
			"source":          string(p.Source),
			"business_method": p.BusinessMethod,
		})
	}
	return summary
}

func (s *GeneratorService) writeSkillsOutput(ctx context.Context, outputPath string, patterns []domain.Pattern, summaryResult *agent.GenerateSkillsResult, stats *Stats, profile *domain.ProjectProfile, spec *domain.ProjectSpec, skillName string) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.write_skills_output",
		"output_path", outputPath,
		"patterns_count", len(patterns),
	)

	if len(patterns) == 0 {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.write_skills_output",
			"duration", time.Since(startedAt),
			"patterns_count", 0,
			"skipped", true,
		)
		return nil
	}
	if summaryResult == nil {
		return fmt.Errorf("%s", i18n.Get("GeneratorGenerateSummaryFailed"))
	}

	// 确保分类摘要非 nil，避免模板侧判空复杂化
	if summaryResult.CategorySummaries == nil {
		summaryResult.CategorySummaries = map[string]agent.CategorySummary{}
	}
	summaryResult.CategorySummaries = s.ensureCategorySummaries(patterns, summaryResult.CategorySummaries)

	mainPath := filepath.Join(outputPath, "SKILL.md")
	if err := s.ensureSkillWritable(mainPath); err != nil {
		return err
	}

	// 5. 确保输出目录存在
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.create_output_dir",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	projectConfig := s.configRepo.GetProjectConfig()
	skillsTemplatesHash := metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS))
	if profile == nil {
		return fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
	}
	profile = cleanProjectProfile(profile)
	if skillName == "" {
		skillName = generatedSkillName(projectConfig.Name)
	}
	templateProjectName := projectConfig.Name
	if profile.ProjectName != "" {
		templateProjectName = profile.ProjectName
	}
	templateLanguage := projectConfig.Language
	if profile.Language != "" {
		templateLanguage = profile.Language
	}

	// 准备模板数据
	data := map[string]interface{}{
		"ProgramVersion":      metadata.ProgramVersion,
		"SkillsTemplatesHash": skillsTemplatesHash,
		"ProjectName":         templateProjectName,
		"SkillName":           skillName,
		"Language":            templateLanguage,
		"PatternCount":        len(patterns),
		"AvgConfidence":       stats.AvgConfidence * 100,
		"Categories":          len(summaryResult.CategorySummaries),
		"LastUpdated":         time.Now().Format("2006-01-02 15:04:05"),
		"CategorySummaries":   summaryResult.CategorySummaries,
		"KeyPatterns":         summaryResult.KeyPatterns,
		"BusinessRules":       summaryResult.BusinessRules,
		"BestPractices":       summaryResult.BestPractices,
		"CommonPatterns":      summaryResult.CommonPatterns,
		"STATS":               stats,
		"OverviewReferences":  profileReferenceItems(profile, s.skillsLoader.GetLocale(), "./references/"),
		"ReferenceGroups":     categoryReferenceGroups(patterns, s.skillsLoader.GetLocale()),
	}

	// 生成主 SKILL.md 文件
	mainContent, err := s.skillsLoader.Render("skill", data)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.render_skill_template",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return err
	}

	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.write_skill_file",
			"duration", time.Since(startedAt),
			"path", mainPath,
			"error", err,
		)
		return err
	}

	if err := s.generateAgentMetadata(outputPath, data); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_agent_metadata",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	// 7. 生成参考文档
	if err := s.generateReferenceFiles(ctx, outputPath, summaryResult.CategorySummaries, patterns, profile, spec); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_reference_files",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.write_skills_output",
		"duration", time.Since(startedAt),
		"main_path", mainPath,
		"main_content_length", len(mainContent),
		"patterns_count", len(patterns),
		"categories_count", len(summaryResult.CategorySummaries),
	)
	return nil
}

func (s *GeneratorService) ensureSkillWritable(skillPath string) error {
	content, err := os.ReadFile(skillPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if strings.Contains(string(content), "generated-by: skills-seed") {
		return nil
	}
	return &ManualSkillExistsError{Path: skillPath}
}

func (s *GeneratorService) loadProjectProfile(ctx context.Context) (*domain.ProjectProfile, error) {
	if s.profileRepo == nil {
		return nil, fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
	}

	profile, err := s.profileRepo.Get(ctx)
	if err != nil {
		if errors.Is(err, profilestore.ErrProfileNotFound) {
			return nil, fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
		}
		return nil, err
	}
	return profile, nil
}

func (s *GeneratorService) generateAgentMetadata(outputPath string, data interface{}) error {
	files, err := s.skillsLoader.RenderAgentMetadataFiles(data)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		outputFile := filepath.Join(outputPath, file.Path)
		if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(outputFile, []byte(file.Content), 0644); err != nil {
			return err
		}

		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.generate_agent_metadata",
			"path", outputFile,
			"content_length", len(file.Content),
		)
	}
	return nil
}

func (s *GeneratorService) resolveOutputPath(outputPath string) (string, error) {
	projectRoot := ""
	if s.configRepo != nil {
		projectRoot = s.configRepo.GetProjectConfig().RootPath
	}
	return utils.ResolvePath(projectRoot, outputPath)
}

func (s *GeneratorService) ensureCategorySummaries(
	patterns []domain.Pattern,
	summaries map[string]agent.CategorySummary,
) map[string]agent.CategorySummary {
	if summaries == nil {
		summaries = map[string]agent.CategorySummary{}
	}

	byCategory := make(map[string][]domain.Pattern)
	for _, pattern := range patterns {
		category := string(pattern.Category)
		if category == "" {
			continue
		}
		byCategory[category] = append(byCategory[category], pattern)
	}

	for category, categoryPatterns := range byCategory {
		summary := summaries[category]
		if summary.Category == "" {
			summary.Category = category
		}
		if len(summary.Patterns) == 0 {
			summary.Patterns = patternNames(categoryPatterns)
		}
		if summary.Summary == "" {
			summary.Summary = fmt.Sprintf("%s 分类包含 %d 个项目特定模式：%s。", category, len(categoryPatterns), strings.Join(summary.Patterns, "、"))
		}
		summaries[category] = summary
	}

	return summaries
}

func patternNames(patterns []domain.Pattern) []string {
	names := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.Name != "" {
			names = append(names, pattern.Name)
		}
	}
	return names
}

func categoryNamesWithPatterns(patterns []domain.Pattern) []string {
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		category := string(pattern.Category)
		if category == "" {
			continue
		}
		seen[category] = true
	}

	categories := make([]string, 0, len(seen))
	for category := range seen {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
}

func categoryReferenceGroups(patterns []domain.Pattern, locale string) []skills.ReferenceGroup {
	groupOrder := []string{
		localizedText(locale, "架构与结构", "Architecture & Structure"),
		localizedText(locale, "业务与领域", "Business & Domain"),
		localizedText(locale, "技术模式", "Technical Patterns"),
		localizedText(locale, "高级主题", "Advanced Topics"),
		localizedText(locale, "其他模式", "Other Patterns"),
	}
	groupsByTitle := make(map[string]*skills.ReferenceGroup, len(groupOrder))
	for _, title := range groupOrder {
		groupsByTitle[title] = &skills.ReferenceGroup{Title: title}
	}

	for _, category := range categoryNamesWithPatterns(patterns) {
		meta := categoryReferenceMetadata(category, locale)
		group, ok := groupsByTitle[meta.Group]
		if !ok {
			group = groupsByTitle[groupOrder[len(groupOrder)-1]]
		}
		group.Items = append(group.Items, skills.ReferenceItem{
			Title:       meta.Title,
			Description: meta.Description,
			Path:        "./references/patterns/" + category + ".md",
		})
	}

	groups := make([]skills.ReferenceGroup, 0, len(groupOrder))
	for _, title := range groupOrder {
		group := groupsByTitle[title]
		if len(group.Items) > 0 {
			groups = append(groups, *group)
		}
	}
	return groups
}

func categoryReferenceMetadata(category, locale string) categoryReferenceMeta {
	groupArchitecture := localizedText(locale, "架构与结构", "Architecture & Structure")
	groupBusiness := localizedText(locale, "业务与领域", "Business & Domain")
	groupTechnical := localizedText(locale, "技术模式", "Technical Patterns")
	groupAdvanced := localizedText(locale, "高级主题", "Advanced Topics")
	groupOther := localizedText(locale, "其他模式", "Other Patterns")

	metadataByCategory := map[string]categoryReferenceMeta{
		string(domain.CategoryStructure): {
			Group:       groupArchitecture,
			Title:       localizedText(locale, "结构模式", "Structure Patterns"),
			Description: localizedText(locale, "代码组织模式", "Code organization patterns"),
		},
		string(domain.CategoryNaming): {
			Group:       groupArchitecture,
			Title:       localizedText(locale, "命名模式", "Naming Patterns"),
			Description: localizedText(locale, "命名约定", "Naming conventions"),
		},
		string(domain.CategoryBusiness): {
			Group:       groupBusiness,
			Title:       localizedText(locale, "业务模式", "Business Patterns"),
			Description: localizedText(locale, "业务逻辑模式", "Business logic patterns"),
		},
		string(domain.CategoryDatabase): {
			Group:       groupBusiness,
			Title:       localizedText(locale, "数据库模式", "Database Patterns"),
			Description: localizedText(locale, "数据库操作", "Database operations"),
		},
		string(domain.CategoryAPI): {
			Group:       groupBusiness,
			Title:       localizedText(locale, "API 模式", "API Patterns"),
			Description: localizedText(locale, "API 设计", "API design"),
		},
		string(domain.CategoryError): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "错误处理", "Error Handling"),
			Description: localizedText(locale, "错误处理模式", "Error handling patterns"),
		},
		string(domain.CategoryMiddleware): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "中间件模式", "Middleware Patterns"),
			Description: localizedText(locale, "中间件设计", "Middleware design"),
		},
		string(domain.CategoryConfig): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "配置模式", "Config Patterns"),
			Description: localizedText(locale, "配置管理", "Configuration management"),
		},
		string(domain.CategoryUtils): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "工具模式", "Utils Patterns"),
			Description: localizedText(locale, "工具方法", "Utility methods"),
		},
		string(domain.CategoryConcurrency): {
			Group:       groupAdvanced,
			Title:       localizedText(locale, "并发编程", "Concurrency Patterns"),
			Description: localizedText(locale, "并发编程模式", "Concurrency programming patterns"),
		},
		string(domain.CategoryTesting): {
			Group:       groupAdvanced,
			Title:       localizedText(locale, "测试模式", "Testing Patterns"),
			Description: localizedText(locale, "测试约定", "Testing conventions"),
		},
	}

	if meta, ok := metadataByCategory[category]; ok {
		return meta
	}
	return categoryReferenceMeta{
		Group:       groupOther,
		Title:       category,
		Description: localizedText(locale, "项目特定模式", "Project-specific patterns"),
	}
}

func profileReferenceItems(profile *domain.ProjectProfile, locale, prefix string) []skills.ReferenceItem {
	if profile == nil {
		return nil
	}

	items := make([]skills.ReferenceItem, 0, 3)
	if len(profile.BusinessMethods) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       localizedText(locale, "强业务方法", "Business Methods"),
			Description: localizedText(locale, "复杂业务逻辑方法完整清单", "Full list of complex business logic methods"),
			Path:        prefix + "business-methods.md",
		})
	}
	if len(profile.KeyModules) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       localizedText(locale, "关键模块", "Key Modules"),
			Description: localizedText(locale, "关键模块职责、依赖和入口方法", "Key module responsibilities, dependencies, and entry methods"),
			Path:        prefix + "modules.md",
		})
	}
	if len(profile.CommonUtils) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       localizedText(locale, "通用工具", "Common Utilities"),
			Description: localizedText(locale, "通用工具方法及使用场景", "Common utility functions and usage scenarios"),
			Path:        prefix + "common-utils.md",
		})
	}
	return items
}

func patternForTemplate(pattern domain.Pattern) domain.Pattern {
	if !isUsableBusinessMethod(pattern.BusinessMethod) {
		pattern.BusinessMethod = nil
	}
	return pattern
}

func validBusinessMethods(methods []*domain.BusinessMethod) []*domain.BusinessMethod {
	valid := make([]*domain.BusinessMethod, 0, len(methods))
	for _, method := range methods {
		if isUsableBusinessMethod(method) {
			valid = append(valid, method)
		}
	}
	return valid
}

func isUsableBusinessMethod(method *domain.BusinessMethod) bool {
	return method != nil && strings.TrimSpace(method.Name) != ""
}

func cleanProjectProfile(profile *domain.ProjectProfile) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}

	cleaned := *profile

	cleaned.KeyModules = make([]domain.ModuleInfo, 0, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		module.KeyMethods = filterGeneratedPlaceholders(module.KeyMethods)
		cleaned.KeyModules = append(cleaned.KeyModules, module)
	}

	cleaned.BusinessMethods = make([]domain.BusinessMethod, 0, len(profile.BusinessMethods))
	for _, method := range profile.BusinessMethods {
		if strings.TrimSpace(method.Name) != "" {
			cleaned.BusinessMethods = append(cleaned.BusinessMethods, method)
		}
	}

	cleaned.CommonUtils = make([]domain.UtilityFunction, 0, len(profile.CommonUtils))
	for _, utility := range profile.CommonUtils {
		if strings.TrimSpace(utility.Name) != "" {
			cleaned.CommonUtils = append(cleaned.CommonUtils, utility)
		}
	}

	return &cleaned
}

func filterGeneratedPlaceholders(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(strings.ToUpper(trimmed), "TODO") {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func localizedText(locale, zhCN, enUS string) string {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return zhCN
	}
	return enUS
}

func generatedSkillName(projectName string) string {
	var b strings.Builder
	previousHyphen := false
	for _, r := range strings.ToLower(projectName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousHyphen = false
			continue
		}
		if !previousHyphen {
			b.WriteRune('-')
			previousHyphen = true
		}
	}

	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "project"
	}
	if !strings.HasSuffix(name, "-dev") {
		name += "-dev"
	}
	return name
}

func (s *GeneratorService) projectSpecFromProfileAndPatterns(profile *domain.ProjectProfile, patterns []domain.Pattern, project config.WorkspaceProjectConfig) *domain.ProjectSpec {
	if profile == nil {
		return nil
	}

	spec := &domain.ProjectSpec{
		ProjectName:       profile.ProjectName,
		Language:          profile.Language,
		Summary:           profile.Summary,
		ConfigPatterns:    append([]string(nil), profile.ConfigPatterns...),
		FrameworkPatterns: append([]string(nil), profile.FrameworkPatterns...),
		GeneratedAt:       time.Now().Format("2006-01-02 15:04:05"),
	}
	if project.ID != "" {
		spec.ProjectID = project.ID
		spec.ProjectName = project.ID
		spec.ScopePath = project.Path
		spec.WorkspaceRole = project.Type
		if project.Language != "" {
			spec.Language = project.Language
		}
	}
	if spec.ProjectName == "" {
		spec.ProjectName = "project"
	}
	if spec.Language == "" {
		spec.Language = "unknown"
	}

	for _, layer := range profile.Layers {
		spec.Boundaries = append(spec.Boundaries, domain.ProjectSpecBoundary{
			Type:             "layer",
			Name:             layer.Name,
			Description:      layer.Description,
			Responsibilities: append([]string(nil), layer.Responsibilities...),
			Paths:            append([]string(nil), layer.Files...),
		})
	}
	for _, module := range profile.KeyModules {
		spec.Boundaries = append(spec.Boundaries, domain.ProjectSpecBoundary{
			Type:             "module",
			Name:             module.Name,
			Description:      module.Description,
			Responsibilities: append([]string(nil), module.Responsibilities...),
			Paths:            []string{module.Path},
		})
	}

	for _, pattern := range strongestPatterns(patterns, 12) {
		spec.PatternRules = append(spec.PatternRules, domain.ProjectSpecPatternRule{
			Name:        pattern.Name,
			Category:    string(pattern.Category),
			Description: pattern.Description,
			Rule:        pattern.Rule,
			Confidence:  pattern.Confidence,
			Frequency:   pattern.Frequency,
		})
	}

	for _, method := range profile.BusinessMethods {
		spec.Touchpoints = append(spec.Touchpoints, domain.ProjectSpecTouchpoint{
			Kind:        "business_method",
			Name:        method.Name,
			Path:        method.Location,
			Description: method.Description,
		})
	}
	for _, utility := range profile.CommonUtils {
		spec.Touchpoints = append(spec.Touchpoints, domain.ProjectSpecTouchpoint{
			Kind:        "common_utility",
			Name:        utility.Name,
			Path:        utility.File,
			Description: utility.Description,
		})
	}

	return spec
}

func strongestPatterns(patterns []domain.Pattern, limit int) []domain.Pattern {
	filtered := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern.Name) == "" {
			continue
		}
		filtered = append(filtered, pattern)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Confidence == filtered[j].Confidence {
			if filtered[i].Frequency == filtered[j].Frequency {
				return filtered[i].Name < filtered[j].Name
			}
			return filtered[i].Frequency > filtered[j].Frequency
		}
		return filtered[i].Confidence > filtered[j].Confidence
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func (s *GeneratorService) workspaceRootOutputPath(projectRoot, workspaceName string) (string, error) {
	return s.providerSkillOutputPath(projectRoot, workspaceSkillName(workspaceName))
}

func workspaceSkillName(workspaceName string) string {
	name := generatedSkillName(workspaceName)
	if strings.HasSuffix(name, "-dev") {
		return strings.TrimSuffix(name, "-dev") + "-workspace"
	}
	return name + "-workspace"
}

func (s *GeneratorService) childSkillTarget(projectRoot string, project config.WorkspaceProjectConfig) (childSkillTarget, error) {
	projectRootPath := filepath.Join(projectRoot, filepath.FromSlash(project.Path))
	childConfig, exists, configPath, err := s.workspaceChildConfig(projectRoot, project)
	if err != nil {
		return childSkillTarget{}, err
	}
	if exists {
		outputPath, err := configuredSkillOutputPath(projectRootPath, childConfig)
		if err != nil {
			return childSkillTarget{}, err
		}
		return childSkillTarget{
			OutputPath:      outputPath,
			ConfigRepo:      childConfig,
			UsesChildConfig: true,
			ConfigPath:      configPath,
		}, nil
	}
	outputPath, err := configuredSkillOutputPath(projectRootPath, s.configRepo)
	if err != nil {
		return childSkillTarget{}, err
	}
	return childSkillTarget{
		OutputPath: outputPath,
		ConfigRepo: s.configRepo,
		ConfigPath: configPath,
	}, nil
}

func configuredSkillOutputPath(projectRoot string, configRepo config.Reader) (string, error) {
	provider := ""
	outputPath := ""
	if configRepo != nil {
		provider = configRepo.GetAgentConfig().Provider
		outputPath = config.EffectiveSkillsPath(provider, configRepo.GetOutputConfig())
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = defaultProviderSkillPath(provider)
	}
	return utils.ResolvePath(projectRoot, outputPath)
}

func (s *GeneratorService) providerSkillOutputPath(projectRoot, skillName string) (string, error) {
	provider := ""
	outputPath := ""
	if s.configRepo != nil {
		provider = s.configRepo.GetAgentConfig().Provider
		outputPath = config.EffectiveSkillsPath(provider, s.configRepo.GetOutputConfig())
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = defaultProviderSkillPath(provider)
	}
	resolvedOutputPath, err := utils.ResolvePath(projectRoot, outputPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(resolvedOutputPath), skillName), nil
}

func defaultProviderSkillPath(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude":
		return filepath.Join(".claude", "skills", "skills-seed-skills")
	case "codex":
		return filepath.Join(".agents", "skills", "skills-seed-skills")
	default:
		return filepath.Join(".skills", "skills-seed-skills")
	}
}

func (s *GeneratorService) writeWorkspaceRootSkill(ctx context.Context, outputPath string, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec) error {
	if err := os.MkdirAll(filepath.Join(outputPath, "references"), 0755); err != nil {
		return err
	}

	data, err := s.workspaceTemplateData(ctx, projectConfig, workspaceConfig, profile, spec)
	if err != nil {
		return err
	}
	content, err := s.skillsLoader.RenderRelative("workspace/SKILL", data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "SKILL.md"), []byte(content), 0644); err != nil {
		return err
	}
	if err := s.generateAgentMetadata(outputPath, data); err != nil {
		return err
	}
	overview, err := s.skillsLoader.RenderRelative("workspace/references/workspace-overview", data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "references", "workspace-overview.md"), []byte(overview), 0644); err != nil {
		return err
	}
	rules, err := s.skillsLoader.RenderRelative("workspace/references/cross-project-rules", data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputPath, "references", "cross-project-rules.md"), []byte(rules), 0644)
}

func (s *GeneratorService) workspaceTemplateData(ctx context.Context, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec) (workspaceSkillTemplateData, error) {
	name := workspaceNameOrDefault(projectConfig.Name)
	projects := make([]workspaceProjectTemplateData, 0, len(workspaceConfig.Projects))
	for _, project := range workspaceConfig.Projects {
		target, err := s.childSkillTarget(projectConfig.RootPath, project)
		if err != nil {
			return workspaceSkillTemplateData{}, err
		}
		profileProject := workspaceProjectByID(profile, project.ID)
		childSkillDir := relativeWorkspacePath(projectConfig.RootPath, target.OutputPath)
		skillPath := filepath.ToSlash(filepath.Join(childSkillDir, "SKILL.md"))
		selfManagedConfigPath := filepath.ToSlash(filepath.Join(project.Path, ".skills-seed", "config.yaml"))
		projects = append(projects, workspaceProjectTemplateData{
			WorkspaceProjectConfig: project,
			SkillName:              generatedSkillName(project.ID),
			SkillPath:              skillPath,
			ProjectSpecPath:        filepath.ToSlash(filepath.Join(childSkillDir, "references", "project-spec.md")),
			SkillSummary:           childSkillSummary(filepath.Join(target.OutputPath, "SKILL.md")),
			Responsibility:         profileProject.Responsibility,
			Frameworks:             profileProject.Frameworks,
			SelfManaged:            target.UsesChildConfig,
			SelfManagedConfigPath:  selfManagedConfigPath,
			UsesChildConfig:        target.UsesChildConfig,
			HasFrameworks:          len(profileProject.Frameworks) > 0,
		})
	}
	userContext := runtimecontext.UserContext(ctx)
	shared := workspacePathsFromConfig(workspaceConfig.Shared)
	contracts := workspacePathsFromConfig(workspaceConfig.Contracts)
	infra := workspacePathsFromConfig(workspaceConfig.Infra)
	var dependencies []domain.WorkspaceDependency
	var impactRoutes []domain.WorkspaceRoute
	if profile != nil {
		if profile.Summary != "" {
			name = firstNonEmpty(profile.Name, name)
		}
		shared = chooseWorkspacePaths(shared, profile.Shared)
		contracts = chooseWorkspacePaths(contracts, profile.Contracts)
		infra = chooseWorkspacePaths(infra, profile.Infra)
		dependencies = profile.Dependencies
		impactRoutes = profile.ImpactRoutes
	}
	var routing []domain.WorkspaceRoute
	var rules []domain.WorkspaceRule
	var changeOrder []string
	var parallelGuidance []domain.WorkspaceParallelGuidance
	var loadMultipleWhen []domain.WorkspaceLoadMultipleSkill
	if spec != nil {
		routing = spec.Routing
		rules = spec.Rules
		changeOrder = spec.ChangeOrder
		parallelGuidance = spec.ParallelAgentGuidance
		loadMultipleWhen = spec.LoadMultipleSkillsWhen
	}
	summary := ""
	if profile != nil {
		summary = profile.Summary
	}
	return workspaceSkillTemplateData{
		ProgramVersion:      metadata.ProgramVersion,
		SkillsTemplatesHash: metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS)),
		SkillName:           workspaceSkillName(name),
		ProjectName:         name,
		WorkspaceName:       name,
		WorkspaceFacts:      summary,
		ProjectCount:        len(projects),
		Projects:            projects,
		UserContext:         userContext,
		HasUserContext:      userContext != "",
		Shared:              shared,
		Contracts:           contracts,
		Infra:               infra,
		Dependencies:        dependencies,
		ImpactRoutes:        impactRoutes,
		Routing:             routing,
		Rules:               rules,
		ChangeOrder:         changeOrder,
		ParallelGuidance:    parallelGuidance,
		LoadMultipleWhen:    loadMultipleWhen,
		HasWorkspaceFacts:   summary != "",
		HasShared:           len(shared) > 0,
		HasContracts:        len(contracts) > 0,
		HasInfra:            len(infra) > 0,
		HasDependencies:     len(dependencies) > 0,
		HasImpactRoutes:     len(impactRoutes) > 0,
		HasRouting:          len(routing) > 0,
		HasRules:            len(rules) > 0,
		HasChangeOrder:      len(changeOrder) > 0,
		HasParallelGuidance: len(parallelGuidance) > 0,
		HasLoadMultipleWhen: len(loadMultipleWhen) > 0,
	}, nil
}

func workspaceProjectByID(profile *domain.WorkspaceProfile, id string) domain.WorkspaceProject {
	if profile == nil {
		return domain.WorkspaceProject{}
	}
	for _, project := range profile.Projects {
		if project.ID == id {
			return project
		}
	}
	return domain.WorkspaceProject{}
}

func workspacePathsFromConfig(paths []config.WorkspacePathConfig) []domain.WorkspacePath {
	result := make([]domain.WorkspacePath, 0, len(paths))
	for _, path := range paths {
		result = append(result, domain.WorkspacePath{
			Path:        path.Path,
			Description: path.Description,
		})
	}
	return result
}

func relativeWorkspacePath(workspaceRoot, path string) string {
	if workspaceRoot == "" {
		return path
	}
	rel, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return path
	}
	return rel
}

func childSkillSummary(skillPath string) string {
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// generateReferenceFiles 生成所有参考文档
func (s *GeneratorService) generateReferenceFiles(ctx context.Context, outputPath string, summaries map[string]agent.CategorySummary, patterns []domain.Pattern, profile *domain.ProjectProfile, spec *domain.ProjectSpec) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_reference_files",
		"output_path", outputPath,
		"summaries_count", len(summaries),
		"patterns_count", len(patterns),
	)

	// 确保 references 目录存在
	refsPath := filepath.Join(outputPath, "references")
	if err := os.MkdirAll(refsPath, 0755); err != nil {
		return err
	}

	// 1. 生成 project-overview.md（如果不存在）
	if err := s.generateProjectOverview(refsPath, profile); err != nil {
		return err
	}
	if err := s.generateProfileReferenceFiles(refsPath, profile); err != nil {
		return err
	}
	if spec != nil {
		if err := s.generateProjectSpec(refsPath, spec); err != nil {
			return err
		}
	}

	// 2. 创建 patterns 目录，并清理旧版 examples 目录
	patternsPath := filepath.Join(refsPath, "patterns")
	examplesPath := filepath.Join(refsPath, "examples")

	if err := os.MkdirAll(patternsPath, 0755); err != nil {
		return err
	}
	if err := os.RemoveAll(examplesPath); err != nil {
		return err
	}

	// 3. 为每个实际存在的分类生成 patterns/{category}.md
	for _, categoryName := range categoryNamesWithPatterns(patterns) {
		summary := summaries[categoryName]
		if err := s.generateCategoryPattern(patternsPath, categoryName, summary, patterns); err != nil {
			return err
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_reference_files",
		"duration", time.Since(startedAt),
		"refs_path", refsPath,
		"summaries_count", len(summaries),
	)
	return nil
}

func (s *GeneratorService) generateProjectSpec(refsPath string, spec *domain.ProjectSpec) error {
	specPath := filepath.Join(refsPath, "project-spec.md")
	content, err := s.skillsLoader.RenderReferenceFile("project-spec", projectSpecTemplateData{ProjectSpec: *spec})
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectSpecRenderFailed"), err)
	}
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_project_spec",
		"path", specPath,
		"content_length", len(content),
	)
	return nil
}

// generateProjectOverview 生成项目概览
func (s *GeneratorService) generateProjectOverview(refsPath string, profile *domain.ProjectProfile) error {
	overviewPath := filepath.Join(refsPath, "project-overview.md")

	data := projectOverviewTemplateData{
		ProjectProfile:     *profile,
		OverviewReferences: profileReferenceItems(profile, s.skillsLoader.GetLocale(), "./"),
	}
	content, err := s.skillsLoader.RenderProjectOverview(data)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGenerateOverviewFailed"), err)
	}
	if err := os.WriteFile(overviewPath, []byte(content), 0644); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_project_overview",
		"path", overviewPath,
		"content_length", len(content),
		"placeholder", false,
	)
	return nil
}

func (s *GeneratorService) generateProfileReferenceFiles(refsPath string, profile *domain.ProjectProfile) error {
	files := []struct {
		templateName string
		outputName   string
		enabled      bool
	}{
		{templateName: "business-methods", outputName: "business-methods.md", enabled: len(profile.BusinessMethods) > 0},
		{templateName: "modules", outputName: "modules.md", enabled: len(profile.KeyModules) > 0},
		{templateName: "common-utils", outputName: "common-utils.md", enabled: len(profile.CommonUtils) > 0},
	}

	for _, file := range files {
		if !file.enabled {
			continue
		}
		content, err := s.skillsLoader.RenderReferenceFile(file.templateName, profile)
		if err != nil {
			return fmt.Errorf("%s: reference=%s: %w", i18n.Get("InitGenerateOverviewFailed"), file.templateName, err)
		}

		outputPath := filepath.Join(refsPath, file.outputName)
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.generate_profile_reference",
			"reference", file.templateName,
			"path", outputPath,
			"content_length", len(content),
		)
	}
	return nil
}

// generateCategoryPattern 生成分类模式文档到 patterns/ 目录
func (s *GeneratorService) generateCategoryPattern(patternsPath, categoryName string, summary agent.CategorySummary, allPatterns []domain.Pattern) error {
	// 筛选出当前分类的完整模式对象
	var categoryPatterns []domain.Pattern
	for _, p := range allPatterns {
		if string(p.Category) == categoryName {
			categoryPatterns = append(categoryPatterns, patternForTemplate(p))
		}
	}

	// 调试日志
	if len(categoryPatterns) == 0 {
		logger.Warn(i18n.Get("LoggerGeneratorSkipEmptyCategoryPattern"), "category", categoryName)
		return nil
	}

	logger.Diagnostic(i18n.Get("LoggerGeneratorGeneratingPatternFile"),
		"category", categoryName,
		"patternObjects", len(categoryPatterns),
		"summaryPatterns", len(summary.Patterns),
		"firstPatternGoodExample", len(categoryPatterns[0].GoodExample),
	)

	// 准备模板数据
	data := map[string]interface{}{
		"Category":        summary.Category,
		"Summary":         summary.Summary,
		"Patterns":        summary.Patterns, // 模式名称列表
		"PatternObjects":  categoryPatterns, // 完整的模式对象（包含good_example等）
		"UsageScenes":     summary.UsageScenes,
		"Priority":        summary.Priority,
		"PatternCount":    len(categoryPatterns),
		"Confidence":      s.calculateCategoryConfidence(allPatterns, categoryName) * 100,
		"LastUpdated":     time.Now().Format("2006-01-02 15:04:05"),
		"BusinessMethods": validBusinessMethods(summary.BusinessMethods),
	}

	// 渲染模板
	content, err := s.skillsLoader.RenderPattern(templateCategoryName(categoryName), data)
	if err != nil {
		return fmt.Errorf("%s: category=%s: %w", i18n.Get("GeneratorRenderPatternTemplateFailed"), categoryName, err)
	}

	// 写入文件到 patterns/{category}.md
	outputPath := filepath.Join(patternsPath, categoryName+".md")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_category_pattern",
		"category", categoryName,
		"path", outputPath,
		"content_length", len(content),
		"patterns_count", len(categoryPatterns),
	)
	return nil
}

// Stats 统计信息
type Stats struct {
	Total          int
	AvgConfidence  float64
	HighConfidence []domain.Pattern
	Frequent       []domain.Pattern
	ByCategory     map[string][]domain.Pattern
}

// calculateStats 计算统计信息
func (s *GeneratorService) calculateStats(patterns []domain.Pattern) *Stats {
	stats := &Stats{
		Total:      len(patterns),
		ByCategory: make(map[string][]domain.Pattern),
	}

	if len(patterns) == 0 {
		return stats
	}

	// 计算平均置信度
	var totalConfidence float64
	for _, p := range patterns {
		totalConfidence += p.Confidence
	}
	stats.AvgConfidence = totalConfidence / float64(len(patterns))

	// 按分类统计
	for _, p := range patterns {
		category := string(p.Category)
		stats.ByCategory[category] = append(stats.ByCategory[category], p)
	}

	// 筛选高置信度模式（>0.8）
	for _, p := range patterns {
		if p.Confidence > 0.8 {
			stats.HighConfidence = append(stats.HighConfidence, p)
		}
	}

	// 筛选频繁模式（>3次）
	for _, p := range patterns {
		if p.Frequency > 3 {
			stats.Frequent = append(stats.Frequent, p)
		}
	}

	return stats
}

// calculateCategoryConfidence 计算特定分类的平均置信度
func (s *GeneratorService) calculateCategoryConfidence(patterns []domain.Pattern, category string) float64 {
	var total float64
	var count int
	for _, p := range patterns {
		if string(p.Category) == category {
			total += p.Confidence
			count++
		}
	}
	if count == 0 {
		return 0.0
	}
	return total / float64(count)
}

func templateCategoryName(category string) string {
	switch category {
	case string(domain.CategoryError):
		return "error-handling"
	default:
		return category
	}
}
