package generator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/skilloutput"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

// GeneratorService 生成服务
type GeneratorService struct {
	patternRepo    patternReader
	profileRepo    profileReader
	workflowRepo   domain.WorkflowRepository
	ruleRepo       domain.RuleRepository
	skillsLoader   *skills.Loader
	planBuilder    *planBuilder
	renderer       *skillgen.Renderer
	configRepo     config.Reader
	symbolResolver sourcecode.Resolver
}

// NewGeneratorService 创建生成服务
func NewGeneratorService(
	patternRepo patternReader,
	profileRepo profileReader,
	skillsLoader *skills.Loader,
	configRepo config.Reader,
	workflowRepo domain.WorkflowRepository,
	ruleRepo domain.RuleRepository,
) *GeneratorService {
	structuralConfig := config.StructuralConfig{Provider: config.StructuralProviderAuto}
	if configRepo != nil {
		structuralConfig = configRepo.GetCurrentLearningConfig().Structural
	}
	return &GeneratorService{
		patternRepo:    patternRepo,
		profileRepo:    profileRepo,
		workflowRepo:   workflowRepo,
		ruleRepo:       ruleRepo,
		skillsLoader:   skillsLoader,
		planBuilder:    newPlanBuilder(skillsLoader),
		renderer:       skillgen.NewRenderer(skillsLoader),
		configRepo:     configRepo,
		symbolResolver: sourcecode.NewResolver(structuralConfig),
	}
}

// GenerateSkills 生成 Skills 文件夹
func (s *GeneratorService) GenerateSkills(ctx context.Context, outputPath string) error {
	return s.GenerateSkillsWithOptions(ctx, outputPath, GenerateOptions{})
}

// GenerateSkillsWithOptions 使用显式运行参数生成 Skill。
func (s *GeneratorService) GenerateSkillsWithOptions(ctx context.Context, outputPath string, opts GenerateOptions) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_skills",
		"output_path", outputPath,
	)
	hooks := opts.Progress
	retryProgress := agent.NewRetryProgressBinder(hooks.OnStepUpdate)
	ctx = retryProgress.WithContext(ctx)

	runStep := func(label string, fn func() error) error {
		retryProgress.StartStep(label)
		if hooks.OnStepStart != nil {
			hooks.OnStepStart(label)
		}
		if err := fn(); err != nil {
			retryProgress.FinishStep(label, false)
			return err
		}
		retryProgress.FinishStep(label, true)
		if hooks.OnStepComplete != nil {
			hooks.OnStepComplete(label)
		}
		return nil
	}

	var resolvedOutputPath string
	if err := runStep(i18n.Get("ProgressGenerateResolveOutput"), func() error {
		var resolveErr error
		resolvedOutputPath, resolveErr = s.resolveOutputPath(outputPath)
		return resolveErr
	}); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.resolve_output_path",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	var patterns []domain.Pattern
	if err := runStep(i18n.Get("ProgressGenerateLoadPatterns"), func() error {
		var loadErr error
		patterns, loadErr = s.patternRepo.GetAll(ctx)
		patterns = domain.ActivePatterns(patterns)
		return loadErr
	}); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.load_patterns",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return err
	}

	patternInsights, err := s.patternGenerationInsights(ctx, patterns)
	if err != nil {
		return err
	}
	rankedPatterns := rankPatternsForGeneration(patterns, patternInsights)

	workflowReferences, err := s.loadWorkflowReferences()
	if err != nil {
		return err
	}
	rules, err := s.loadRules(opts.ProjectedRules)
	if err != nil {
		return err
	}
	ruleReferences := ruleReferences(rules)
	var profile *domain.ProjectProfile
	resourceOnly := false
	if err := runStep(i18n.Get("ProgressGenerateLoadProfile"), func() error {
		var profileErr error
		profile, resourceOnly, profileErr = s.loadProjectProfileForGeneration(ctx, workflowReferences, ruleReferences)
		return profileErr
	}); err != nil {
		return err
	}
	projectConfig := s.configRepo.GetProjectConfig()
	targetAgent := "agent"
	if s.configRepo != nil {
		targetAgent = s.configRepo.GetEffectiveSkillsTarget()
	}
	templatesHash := metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS))
	projectRoot := stringx.FirstNonBlank(projectConfig.RootPath, runtimecontext.ProjectRoot(ctx))
	snapshot := verifiedKnowledgeSnapshot{RenderProfile: profile}
	if !resourceOnly {
		var snapshotErr error
		snapshot, snapshotErr = s.buildVerifiedKnowledgeSnapshot(ctx, profile, rankedPatterns, projectRoot)
		if snapshotErr != nil {
			return fmt.Errorf("build verified knowledge snapshot: %w", snapshotErr)
		}
	}
	var summaryResult generationSummary
	if err := runStep(i18n.Get("ProgressGenerateSummary"), func() error {
		summaryResult = s.buildDeterministicSummary(snapshot.Patterns, patternInsights)
		return nil
	}); err != nil {
		return err
	}

	if err := runStep(i18n.Get("ProgressGenerateWriteSkills"), func() error {
		return skilloutput.ReplaceWithinRoot(projectRoot, resolvedOutputPath, func(staging string) error {
			if err := writeRuleOutputs(rules, staging); err != nil {
				return err
			}
			if err := s.writeWorkflowOutputs(staging); err != nil {
				return err
			}
			plan, err := s.planBuilder.Build(staging, snapshot, summaryResult, PlanOptions{
				SkillName:           skillgen.GeneratedSkillName(projectConfig.Name),
				ProjectName:         projectConfig.Name,
				Language:            projectConfig.Language,
				ProgramVersion:      metadata.ProgramVersion,
				SkillsTemplatesHash: templatesHash,
				ResourceOnly:        resourceOnly,
				WorkflowReferences:  workflowReferences,
				RuleReferences:      ruleReferences,
			})
			if err != nil {
				return err
			}
			if err := s.renderer.Render(ctx, plan); err != nil {
				return err
			}
			if hooks.OnStepUpdate != nil {
				hooks.OnStepUpdate(i18n.Get("ProgressGenerateAuditReadiness"))
			}
			if err := skilloutput.AuditReadiness(staging, skillReadinessRequirements(plan, snapshot)); err != nil {
				return err
			}
			return skilloutput.WriteManifest(staging, skilloutput.Manifest{
				KnowledgeSnapshotHash: knowledgeSnapshotHash(snapshot),
				ProgramVersion:        metadata.ProgramVersion,
				TargetAgent:           targetAgent,
				TemplatesHash:         templatesHash,
			})
		})
	}); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_skills",
			"duration", time.Since(startedAt),
			"patterns_count", len(patterns),
			"render_patterns_count", len(snapshot.Patterns),
			"resolved_output_path", resolvedOutputPath,
			"error", err,
		)
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_skills",
		"duration", time.Since(startedAt),
		"patterns_count", len(patterns),
		"render_patterns_count", len(snapshot.Patterns),
		"resolved_output_path", resolvedOutputPath,
		"categories_count", len(domain.CategoryNamesWithPatterns(snapshot.Patterns)),
	)
	return nil
}

