package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	workspacestore "github.com/silaswei-io/skills-seed/internal/infra/storage/workspace"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/service/skilloutput"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

type WorkspaceGenerator struct {
	configRepo           config.Reader
	skillsLoader         *skills.Loader
	renderer             *skillgen.Renderer
	workspaceProfileRepo domain.WorkspaceProfileRepository
	workspaceSpecRepo    domain.WorkspaceSpecRepository
	scopedProfileRepo    domain.ScopedProjectProfileRepository
	projectSpecRepo      domain.ProjectSpecRepository
	scopedSpecRepo       domain.ScopedProjectSpecRepository
	patternStatsRepo     domain.PatternStatsRepository
	patternRepo          domain.PatternRepository
	workflowRepo         domain.WorkflowRepository
}

func NewWorkspaceGenerator(
	patternRepo domain.PatternRepository,
	profileRepo domain.ProjectProfileRepository,
	skillsLoader *skills.Loader,
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
		renderer:             skillgen.NewRenderer(skillsLoader),
		configRepo:           configRepo,
		workspaceProfileRepo: workspaceProfileRepo,
		workspaceSpecRepo:    workspaceSpecRepo,
	}
}

// SetWorkflowRepository 注入当前目标的工作流仓储。
func (g *WorkspaceGenerator) SetWorkflowRepository(repo domain.WorkflowRepository) {
	g.workflowRepo = repo
}

// GenerateProgressHooks 复用 generator.GenerateProgressHooks，供 workspace 生成流程消费。
type GenerateProgressHooks = generator.GenerateProgressHooks

// GenerateProjectStepTotal 复用单项目生成流程的步骤总数。
const GenerateProjectStepTotal = generator.GenerateProjectStepTotal

// GenerateWorkspaceSkills 只生成工作区根 skill，子项目 skill 由各子仓自己生成
func (g *WorkspaceGenerator) GenerateWorkspaceSkills(ctx context.Context) error {
	return g.GenerateWorkspaceSkillsWithOptions(ctx, WorkspaceGenerateOptions{})
}

// GenerateWorkspaceSkillsWithOptions 按指定选项生成工作区根 skill。
func (g *WorkspaceGenerator) GenerateWorkspaceSkillsWithOptions(ctx context.Context, opts WorkspaceGenerateOptions) error {
	return g.generateWorkspaceSkills(ctx, opts)
}

// ResolveWorkspaceRootOutputPath 解析工作区根 skill 最终输出目录。
func (g *WorkspaceGenerator) ResolveWorkspaceRootOutputPath(opts WorkspaceGenerateOptions) (string, error) {
	projectConfig := g.configRepo.GetProjectConfig()
	return g.resolveWorkspaceRootOutputPath(projectConfig.RootPath, projectConfig.Name, opts)
}

func (g *WorkspaceGenerator) generateWorkspaceSkills(ctx context.Context, opts WorkspaceGenerateOptions) error {
	startedAt := time.Now()
	projectConfig := g.configRepo.GetProjectConfig()
	workspaceConfig := g.configRepo.GetWorkspaceConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}

	projectRoot := projectConfig.RootPath
	rootOutputPath, err := g.resolveWorkspaceRootOutputPath(projectRoot, projectConfig.Name, opts)
	if err != nil {
		return err
	}
	profile, spec, err := g.analyzeWorkspaceForGenerate(ctx, projectConfig, workspaceConfig)
	if err != nil {
		return err
	}
	if err := g.writeWorkspaceRootSkill(ctx, rootOutputPath, projectConfig, workspaceConfig, profile, spec, opts); err != nil {
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_workspace_skills",
		"duration", time.Since(startedAt),
		"projects_count", len(workspaceConfig.Projects),
	)
	return nil
}

func (g *WorkspaceGenerator) resolveWorkspaceRootOutputPath(projectRoot, workspaceName string, opts WorkspaceGenerateOptions) (string, error) {
	if strings.TrimSpace(opts.RootOutputPath) != "" {
		return resolveProjectOutputPath(projectRoot, opts.RootOutputPath)
	}
	return g.workspaceRootOutputPath(projectRoot, workspaceName)
}

func (g *WorkspaceGenerator) analyzeWorkspaceForGenerate(ctx context.Context, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig) (*domain.WorkspaceProfile, *domain.WorkspaceSpec, error) {
	workspaceName := workspaceNameOrDefault(projectConfig.Name)
	projectRoot := projectConfig.RootPath
	if profile, spec, ok, err := g.loadPersistedWorkspaceArtifacts(ctx, workspaceName, projectRoot, workspaceConfig); err != nil {
		return nil, nil, err
	} else if ok {
		return profile, spec, nil
	}

	profile := workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig)
	spec := workspacediscovery.SpecFromProfile(profile)
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

