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
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

// GeneratorService 生成服务
type GeneratorService struct {
	patternRepo       domain.PatternRepository
	patternStatsRepo  domain.PatternStatsRepository
	profileRepo       domain.ProjectProfileRepository
	scopedProfileRepo domain.ScopedProjectProfileRepository
	projectSpecRepo   domain.ProjectSpecRepository
	scopedSpecRepo    domain.ScopedProjectSpecRepository
	workflowRepo      domain.WorkflowRepository
	skillsLoader      *skills.Loader
	planBuilder       *planBuilder
	renderer          *skillgen.Renderer
	configRepo        config.Reader
}

// SetWorkflowRepository 注入用户工作流仓储。
func (s *GeneratorService) SetWorkflowRepository(repo domain.WorkflowRepository) {
	s.workflowRepo = repo
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
	configRepo config.Reader,
) *GeneratorService {
	scopedProfileRepo, _ := profileRepo.(domain.ScopedProjectProfileRepository)
	projectSpecRepo, _ := profileRepo.(domain.ProjectSpecRepository)
	scopedSpecRepo, _ := profileRepo.(domain.ScopedProjectSpecRepository)
	patternStatsRepo, _ := patternRepo.(domain.PatternStatsRepository)
	return &GeneratorService{
		patternRepo:       patternRepo,
		patternStatsRepo:  patternStatsRepo,
		profileRepo:       profileRepo,
		scopedProfileRepo: scopedProfileRepo,
		projectSpecRepo:   projectSpecRepo,
		scopedSpecRepo:    scopedSpecRepo,
		skillsLoader:      skillsLoader,
		planBuilder:       newPlanBuilder(skillsLoader),
		renderer:          skillgen.NewRenderer(skillsLoader),
		configRepo:        configRepo,
	}
}

// GenerateSkills 生成 Skills 文件夹
func (s *GeneratorService) GenerateSkills(ctx context.Context, outputPath string) error {
	return s.GenerateSkillsWithProgress(ctx, outputPath, nil, nil)
}

func (s *GeneratorService) GenerateSkillsWithOptions(ctx context.Context, outputPath string, opts GenerateOptions) error {
	return s.GenerateSkillsWithHooks(ctx, outputPath, GenerateProgressHooks{}, opts)
}

func (s *GeneratorService) GenerateSkillsWithProgress(ctx context.Context, outputPath string, onStepStart func(label string), onStepComplete func(label string)) error {
	return s.GenerateSkillsWithHooks(ctx, outputPath, GenerateProgressHooks{
		OnStepStart:    onStepStart,
		OnStepComplete: onStepComplete,
	}, GenerateOptions{})
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

	stats := s.calculateStats(patterns)

	var profile *domain.ProjectProfile
	if err := runStep(i18n.Get("ProgressGenerateLoadProfile"), func() error {
		var profileErr error
		profile, profileErr = s.loadProjectProfile(ctx)
		return profileErr
	}); err != nil {
		return err
	}
	profile = cleanProjectProfile(profile)
	projectConfig := s.configRepo.GetProjectConfig()
	projectRoot := firstNonEmptyString(projectConfig.RootPath, runtimecontext.ProjectRoot(ctx))
	profile, rankedPatterns = sanitizeGenerationInputs(profile, rankedPatterns, projectRoot)
	spec := s.projectSpecFromProfileAndPatterns(profile, rankedPatterns, config.WorkspaceProjectConfig{})

	if err := runStep(i18n.Get("ProgressGenerateCheckOutput"), func() error {
		return s.ensureGeneratedOutputDirWritable(resolvedOutputPath)
	}); err != nil {
		return err
	}

	var summaryResult *agent.GenerateSkillsResult
	if err := runStep(i18n.Get("ProgressGenerateSummary"), func() error {
		summaryResult = s.buildDeterministicSummary(rankedPatterns, patternInsights)
		return nil
	}); err != nil {
		return err
	}

	if err := runStep(i18n.Get("ProgressGenerateWriteSkills"), func() error {
		if s.projectSpecRepo != nil {
			if err := s.projectSpecRepo.SaveSpec(ctx, spec); err != nil {
				return err
			}
		}

		// 确保分类摘要非 nil
		if summaryResult.CategorySummaries == nil {
			summaryResult.CategorySummaries = map[string]agent.CategorySummary{}
		}
		summaryResult.CategorySummaries = s.ensureCategorySummaries(rankedPatterns, summaryResult.CategorySummaries)

		workflowReferences, err := s.loadWorkflowReferences()
		if err != nil {
			return err
		}
		if err := s.rebuildGeneratedOutputDir(resolvedOutputPath); err != nil {
			return err
		}
		if err := s.writeWorkflowOutputs(resolvedOutputPath); err != nil {
			return err
		}

		plan, err := s.planBuilder.Build(resolvedOutputPath, rankedPatterns, summaryResult, stats, profile, spec, PlanOptions{
			SkillName:           generatedSkillName(projectConfig.Name),
			ProjectName:         projectConfig.Name,
			Language:            projectConfig.Language,
			ProgramVersion:      metadata.ProgramVersion,
			SkillsTemplatesHash: metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS)),
			SkipReferences:      opts.SkipReferences,
			WorkflowReferences:  workflowReferences,
			ProjectRoot:         projectRoot,
		})
		if err != nil {
			return err
		}
		return s.renderer.Render(ctx, plan)
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
		"categories_count", len(stats.ByCategory),
	)
	return nil
}

func (s *GeneratorService) ensureGeneratedOutputDirWritable(outputPath string) error {
	if err := skilloutput.EnsureWritable(outputPath); err != nil {
		var manualErr *skilloutput.ManualSkillExistsError
		if errors.As(err, &manualErr) {
			return &ManualSkillExistsError{Path: manualErr.Path}
		}
		return err
	}
	return nil
}

func (s *GeneratorService) rebuildGeneratedOutputDir(outputPath string) error {
	if err := skilloutput.Rebuild(outputPath); err != nil {
		var manualErr *skilloutput.ManualSkillExistsError
		if errors.As(err, &manualErr) {
			return &ManualSkillExistsError{Path: manualErr.Path}
		}
		return err
	}
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
