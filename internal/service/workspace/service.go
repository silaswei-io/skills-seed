package workspace

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
	workspacestore "github.com/silaswei-io/skills-seed/internal/infra/storage/workspace"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

type WorkspaceGenerator struct {
	configRepo           config.Reader
	skillsLoader         *skills.Loader
	writer               *generator.SkillWriter
	agent                agent.Agent
	workspaceProfileRepo domain.WorkspaceProfileRepository
	workspaceSpecRepo    domain.WorkspaceSpecRepository
	scopedProfileRepo    domain.ScopedProjectProfileRepository
	projectSpecRepo      domain.ProjectSpecRepository
	scopedSpecRepo       domain.ScopedProjectSpecRepository
	patternStatsRepo     domain.PatternStatsRepository
	patternRepo          domain.PatternRepository
}

func NewWorkspaceGenerator(
	patternRepo domain.PatternRepository,
	profileRepo domain.ProjectProfileRepository,
	skillsLoader *skills.Loader,
	ag agent.Agent,
	configRepo config.Reader,
	workspaceProfileRepo domain.WorkspaceProfileRepository,
	workspaceSpecRepo domain.WorkspaceSpecRepository,
) *WorkspaceGenerator {
	scopedProfileRepo, _ := profileRepo.(domain.ScopedProjectProfileRepository)
	projectSpecRepo, _ := profileRepo.(domain.ProjectSpecRepository)
	scopedSpecRepo, _ := profileRepo.(domain.ScopedProjectSpecRepository)
	patternStatsRepo, _ := patternRepo.(domain.PatternStatsRepository)
	return &WorkspaceGenerator{
		patternRepo:          patternRepo,
		patternStatsRepo:     patternStatsRepo,
		scopedProfileRepo:    scopedProfileRepo,
		projectSpecRepo:      projectSpecRepo,
		scopedSpecRepo:       scopedSpecRepo,
		skillsLoader:         skillsLoader,
		writer:               generator.NewSkillWriter(skillsLoader),
		agent:                ag,
		configRepo:           configRepo,
		workspaceProfileRepo: workspaceProfileRepo,
		workspaceSpecRepo:    workspaceSpecRepo,
	}
}

func (g *WorkspaceGenerator) fileAnalysisTracker() domain.FileAnalysisTracker {
	tracker, _ := g.patternRepo.(domain.FileAnalysisTracker)
	return tracker
}

// GenerateProgressHooks 复用 generator.GenerateProgressHooks，供 workspace 生成流程消费。
type GenerateProgressHooks = generator.GenerateProgressHooks

// GenerateProjectStepTotal 复用单项目生成流程的步骤总数。
const GenerateProjectStepTotal = generator.GenerateProjectStepTotal

// GenerateWorkspaceSkills 只生成工作区根 skill，子项目 skill 由各子仓自己生成
func (g *WorkspaceGenerator) GenerateWorkspaceSkills(ctx context.Context) error {
	return g.generateWorkspaceSkills(ctx)
}

func (g *WorkspaceGenerator) generateWorkspaceSkills(ctx context.Context) error {
	startedAt := time.Now()
	projectConfig := g.configRepo.GetProjectConfig()
	workspaceConfig := g.configRepo.GetWorkspaceConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}

	projectRoot := projectConfig.RootPath
	rootOutputPath, err := g.workspaceRootOutputPath(projectRoot, projectConfig.Name)
	if err != nil {
		return err
	}
	profile, spec, err := g.analyzeWorkspaceForGenerate(ctx, projectConfig, workspaceConfig)
	if err != nil {
		return err
	}
	if err := g.writeWorkspaceRootSkill(ctx, rootOutputPath, projectConfig, workspaceConfig, profile, spec); err != nil {
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_workspace_skills",
		"duration", time.Since(startedAt),
		"projects_count", len(workspaceConfig.Projects),
	)
	return nil
}

