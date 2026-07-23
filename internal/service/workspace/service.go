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
	"github.com/silaswei-io/skills-seed/internal/utils"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

type WorkspaceGenerator struct {
	configRepo           config.Reader
	skillsLoader         *skills.Loader
	renderer             *skillgen.Renderer
	workspaceProfileRepo domain.WorkspaceProfileRepository
	workspaceSpecRepo    domain.WorkspaceSpecRepository
	workflowRepo         domain.WorkflowRepository
}

func NewWorkspaceGenerator(
	skillsLoader *skills.Loader,
	configRepo config.Reader,
	workspaceProfileRepo domain.WorkspaceProfileRepository,
	workspaceSpecRepo domain.WorkspaceSpecRepository,
	workflowRepo domain.WorkflowRepository,
) *WorkspaceGenerator {
	return &WorkspaceGenerator{
		skillsLoader:         skillsLoader,
		renderer:             skillgen.NewRenderer(skillsLoader),
		configRepo:           configRepo,
		workspaceProfileRepo: workspaceProfileRepo,
		workspaceSpecRepo:    workspaceSpecRepo,
		workflowRepo:         workflowRepo,
	}
}

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
	if strings.TrimSpace(opts.RootOutputPath) == "" {
		legacyOutputPath, err := g.targetSkillOutputPath(projectRoot, legacyWorkspaceSkillName(projectConfig.Name))
		if err != nil {
			return err
		}
		if legacyOutputPath != rootOutputPath {
			if err := skilloutput.Remove(legacyOutputPath); err != nil {
				return err
			}
		}
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
		return utils.ResolveProjectOutputPath(projectRoot, opts.RootOutputPath)
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
	spec := workspacediscovery.SpecFromProfile(profile, g.skillsLoader.GetLocale())
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
			profile, err = workspacediscovery.ReconcileProfile(workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig), storedProfile)
			if err != nil {
				return nil, nil, false, err
			}
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
			spec, err = workspacediscovery.ReconcileSpec(
				workspacediscovery.SpecFromProfile(profile, g.skillsLoader.GetLocale()),
				storedSpec,
				profile,
				workspacediscovery.ValidationOptions{RootPath: projectRoot},
			)
			if err != nil {
				return nil, nil, false, err
			}
			loaded = true
		}
	}
	if spec == nil {
		spec = workspacediscovery.SpecFromProfile(profile, g.skillsLoader.GetLocale())
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

func (g *WorkspaceGenerator) writeWorkspaceRootSkill(ctx context.Context, outputPath string, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec, opts WorkspaceGenerateOptions) error {
	data, err := g.workspaceTemplateData(ctx, projectConfig, workspaceConfig, profile, spec, opts)
	if err != nil {
		return err
	}
	return skilloutput.ReplaceWithinRoot(projectConfig.RootPath, outputPath, func(staging string) error {
		if err := generator.WriteWorkflowOutputs(g.workflowRepo, staging, g.skillsLoader.GetLocale()); err != nil {
			return err
		}
		p := skillgen.NewPlan(staging)
		p.AddFile("SKILL.md", skillgen.CatalogTemplate, "workspace-skill", data)
		p.AgentMetadataData = data
		if !opts.SkipReferences {
			p.AddDir("references")
			p.AddFile("references/workspace-overview.md", skillgen.CatalogTemplate, "workspace-reference-overview", data)
			p.AddFile("references/cross-project-rules.md", skillgen.CatalogTemplate, "workspace-reference-cross-project-rules", data)
		}
		return g.renderer.Render(ctx, p)
	})
}

func (g *WorkspaceGenerator) workspaceTemplateData(ctx context.Context, projectConfig config.ProjectConfig, workspaceConfig config.WorkspaceConfig, profile *domain.WorkspaceProfile, spec *domain.WorkspaceSpec, opts WorkspaceGenerateOptions) (workspaceSkillTemplateData, error) {
	name := workspaceNameOrDefault(projectConfig.Name)
	projects := make([]workspaceProjectTemplateData, 0, len(workspaceConfig.Projects))
	for _, project := range workspaceConfig.Projects {
		target, err := workspacediscovery.ResolveChildSkillTarget(projectConfig.RootPath, project, g.configRepo)
		if err != nil {
			return workspaceSkillTemplateData{}, err
		}
		profileProject := workspaceProjectByID(profile, project.ID)
		childSkillDir := relativeWorkspacePath(projectConfig.RootPath, target.OutputPath)
		skillPath := filepath.ToSlash(filepath.Join(childSkillDir, "SKILL.md"))
		selfManagedConfigPath := filepath.ToSlash(filepath.Join(project.Path, ".skills-seed", "config.yaml"))
		projects = append(projects, workspaceProjectTemplateData{
			WorkspaceProjectConfig: project,
			SkillName:              skillgen.GeneratedSkillName(project.ID),
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
		shared = profile.Shared
		contracts = profile.Contracts
		infra = profile.Infra
		dependencies = profile.Dependencies
		impactRoutes = profile.ImpactRoutes
	}
	var routing []domain.WorkspaceRoute
	var rules []domain.WorkspaceRule
	var guidance []domain.WorkspaceRule
	var changeOrder []string
	var parallelGuidance []domain.WorkspaceParallelGuidance
	var loadMultipleWhen []domain.WorkspaceLoadMultipleSkill
	if spec != nil {
		routing = spec.Routing
		for _, rule := range spec.Rules {
			if rule.Authority == domain.WorkspaceRuleAuthorityInferred {
				guidance = append(guidance, rule)
				continue
			}
			rules = append(rules, rule)
		}
		changeOrder = spec.ChangeOrder
		parallelGuidance = spec.ParallelAgentGuidance
		loadMultipleWhen = spec.LoadMultipleSkillsWhen
	}
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
		SkillName:           skillgen.GeneratedWorkspaceSkillName(name),
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
		Guidance:            guidance,
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
		HasGuidance:         len(guidance) > 0,
		HasChangeOrder:      len(changeOrder) > 0,
		HasParallelGuidance: len(parallelGuidance) > 0,
		HasLoadMultipleWhen: len(loadMultipleWhen) > 0,
		HasWorkflowRefs:     len(workflowReferences) > 0,
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