func (g *WorkspaceGenerator) writeWorkspaceRootSkill(ctx context.Context, outputPath string, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec, opts WorkspaceGenerateOptions) error {
	data, err := g.workspaceTemplateData(ctx, projectConfig, workspaceConfig, profile, spec, opts)
	if err != nil {
		return err
	}
	if err := ensureGeneratedOutputDirWritable(outputPath); err != nil {
		return err
	}
	if err := skilloutput.Rebuild(outputPath); err != nil {
		var manualErr *skilloutput.ManualSkillExistsError
		if errors.As(err, &manualErr) {
			return &generator.ManualSkillExistsError{Path: manualErr.Path}
		}
		return err
	}
	if err := generator.WriteWorkflowOutputs(g.workflowRepo, outputPath, g.skillsLoader.GetLocale()); err != nil {
		return err
	}
	p := skillgen.NewPlan(outputPath)
	p.AddFile("SKILL.md", skillgen.CatalogTemplate, "workspace-skill", data)
	p.AgentMetadataData = data
	if !opts.SkipReferences {
		p.AddDir("references")
		p.AddFile("references/workspace-overview.md", skillgen.CatalogTemplate, "workspace-reference-overview", data)
		p.AddFile("references/cross-project-rules.md", skillgen.CatalogTemplate, "workspace-reference-cross-project-rules", data)
	}
	return g.renderer.Render(ctx, p)
}

func ensureGeneratedOutputDirWritable(outputPath string) error {
	if err := skilloutput.EnsureWritable(outputPath); err != nil {
		var manualErr *skilloutput.ManualSkillExistsError
		if errors.As(err, &manualErr) {
			return &generator.ManualSkillExistsError{Path: manualErr.Path}
		}
		return err
	}
	return nil
}

func (g *WorkspaceGenerator) workspaceTemplateData(ctx context.Context, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec, opts WorkspaceGenerateOptions) (workspaceSkillTemplateData, error) {
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
	knownProjects := workspaceProjectSet(workspaceConfig.Projects)
	unknownProjects := workspaceUnknownProjectSet(knownProjects, profile, spec)
	shared = filterWorkspacePaths(shared, knownProjects, unknownProjects)
	contracts = filterWorkspacePaths(contracts, knownProjects, unknownProjects)
	infra = filterWorkspacePaths(infra, knownProjects, unknownProjects)
	dependencies = filterWorkspaceDependencies(dependencies, knownProjects)
	impactRoutes = filterWorkspaceRoutes(impactRoutes, knownProjects, projectConfig.RootPath)
	routing = filterWorkspaceRoutes(routing, knownProjects, projectConfig.RootPath)
	rules = filterWorkspaceRules(rules, knownProjects, unknownProjects)
	parallelGuidance = filterWorkspaceParallelGuidance(parallelGuidance, unknownProjects)
	loadMultipleWhen = filterWorkspaceLoadMultiple(loadMultipleWhen, knownProjects, unknownProjects)
	workflowReferences, err := generator.LoadWorkflowReferences(g.workflowRepo, g.skillsLoader.GetLocale())
	if err != nil {
		return workspaceSkillTemplateData{}, err
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
		WorkflowReferences:  workflowReferences,
		SkipReferences:      opts.SkipReferences,
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
		HasWorkflowRefs:     len(workflowReferences) > 0,
	}, nil
}

func workspaceProjectSet(projects []config.WorkspaceProjectConfig) map[string]struct{} {
	known := make(map[string]struct{}, len(projects))
	for _, project := range projects {
		if id := strings.TrimSpace(project.ID); id != "" {
			known[id] = struct{}{}
		}
	}
	return known
}

func workspaceUnknownProjectSet(knownProjects map[string]struct{}, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec) map[string]struct{} {
	unknown := map[string]struct{}{}
	add := func(ids ...string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := knownProjects[id]; ok {
				continue
			}
			unknown[id] = struct{}{}
		}
	}
	if profile != nil {
		for _, path := range append(append([]domain.WorkspacePath{}, profile.Shared...), append(profile.Contracts, profile.Infra...)...) {
			add(path.Consumers...)
			add(path.Producers...)
			add(path.AffectedProjects...)
		}
		for _, dependency := range profile.Dependencies {
			add(dependency.From, dependency.To)
		}
		for _, route := range profile.ImpactRoutes {
			add(route.ProjectIDs...)
		}
	}
	if spec != nil {
		for _, route := range spec.Routing {
			add(route.ProjectIDs...)
		}
		for _, rule := range spec.Rules {
			add(rule.AppliesTo...)
		}
		for _, item := range spec.LoadMultipleSkillsWhen {
			add(item.ProjectIDs...)
		}
	}
	return unknown
}