func (g *WorkspaceGenerator) analyzeWorkspaceForGenerate(ctx context.Context, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig) (*domain.WorkspaceProfile, *domain.WorkspaceSpec, error) {
	workspaceName := workspaceNameOrDefault(projectConfig.Name)
	projectRoot := projectConfig.RootPath
	if profile, spec, ok, err := g.loadPersistedWorkspaceArtifacts(ctx, workspaceName, projectRoot, workspaceConfig); err != nil {
		return nil, nil, err
	} else if ok {
		return profile, spec, nil
	}

	if g.agent == nil {
		return nil, nil, fmt.Errorf("%s", i18n.Get("GeneratorGenerateSummaryFailed"))
	}

	workspaceInput, err := g.workspaceGenerateInput(projectConfig, workspaceConfig)
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

	profile, err := g.agent.AnalyzeWorkspaceProfile(ctx, &agent.AnalyzeWorkspaceProfileRequest{
		WorkspaceName:      workspaceName,
		WorkspaceRoot:      projectRoot,
		WorkspaceInputPath: inputPath,
	})
	if err != nil {
		return nil, nil, err
	}
	profile = workspacediscovery.MergeProfile(workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig), profile)
	profilePath, err := writeJSONFile(filepath.Join(tmpDir, "workspace-profile.json"), profile)
	if err != nil {
		return nil, nil, err
	}

	spec, err := g.agent.AnalyzeWorkspaceSpec(ctx, &agent.AnalyzeWorkspaceSpecRequest{
		WorkspaceName:        workspaceName,
		WorkspaceRoot:        projectRoot,
		WorkspaceInputPath:   inputPath,
		WorkspaceProfilePath: profilePath,
	})
	if err != nil {
		return nil, nil, err
	}
	spec = workspacediscovery.MergeSpec(workspacediscovery.SpecFromProfile(profile), spec)
	return profile, spec, nil
}

func (g *WorkspaceGenerator) loadPersistedWorkspaceArtifacts(ctx context.Context, workspaceName, projectRoot string, workspaceConfig config.WorkspaceConfig) (*domain.WorkspaceProfile, *domain.WorkspaceSpec, bool, error) {
	if g.workspaceProfileRepo == nil && g.workspaceSpecRepo == nil {
		return nil, nil, false, nil
	}

	var profile *domain.WorkspaceProfile
	var spec *domain.WorkspaceSpec
	loaded := false
	if g.workspaceProfileRepo != nil {
		storedProfile, err := g.workspaceProfileRepo.Get(ctx)
		if err != nil {
			if !isWorkspaceArtifactNotFound(err) {
				return nil, nil, false, err
			}
		} else {
			profile = workspacediscovery.MergeProfile(workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig), storedProfile)
			loaded = true
		}
	}
	if profile == nil {
		profile = workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig)
	}
	if g.workspaceSpecRepo != nil {
		storedSpec, err := g.workspaceSpecRepo.Get(ctx)
		if err != nil {
			if !isWorkspaceArtifactNotFound(err) {
				return nil, nil, false, err
			}
		} else {
			spec = workspacediscovery.MergeSpec(workspacediscovery.SpecFromProfile(profile), storedSpec)
			loaded = true
		}
	}
	if spec == nil {
		spec = workspacediscovery.SpecFromProfile(profile)
	}
	return profile, spec, loaded, nil
}

func isWorkspaceArtifactNotFound(err error) bool {
	return errors.Is(err, workspacestore.ErrProfileNotFound) || errors.Is(err, workspacestore.ErrSpecNotFound)
}

func (g *WorkspaceGenerator) workspaceGenerateInput(projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig) (workspaceGenerateInputData, error) {
	projectRoot := projectConfig.RootPath
	input := workspaceGenerateInputData{
		Name:       workspaceNameOrDefault(projectConfig.Name),
		RootPath:   projectRoot,
		Projects:   make([]workspaceGenerateInputProject, 0, len(workspaceConfig.Projects)),
		ConfigPath: filepath.Join(projectRoot, ".skills-seed", "config.yaml"),
	}
	for _, project := range workspaceConfig.Projects {
		target, err := g.childSkillTarget(projectRoot, project)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (g *WorkspaceGenerator) writeWorkspaceRootSkill(ctx context.Context, outputPath string, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec) error {
	data, err := g.workspaceTemplateData(ctx, projectConfig, workspaceConfig, profile, spec)
	if err != nil {
		return err
	}
	decision, err := g.prepareWorkspaceSkillsFingerprint(ctx, outputPath, data)
	if err != nil {
		return err
	}
	if decision.ShouldSkip() && workspaceRootSkillsOutputExists(outputPath) {
		logger.Info(i18n.Get("GenerateWorkspaceRootSkipped"))
		return nil
	}
	if err := os.MkdirAll(filepath.Join(outputPath, "references"), 0755); err != nil {
		return err
	}
	content, err := g.skillsLoader.Render("workspace-skill", data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "SKILL.md"), []byte(content), 0644); err != nil {
		return err
	}
	if err := g.writer.GenerateAgentMetadata(outputPath, data); err != nil {
		return err
	}
	overview, err := g.skillsLoader.Render("workspace-reference-overview", data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "references", "workspace-overview.md"), []byte(overview), 0644); err != nil {
		return err
	}
	rules, err := g.skillsLoader.Render("workspace-reference-cross-project-rules", data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "references", "cross-project-rules.md"), []byte(rules), 0644); err != nil {
		return err
	}
	return decision.Commit(ctx, g.fileAnalysisTracker())
}

