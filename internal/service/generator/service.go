package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	skillsLoader      *skills.Loader
	writer            *SkillWriter
	agent             agent.Agent
	configRepo        config.Reader
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
	patternStatsRepo, _ := patternRepo.(domain.PatternStatsRepository)
	return &GeneratorService{
		patternRepo:       patternRepo,
		patternStatsRepo:  patternStatsRepo,
		profileRepo:       profileRepo,
		scopedProfileRepo: scopedProfileRepo,
		projectSpecRepo:   projectSpecRepo,
		scopedSpecRepo:    scopedSpecRepo,
		skillsLoader:      skillsLoader,
		writer:            NewSkillWriter(skillsLoader),
		agent:             ag,
		configRepo:        configRepo,
	}
}

// GenerateSkills 生成 Skills 文件夹
func (s *GeneratorService) GenerateSkills(ctx context.Context, outputPath string) error {
	return s.GenerateSkillsWithProgress(ctx, outputPath, nil, nil)
}

func (s *GeneratorService) GenerateSkillsWithOptions(ctx context.Context, outputPath string, opts GenerateOptions) error {
	return s.GenerateSkillsWithProgress(ctx, outputPath, nil, nil)
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
		return loadErr
	}); err != nil {
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
	spec := s.projectSpecFromProfileAndPatterns(profile, rankedPatterns, config.WorkspaceProjectConfig{})

	var decision *domain.InputFingerprintDecision
	skipGeneration := false
	if err := runStep(i18n.Get("ProgressGenerateCheckFingerprint"), func() error {
		var fingerprintErr error
		decision, fingerprintErr = s.prepareProjectSkillsFingerprint(ctx, resolvedOutputPath, rankedPatterns, patternInsights, profile)
		if fingerprintErr != nil {
			return fingerprintErr
		}
		skipGeneration = decision.ShouldSkip() && projectSkillsOutputExists(resolvedOutputPath, rankedPatterns)
		return nil
	}); err != nil {
		return err
	}

	var summaryResult *agent.GenerateSkillsResult
	if err := runStep(i18n.Get("ProgressGenerateSummary"), func() error {
		if skipGeneration {
			return nil
		}
		var summaryErr error
		summaryResult, summaryErr = s.generateSkillsSummary(ctx, rankedPatterns, patternInsights, resolvedOutputPath, startedAt)
		return summaryErr
	}); err != nil {
		return err
	}

	if err := runStep(i18n.Get("ProgressGenerateWriteSkills"), func() error {
		if skipGeneration {
			logger.Info(i18n.Get("GenerateSkillsSkipped"))
			return nil
		}
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

		mainPath := filepath.Join(resolvedOutputPath, "SKILL.md")
		if err := s.ensureSkillWritable(mainPath); err != nil {
			return err
		}

		projectConfig := s.configRepo.GetProjectConfig()
		return s.writer.WriteSkillsOutput(ctx, resolvedOutputPath, rankedPatterns, summaryResult, stats, profile, spec, SkillWriteOptions{
			SkillName:           generatedSkillName(projectConfig.Name),
			ProjectName:         projectConfig.Name,
			Language:            projectConfig.Language,
			ProgramVersion:      metadata.ProgramVersion,
			SkillsTemplatesHash: metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS)),
			SkipReferences:      opts.SkipReferences,
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
	if !skipGeneration {
		if err := decision.Commit(ctx, s.fileAnalysisTracker()); err != nil {
			return err
		}
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

func (s *GeneratorService) generateSkillsSummary(ctx context.Context, patterns []domain.Pattern, insights map[string]domain.PatternInsight, resolvedOutputPath string, startedAt time.Time) (*agent.GenerateSkillsResult, error) {
	patternsJSONBytes, err := json.MarshalIndent(summarizePatternsForAgent(patterns, insights), "", "  ")
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

func summarizePatternsForAgent(patterns []domain.Pattern, insights map[string]domain.PatternInsight) []map[string]interface{} {
	summary := make([]map[string]interface{}, 0, len(patterns))
	for _, p := range patterns {
		insight := insights[p.ID]
		lastHitAt := ""
		if !insight.LastHitAt.IsZero() {
			lastHitAt = insight.LastHitAt.Format(time.RFC3339)
		}
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
			"metrics":         p.Metrics,
			"hit_count":       insight.HitCount,
			"last_hit_at":     lastHitAt,
			"generation_rank": insight.GenerationRank,
		})
	}
	return summary
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