func filterWorkspacePaths(paths []domain.WorkspacePath, knownProjects, unknownProjects map[string]struct{}) []domain.WorkspacePath {
	out := make([]domain.WorkspacePath, 0, len(paths))
	for _, item := range paths {
		item.Consumers = knownProjectIDs(item.Consumers, knownProjects)
		item.Producers = knownProjectIDs(item.Producers, knownProjects)
		item.AffectedProjects = knownProjectIDs(item.AffectedProjects, knownProjects)
		if referencesUnknownProject(item.Description, unknownProjects) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterWorkspaceDependencies(dependencies []domain.WorkspaceDependency, knownProjects map[string]struct{}) []domain.WorkspaceDependency {
	out := make([]domain.WorkspaceDependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		if _, ok := knownProjects[strings.TrimSpace(dependency.From)]; !ok {
			continue
		}
		if _, ok := knownProjects[strings.TrimSpace(dependency.To)]; !ok {
			continue
		}
		out = append(out, dependency)
	}
	return out
}

func filterWorkspaceRoutes(routes []domain.WorkspaceRoute, knownProjects map[string]struct{}, rootPath string) []domain.WorkspaceRoute {
	out := make([]domain.WorkspaceRoute, 0, len(routes))
	for _, route := range routes {
		route.ProjectIDs = knownProjectIDs(route.ProjectIDs, knownProjects)
		if len(route.ProjectIDs) == 0 {
			continue
		}
		if !workspaceRoutePathExists(rootPath, route.PathPattern) {
			continue
		}
		out = append(out, route)
	}
	return out
}

func filterWorkspaceRules(rules []domain.WorkspaceRule, knownProjects, unknownProjects map[string]struct{}) []domain.WorkspaceRule {
	out := make([]domain.WorkspaceRule, 0, len(rules))
	for _, rule := range rules {
		rule.AppliesTo = knownProjectIDs(rule.AppliesTo, knownProjects)
		if referencesUnknownProject(rule.Description, unknownProjects) {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func filterWorkspaceParallelGuidance(items []domain.WorkspaceParallelGuidance, unknownProjects map[string]struct{}) []domain.WorkspaceParallelGuidance {
	out := make([]domain.WorkspaceParallelGuidance, 0, len(items))
	for _, item := range items {
		if referencesUnknownProject(item.Scope, unknownProjects) || referencesUnknownProject(item.Condition, unknownProjects) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterWorkspaceLoadMultiple(items []domain.WorkspaceLoadMultipleSkill, knownProjects, unknownProjects map[string]struct{}) []domain.WorkspaceLoadMultipleSkill {
	out := make([]domain.WorkspaceLoadMultipleSkill, 0, len(items))
	for _, item := range items {
		item.ProjectIDs = knownProjectIDs(item.ProjectIDs, knownProjects)
		if len(item.ProjectIDs) == 0 {
			continue
		}
		if referencesUnknownProject(item.Condition, unknownProjects) || referencesUnknownProject(item.Reason, unknownProjects) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func knownProjectIDs(ids []string, knownProjects map[string]struct{}) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := knownProjects[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func referencesUnknownProject(text string, unknownProjects map[string]struct{}) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for id := range unknownProjects {
		if strings.Contains(text, id) {
			return true
		}
	}
	return false
}

func workspaceRoutePathExists(rootPath, pattern string) bool {
	prefix := workspaceRouteStaticPrefix(pattern)
	if prefix == "" {
		return true
	}
	if strings.HasPrefix(prefix, ".") {
		return false
	}
	if _, err := os.Stat(filepath.Join(rootPath, filepath.FromSlash(prefix))); err == nil {
		return true
	}
	return false
}

func workspaceRouteStaticPrefix(pattern string) string {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return ""
	}
	cut := len(pattern)
	for _, marker := range []string{"*", "?", "["} {
		if idx := strings.Index(pattern, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	prefix := strings.Trim(pattern[:cut], "/")
	if prefix == "" {
		return ""
	}
	parts := strings.Split(prefix, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
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