func skillReadinessRequirements(plan *skillgen.Plan, snapshot verifiedKnowledgeSnapshot) skilloutput.ReadinessRequirements {
	requirements := skilloutput.ReadinessRequirements{RequiredContent: make(map[string][]string)}
	if plan != nil {
		for _, file := range plan.Files {
			requirements.ExpectedFiles = append(requirements.ExpectedFiles, file.Path)
		}
	}
	if snapshot.RenderProfile == nil {
		return requirements
	}
	for _, rule := range snapshot.RenderProfile.EngineeringRules {
		requirements.RequiredContent["references/project-spec.md"] = append(requirements.RequiredContent["references/project-spec.md"], rule.Rule)
		if rule.CommandPolicy == "" {
			continue
		}
		requirements.RequiredContent["SKILL.md"] = append(requirements.RequiredContent["SKILL.md"], rule.Title, rule.CommandPolicy)
	}
	for _, coverage := range snapshot.RenderProfile.AuthorityCoverage {
		requirements.RequiredContent["references/project-spec.md"] = append(requirements.RequiredContent["references/project-spec.md"], coverage.Source)
	}
	return requirements
}

func (s *GeneratorService) loadProjectProfileForGeneration(ctx context.Context, workflowReferences []WorkflowReference, ruleReferences []RuleReference) (*domain.ProjectProfile, bool, error) {
	profile, err := s.loadProjectProfile(ctx)
	if err == nil {
		return profile, false, nil
	}
	if len(workflowReferences) == 0 && len(ruleReferences) == 0 || !errors.Is(err, errProjectProfileMissing) {
		return nil, false, err
	}
	return s.fallbackProjectProfile(), true, nil
}

func (s *GeneratorService) loadProjectProfile(ctx context.Context) (*domain.ProjectProfile, error) {
	if s.profileRepo == nil {
		return nil, missingProjectProfileError()
	}

	profile, err := s.profileRepo.Get(ctx)
	if err != nil {
		if errors.Is(err, profilestore.ErrProfileNotFound) {
			return nil, missingProjectProfileError()
		}
		return nil, err
	}
	return profile, nil
}

var errProjectProfileMissing = errors.New("project profile missing")

type projectProfileMissingError struct{}

func (projectProfileMissingError) Error() string {
	return i18n.Get("GenerateProjectProfileMissing")
}

func (projectProfileMissingError) Is(target error) bool {
	return target == errProjectProfileMissing
}

func missingProjectProfileError() error {
	return projectProfileMissingError{}
}

func (s *GeneratorService) fallbackProjectProfile() *domain.ProjectProfile {
	profile := &domain.ProjectProfile{}
	if s.configRepo != nil {
		projectConfig := s.configRepo.GetProjectConfig()
		profile.ProjectName = projectConfig.Name
		profile.Language = projectConfig.Language
	}
	return profile
}
