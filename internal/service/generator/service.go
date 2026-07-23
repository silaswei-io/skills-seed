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
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/skilloutput"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

// GeneratorService 生成服务
type GeneratorService struct {
	patternRepo      patternReader
	patternStatsRepo patternStatsReader
	profileRepo      profileReader
	projectSpecRepo  projectSpecWriter
	workflowRepo     domain.WorkflowRepository
	skillsLoader     *skills.Loader
	planBuilder      *planBuilder
	renderer         *skillgen.Renderer
	configRepo       config.Reader
	symbolResolver   sourcecode.Resolver
}

// NewGeneratorService 创建生成服务
func NewGeneratorService(
	patternRepo patternReader,
	profileRepo profileReader,
	skillsLoader *skills.Loader,
	configRepo config.Reader,
	workflowRepo domain.WorkflowRepository,
) *GeneratorService {
	projectSpecRepo, _ := profileRepo.(projectSpecWriter)
	patternStatsRepo, _ := patternRepo.(patternStatsReader)
	structuralConfig := config.StructuralConfig{Provider: config.StructuralProviderAuto}
	if configRepo != nil {
		structuralConfig = configRepo.GetCurrentLearningConfig().Structural
	}
	return &GeneratorService{
		patternRepo:      patternRepo,
		patternStatsRepo: patternStatsRepo,
		profileRepo:      profileRepo,
		projectSpecRepo:  projectSpecRepo,
		workflowRepo:     workflowRepo,
		skillsLoader:     skillsLoader,
		planBuilder:      newPlanBuilder(skillsLoader),
		renderer:         skillgen.NewRenderer(skillsLoader),
		configRepo:       configRepo,
		symbolResolver:   sourcecode.NewResolver(structuralConfig),
	}
}

// GenerateSkills 生成 Skills 文件夹
func (s *GeneratorService) GenerateSkills(ctx context.Context, outputPath string) error {
	return s.GenerateSkillsWithHooks(ctx, outputPath, GenerateProgressHooks{}, GenerateOptions{})
}

func (s *GeneratorService) GenerateSkillsWithHooks(ctx context.Context, outputPath string, hooks GenerateProgressHooks, opts GenerateOptions) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_skills",
		"output_path", outputPath,
	)
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
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		patterns = activePatterns(patterns)
		return loadErr
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
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

	var profile *domain.ProjectProfile
	if err := runStep(i18n.Get("ProgressGenerateLoadProfile"), func() error {
		var profileErr error
		profile, profileErr = s.loadProjectProfile(ctx)
		return profileErr
	}); err != nil {
		return err
	}
	projectConfig := s.configRepo.GetProjectConfig()
	projectRoot := firstNonEmptyString(projectConfig.RootPath, runtimecontext.ProjectRoot(ctx))
	snapshot, err := s.buildVerifiedKnowledgeSnapshot(ctx, profile, rankedPatterns, projectRoot)
	if err != nil {
		return fmt.Errorf("scan project validation facts: %w", err)
	}
	var summaryResult generationSummary
	if err := runStep(i18n.Get("ProgressGenerateSummary"), func() error {
		summaryResult = s.buildDeterministicSummary(snapshot.Patterns, patternInsights)
		return nil
	}); err != nil {
		return err
	}

	if err := runStep(i18n.Get("ProgressGenerateWriteSkills"), func() error {
		if s.projectSpecRepo != nil {
			if err := s.projectSpecRepo.SaveSpec(ctx, snapshot.Spec); err != nil {
				return err
			}
		}

		// 确保分类摘要非 nil
		workflowReferences, err := s.loadWorkflowReferences()
		if err != nil {
			return err
		}
		return skilloutput.ReplaceWithinRoot(projectRoot, resolvedOutputPath, func(staging string) error {
			if err := s.writeWorkflowOutputs(staging); err != nil {
				return err
			}
			plan, err := s.planBuilder.Build(staging, snapshot, summaryResult, PlanOptions{
				SkillName:           skillgen.GeneratedSkillName(projectConfig.Name),
				ProjectName:         projectConfig.Name,
				Language:            projectConfig.Language,
				ProgramVersion:      metadata.ProgramVersion,
				SkillsTemplatesHash: metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS)),
				SkipReferences:      opts.SkipReferences,
				WorkflowReferences:  workflowReferences,
			})
			if err != nil {
				return err
			}
			return s.renderer.Render(ctx, plan)
		})
	}); err != nil {
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
		"categories_count", len(domain.CategoryNamesWithPatterns(snapshot.Patterns)),
	)
	return nil
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