func (g *WorkspaceGenerator) workspaceTemplateData(ctx context.Context, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec) (workspaceSkillTemplateData, error) {
	name := workspaceNameOrDefault(projectConfig.Name)
	projects := make([]workspaceProjectTemplateData, 0, len(workspaceConfig.Projects))
	for _, project := range workspaceConfig.Projects {
		target, err := g.childSkillTarget(projectConfig.RootPath, project)
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
	var shared []domain.WorkspacePath
	var contracts []domain.WorkspacePath
	var infra []domain.WorkspacePath
	var dependencies []domain.WorkspaceDependency
	var impactRoutes []domain.WorkspaceRoute
	if profile != nil {
		if profile.Summary != "" {
			name = firstNonEmpty(profile.Name, name)
		}
		shared = workspacediscovery.ChoosePaths(shared, profile.Shared)
		contracts = workspacediscovery.ChoosePaths(contracts, profile.Contracts)
		infra = workspacediscovery.ChoosePaths(infra, profile.Infra)
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
	patternRules, err := g.workspaceRulesFromPatterns(ctx, workspaceConfig)
	if err != nil {
		return workspaceSkillTemplateData{}, err
	}
	rules = append(rules, patternRules...)
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

func (g *WorkspaceGenerator) workspaceRulesFromPatterns(ctx context.Context, workspaceConfig config.WorkspaceConfig) ([]domain.WorkspaceRule, error) {
	if g.patternRepo == nil {
		return nil, nil
	}
	patterns, err := g.patternRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(patterns, func(i, j int) bool {
		return patterns[i].ID < patterns[j].ID
	})
	rules := make([]domain.WorkspaceRule, 0, len(patterns))
	for _, pattern := range patterns {
		rule := workspaceRuleFromPattern(pattern, workspaceConfig)
		if rule.Title == "" || rule.Description == "" {
			continue
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func workspaceRuleFromPattern(pattern domain.Pattern, workspaceConfig config.WorkspaceConfig) domain.WorkspaceRule {
	title := strings.TrimSpace(pattern.Name)
	if title == "" {
		title = strings.TrimSpace(pattern.ID)
	}
	description := strings.TrimSpace(pattern.Rule)
	if description == "" {
		description = strings.TrimSpace(pattern.Description)
	}
	if title == "" || description == "" {
		return domain.WorkspaceRule{}
	}
	return domain.WorkspaceRule{
		Title:       title,
		Description: description,
		AppliesTo:   workspacePatternAppliesTo(pattern, workspaceConfig),
	}
}

func workspacePatternAppliesTo(pattern domain.Pattern, workspaceConfig config.WorkspaceConfig) []string {
	if strings.TrimSpace(pattern.ProjectID) != "" {
		return []string{strings.TrimSpace(pattern.ProjectID)}
	}
	scopePath := strings.Trim(filepath.ToSlash(strings.TrimSpace(pattern.ScopePath)), "/")
	if scopePath == "" {
		return nil
	}
	for _, project := range workspaceConfig.Projects {
		projectPath := strings.Trim(filepath.ToSlash(strings.TrimSpace(project.Path)), "/")
		if projectPath == "" {
			continue
		}
		if scopePath == projectPath || strings.HasPrefix(scopePath, projectPath+"/") {
			if strings.TrimSpace(project.ID) != "" {
				return []string{strings.TrimSpace(project.ID)}
			}
			return []string{project.Path}
		}
	}
	return nil
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
