package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/utils"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

const (
	learnCurrentProfileAuto    = "auto"
	learnCurrentProfileSkip    = "skip"
	learnCurrentProfileRefresh = "refresh"
)

var sleepAfterWorkspaceChildStep = time.Sleep

type learnCurrentOptions struct {
	language    string
	focusPaths  []string
	profileMode string
	contextText string
	contextFile string
	userContext string
}

type learnHistoryOptions struct {
	limit     int
	since     string
	batchSize int
}

type workspaceLearnInputData struct {
	Name       string                       `json:"name"`
	RootPath   string                       `json:"root_path"`
	Projects   []workspaceLearnInputProject `json:"projects"`
	ConfigPath string                       `json:"config_path,omitempty"`
}

type workspaceLearnInputProject struct {
	ID                 string   `json:"id"`
	Path               string   `json:"path"`
	Type               string   `json:"type"`
	Language           string   `json:"language"`
	SkillPath          string   `json:"skill_path,omitempty"`
	ProjectProfilePath string   `json:"project_profile_path,omitempty"`
	ProjectSpecPath    string   `json:"project_spec_path,omitempty"`
	Summary            string   `json:"summary,omitempty"`
	Frameworks         []string `json:"frameworks,omitempty"`
	KeyModules         []string `json:"key_modules,omitempty"`
}

type workspaceRelationshipFingerprintInput struct {
	Kind                string                  `json:"kind"`
	ProgramVersion      string                  `json:"program_version"`
	PromptTemplatesHash string                  `json:"prompt_templates_hash"`
	WorkspaceInput      workspaceLearnInputData `json:"workspace_input"`
	UserContext         string                  `json:"user_context,omitempty"`
}

// Cmd 返回 learn 命令
func Cmd(cont *container.Container) *cobra.Command {
	learnCmd := &cobra.Command{
		Use:     "learn",
		Short:   i18n.Get("LearnShort"),
		Long:    i18n.Get("LearnLongDesc"),
		Example: i18n.Get("LearnExample"),
	}
	defaultLimit, defaultBatchSize := historyDefaults(cont)

	// learn current 子命令
	currentOpts := learnCurrentOptions{profileMode: learnCurrentProfileAuto}
	currentCmd := &cobra.Command{
		Use:     "current",
		Short:   i18n.Get("LearnCurrentShort"),
		Long:    i18n.Get("LearnCurrentLongDesc"),
		Example: i18n.Get("LearnCurrentExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return runLearnCurrent(cont, currentOpts)
		},
	}
	currentCmd.Flags().StringVarP(&currentOpts.language, "language", "l", "", i18n.Get("LearnFlagLanguage"))
	currentCmd.Flags().StringArrayVarP(&currentOpts.focusPaths, "focus", "f", nil, i18n.Get("LearnFlagFocus"))
	currentCmd.Flags().StringVar(&currentOpts.profileMode, "profile", learnCurrentProfileAuto, i18n.Get("LearnFlagProfile"))
	currentCmd.Flags().StringVar(&currentOpts.contextText, "context", "", i18n.Get("LearnFlagContext"))
	currentCmd.Flags().StringVar(&currentOpts.contextFile, "context-file", "", i18n.Get("LearnFlagContextFile"))

	// learn history 子命令
	historyOpts := learnHistoryOptions{limit: defaultLimit, batchSize: defaultBatchSize}
	historyCmd := &cobra.Command{
		Use:     "history",
		Short:   i18n.Get("LearnHistoryShort"),
		Long:    i18n.Get("LearnHistoryLongDesc"),
		Example: i18n.Get("LearnHistoryExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return runLearnHistory(cont, historyOpts)
		},
	}
	historyCmd.Flags().IntVarP(&historyOpts.limit, "limit", "n", defaultLimit, i18n.Get("LearnFlagLimit"))
	historyCmd.Flags().StringVarP(&historyOpts.since, "since", "s", "", i18n.Get("LearnFlagSince"))
	historyCmd.Flags().IntVarP(&historyOpts.batchSize, "batch-size", "b", defaultBatchSize, i18n.Get("LearnFlagBatchSize"))

	learnCmd.AddCommand(currentCmd, historyCmd)

	return learnCmd
}

func historyDefaults(cont *container.Container) (int, int) {
	defaultLimit := 50
	defaultBatchSize := 10
	if cont == nil || cont.ConfigRepo == nil {
		return defaultLimit, defaultBatchSize
	}
	learningConfig := cont.ConfigRepo.GetLearningConfig()
	if learningConfig.History.MaxCommits > 0 {
		defaultLimit = learningConfig.History.MaxCommits
	}
	if learningConfig.History.BatchSize > 0 {
		defaultBatchSize = learningConfig.History.BatchSize
	}
	return defaultLimit, defaultBatchSize
}

// RunLearnCurrent 导出：从当前代码库学习（供 sync 调用）
func RunLearnCurrent(cont *container.Container) error {
	return runLearnCurrent(cont, learnCurrentOptions{profileMode: learnCurrentProfileAuto})
}

// RunLearnCurrentWithContext 导出：从当前代码库学习，并附加一次性用户上下文（供 sync 调用）
func RunLearnCurrentWithContext(cont *container.Container, userContext string) error {
	return runLearnCurrent(cont, learnCurrentOptions{profileMode: learnCurrentProfileAuto, userContext: userContext})
}

func runLearnCurrent(cont *container.Container, opts learnCurrentOptions) error {
	if opts.profileMode == "" {
		opts.profileMode = learnCurrentProfileAuto
	}
	if opts.userContext == "" {
		userContext, err := commandutil.ResolveRuntimeContext(opts.contextText, opts.contextFile)
		if err != nil {
			return err
		}
		opts.userContext = userContext
	}
	if cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return runLearnWorkspaceCurrent(cont, opts)
	}
	return runLearnCurrentProject(cont, opts)
}

func runLearnCurrentProject(cont *container.Container, opts learnCurrentOptions) error {
	_, err := runLearnCurrentProjectWithOptions(cont, learnCurrentProjectOptions{
		showProgress:     true,
		showDetailedLogs: true,
		userContext:      opts.userContext,
		language:         opts.language,
		focusPaths:       opts.focusPaths,
		profileMode:      opts.profileMode,
	})
	return err
}

type learnCurrentProjectOptions struct {
	tokenScope       string
	showProgress     bool
	showDetailedLogs bool
	userContext      string
	onStepStart      func(label string)
	onStepComplete   func(label string)
	onStepUpdate     func(label string)
	language         string
	focusPaths       []string
	profileMode      string
}

type learnCurrentProjectResult struct {
	projectName   string
	changedCount  int
	deletedCount  int
	skippedCount  int
	patternsCount int
	savedCount    int
	skipped       bool
	duration      time.Duration
	tokenContext  context.Context
}

type aiFileSelectionSummary struct {
	Applied        bool
	CandidateCount int
	SelectedCount  int
	SkippedCount   int
	Reason         string
}

func runLearnCurrentProjectWithOptions(cont *container.Container, opts learnCurrentProjectOptions) (*learnCurrentProjectResult, error) {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return nil, err
	}

	ctx := agent.WithTokenUsageScope(context.Background(), opts.tokenScope)
	ctx = runtimecontext.WithSeedPath(ctx, cont.SeedPath)
	ctx = runtimecontext.WithUserContext(ctx, opts.userContext)
	startedAt := time.Now()
	tracker := progress.New(5)
	retryProgress := agent.NewRetryProgressBinder(func(label string) {
		if opts.onStepUpdate != nil {
			opts.onStepUpdate(label)
		}
		if opts.showProgress {
			tracker.UpdateStep(label)
		}
	})
	ctx = retryProgress.WithContext(ctx)
	runStep := func(label string, fn func() error) error {
		retryProgress.StartStep(label)
		if opts.onStepStart != nil {
			opts.onStepStart(label)
		}
		var err error
		if opts.showProgress {
			err = tracker.RunStep(label, fn)
		} else {
			err = fn()
		}
		if err != nil {
			retryProgress.FinishStep(label, false)
			return err
		}
		retryProgress.FinishStep(label, true)
		if opts.onStepComplete != nil {
			opts.onStepComplete(label)
		}
		return nil
	}

	var projectRoot string
	var projectName string
	var currentLanguage string
	var resolvedFocusPaths []string
	var refreshProfile bool
	var existingProfile *domain.ProjectProfile

	if opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentStart"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "command.learn_current",
		"agent", cont.Agent.Name(),
		"seed_path", cont.SeedPath,
	)

	// 解析项目上下文可能访问 Git 和配置文件，单独作为第一步展示，避免用户以为命令无响应
	prepareStartedAt := time.Now()
	if err := runStep(i18n.Get("ProgressLearnCurrentPrepareProject"), func() error {
		var err error
		projectRoot, err = cont.GitRepo.GetProjectRoot(ctx)
		if err != nil {
			projectRoot = cont.ConfigRepo.GetProjectConfig().RootPath
		}
		if projectRoot == "" {
			projectRoot, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		projectName = filepath.Base(projectRoot)
		if configuredName := cont.ConfigRepo.GetProjectConfig().Name; configuredName != "" {
			projectName = configuredName
		}

		currentLanguage = opts.language
		if currentLanguage == "" {
			currentLanguage = cont.ConfigRepo.GetProjectConfig().Language
		}
		if currentLanguage == "" {
			currentLanguage = "unknown"
		}

		resolvedFocusPaths, err = resolveFocusPaths(projectRoot, opts.focusPaths)
		if err != nil {
			return err
		}
		profileExists := false
		if cont.ProfileRepo != nil {
			if profile, getErr := cont.ProfileRepo.Get(ctx); getErr == nil {
				existingProfile = profile
				profileExists = true
			}
		}
		refreshProfile, err = shouldRefreshProfile(projectRoot, resolvedFocusPaths, opts.profileMode, profileExists)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.prepare_project",
			"duration", time.Since(prepareStartedAt),
			"error", err,
		)
		return nil, fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGetCurrentDir", map[string]interface{}{"Error": err.Error()}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.prepare_project",
		"duration", time.Since(prepareStartedAt),
		"project_root", projectRoot,
		"project_name", projectName,
		"language", currentLanguage,
		"focus_paths", strings.Join(utils.RelativePaths(projectRoot, resolvedFocusPaths), ","),
		"profile_mode", opts.profileMode,
		"refresh_profile", refreshProfile,
	)
	if opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentInfo", map[string]interface{}{
			"ProjectRoot": projectRoot,
			"ProjectName": projectName,
			"Language":    currentLanguage,
		}))
		if len(resolvedFocusPaths) > 0 {
			logger.Info(i18n.GetWithParams("LearnCurrentFocusInfo", map[string]interface{}{
				"Focus":       strings.Join(utils.RelativePaths(projectRoot, resolvedFocusPaths), ", "),
				"ProfileMode": opts.profileMode,
			}))
		}
	}

	var incrementalChanges *incrementalFileChanges
	var effectiveFocusPaths []string
	var selectedFiles []domain.FileInfo
	var selectionSummary aiFileSelectionSummary
	detectStartedAt := time.Now()
	if err := runStep(i18n.Get("ProgressLearnCurrentDetectChanges"), func() error {
		var err error
		incrementalChanges, err = prepareIncrementalFileChanges(ctx, cont.FileTracker, cont.ConfigRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, resolvedFocusPaths)
		if err != nil {
			return err
		}
		focusRelPaths := incrementalChanges.FocusPaths()
		effectiveFocusPaths = resolveIncrementalFocusPaths(projectRoot, focusRelPaths)
		selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, incrementalChanges.AddedOrModified))
		currentLearningConfig := cont.ConfigRepo.GetCurrentLearningConfig()
		shouldSelectRelevantFiles := currentLearningConfig.SelectRelevantFiles &&
			len(focusRelPaths) >= currentLearningConfig.SelectRelevantFilesMinCandidates
		if shouldSelectRelevantFiles {
			selectionResult, selectErr := fileanalysis.ApplyAIFileSelector(ctx, cont.Agent, fileanalysis.AISelectorOptions{
				ProjectRoot: projectRoot,
				Candidates:  focusRelPaths,
				Changes:     incrementalChanges,
				UserContext: opts.userContext,
			})
			if selectErr != nil {
				logger.Warn("AI file selector failed; falling back to all candidate files")
				logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
					"operation", "command.learn_current.select_relevant_files",
					"error", selectErr,
					"candidate_count", len(focusRelPaths),
				)
			} else if selectionResult != nil && len(selectionResult.SelectedPaths) > 0 {
				effectiveFocusPaths = resolveIncrementalFocusPaths(projectRoot, selectionResult.SelectedPaths)
				selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(selectionResult.SelectedPaths, incrementalChanges.AddedOrModified))
				incrementalChanges.ApplyAISelection(selectionResult.SelectedPaths, selectionResult.Reason)
				selectionSummary = aiFileSelectionSummary{
					Applied:        true,
					CandidateCount: len(focusRelPaths),
					SelectedCount:  len(selectionResult.SelectedPaths),
					SkippedCount:   len(selectionResult.SkippedPaths),
					Reason:         selectionResult.Reason,
				}
				logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
					"operation", "command.learn_current.select_relevant_files",
					"candidate_count", len(focusRelPaths),
					"selected_count", len(selectionResult.SelectedPaths),
					"skipped_count", len(selectionResult.SkippedPaths),
					"reason", selectionResult.Reason,
				)
			}
		} else if currentLearningConfig.SelectRelevantFiles && len(focusRelPaths) > 0 {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
				"operation", "command.learn_current.select_relevant_files",
				"candidate_count", len(focusRelPaths),
				"min_candidates", currentLearningConfig.SelectRelevantFilesMinCandidates,
				"skipped", true,
			)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentIncrementalSummary", map[string]interface{}{
			"Changed":   len(incrementalChanges.AddedOrModified),
			"Deleted":   len(incrementalChanges.Deleted),
			"Unchanged": len(incrementalChanges.Unchanged),
			"Skipped":   len(incrementalChanges.Skipped),
		}))
		if len(incrementalChanges.ExcludedGeneratedSkillDirs) > 0 {
			logger.Info(i18n.GetWithParams("LearnCurrentGeneratedSkillsExcluded", map[string]interface{}{
				"Paths": strings.Join(incrementalChanges.ExcludedGeneratedSkillDirs, ", "),
			}))
		}
		if selectionSummary.Applied {
			logger.Info(i18n.GetWithParams("LearnCurrentAIFileSelectionSummary", map[string]interface{}{
				"Candidates": selectionSummary.CandidateCount,
				"Selected":   selectionSummary.SelectedCount,
				"Skipped":    selectionSummary.SkippedCount,
			}))
			logger.Info(i18n.GetWithParams("LearnCurrentFingerprintCommitPlan", map[string]interface{}{
				"Records": len(incrementalChanges.Records),
			}))
		}
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.detect_changes",
		"duration", time.Since(detectStartedAt),
		"changed_count", len(incrementalChanges.AddedOrModified),
		"deleted_count", len(incrementalChanges.Deleted),
		"unchanged_count", len(incrementalChanges.Unchanged),
		"skipped_count", len(incrementalChanges.Skipped),
	)
	var patterns []domain.Pattern
	var businessRulesCount int
	var bestPracticesCount int
	var commonPatternsCount int

	if !incrementalChanges.HasChanges() {
		if opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentNoFileChanges"))
		}
		if err := runStep(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error { return nil }); err != nil {
			return nil, err
		}
		if err := runStep(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error { return nil }); err != nil {
			return nil, err
		}
		if err := runStep(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil }); err != nil {
			return nil, err
		}
		if opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentComplete"))
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current",
			"duration", time.Since(startedAt),
			"patterns_count", 0,
			"saved_count", 0,
			"skipped", true,
		)
		result := &learnCurrentProjectResult{
			projectName:  projectName,
			changedCount: len(incrementalChanges.AddedOrModified),
			deletedCount: len(incrementalChanges.Deleted),
			skippedCount: len(incrementalChanges.Skipped),
			skipped:      true,
			duration:     time.Since(startedAt),
			tokenContext: ctx,
		}
		if opts.showDetailedLogs {
			agent.FlushTokenUsageScope(ctx)
		}
		return result, nil
	}

	// AI 分析是 learn current 最耗时的步骤，进度行会持续刷新当前耗时
	analyzeStartedAt := time.Now()
	if err := runStep(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error {
		knownPatternsJSON, knownPatternsCount := cont.LearnerSvc.KnownPatternsSnapshot(ctx)
		analyzeResult, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(ctx, projectRoot, projectName, currentLanguage, analyzer.AnalyzeCodebaseOptions{
			FocusPaths:         effectiveFocusPaths,
			SelectedFiles:      selectedFiles,
			SelectedFilesSet:   true,
			KnownPatternsJSON:  knownPatternsJSON,
			KnownPatternsCount: knownPatternsCount,
			UseSnapshotDiffs:   true,
		})
		if err != nil {
			return err
		}
		patterns = learnedPatterns
		businessRulesCount = len(analyzeResult.BusinessRules)
		bestPracticesCount = len(analyzeResult.BestPractices)
		commonPatternsCount = len(analyzeResult.CommonPatterns)
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.analyze_codebase",
			"duration", time.Since(analyzeStartedAt),
			"error", err,
		)
		return nil, fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToAnalyzeCodebase", map[string]interface{}{"Error": err.Error()}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.analyze_codebase",
		"duration", time.Since(analyzeStartedAt),
		"patterns_count", len(patterns),
		"business_rules_count", businessRulesCount,
		"best_practices_count", bestPracticesCount,
		"common_patterns_count", commonPatternsCount,
	)

	if opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentResult", map[string]interface{}{
			"PatternsCount":       len(patterns),
			"BusinessRulesCount":  businessRulesCount,
			"BestPracticesCount":  bestPracticesCount,
			"CommonPatternsCount": commonPatternsCount,
		}))
	}

	savedCount := 0
	saveStartedAt := time.Now()
	if err := runStep(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error {
		var err error
		savedCount, err = cont.LearnerSvc.SavePatternsStrict(ctx, patterns, "learn_current")
		return err
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.save_patterns",
			"duration", time.Since(saveStartedAt),
			"patterns_count", len(patterns),
			"saved_count", savedCount,
			"error", err,
		)
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.save_patterns",
		"duration", time.Since(saveStartedAt),
		"patterns_count", len(patterns),
		"saved_count", savedCount,
	)
	if opts.showDetailedLogs && len(patterns) > 0 {
		logger.Info(i18n.GetWithParams("LearnCurrentPatternsSaved", map[string]interface{}{"Count": savedCount}))
	}
	if opts.profileMode == learnCurrentProfileAuto && savedCount > 0 {
		refreshProfile = true
	}

	profileStartedAt := time.Now()
	if refreshProfile {
		if err := runStep(i18n.Get("ProgressLearnCurrentSaveProfile"), func() error {
			projectOptions := analyzer.AnalyzeProjectOptions{}
			if existingProfile != nil && len(effectiveFocusPaths) > 0 {
				projectOptions.ExistingProfile = existingProfile
				projectOptions.FocusPaths = effectiveFocusPaths
			}
			result, err := cont.AnalyzerSvc.AnalyzeProjectFullWithOptions(ctx, projectRoot, projectName, currentLanguage, projectOptions)
			if err != nil {
				return err
			}
			profile := analyzer.NewProjectProfile(result, projectName, currentLanguage)
			return cont.ProfileRepo.Save(ctx, profile)
		}); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "command.learn_current.save_project_profile",
				"duration", time.Since(profileStartedAt),
				"error", err,
			)
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.save_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", opts.profileMode,
			"incremental_profile", existingProfile != nil && len(resolvedFocusPaths) > 0,
		)
		if opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSaved"))
		}
	} else {
		if err := runStep(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil }); err != nil {
			return nil, err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.skip_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", opts.profileMode,
		)
		if opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
		}
	}

	if err := commitIncrementalFileChanges(ctx, cont.FileTracker, incrementalChanges); err != nil {
		return nil, err
	}

	if opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentComplete"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current",
		"duration", time.Since(startedAt),
		"patterns_count", len(patterns),
		"saved_count", savedCount,
	)
	if err := commandutil.MarkLearned(ctx, cont); err != nil {
		return nil, err
	}
	if savedCount > 0 {
		if err := markLearnedSkillsDirty(ctx, cont, domain.SkillsDirtyTarget{Project: true}); err != nil {
			return nil, err
		}
	}

	result := &learnCurrentProjectResult{
		projectName:   projectName,
		changedCount:  len(incrementalChanges.AddedOrModified),
		deletedCount:  len(incrementalChanges.Deleted),
		skippedCount:  len(incrementalChanges.Skipped),
		patternsCount: len(patterns),
		savedCount:    savedCount,
		duration:      time.Since(startedAt),
		tokenContext:  ctx,
	}
	if opts.showDetailedLogs {
		agent.FlushTokenUsageScope(ctx)
	}
	return result, nil
}

func logLearnWorkspaceProjectSummary(projectName string, result *learnCurrentProjectResult) {
	if result == nil {
		return
	}
	if result.projectName != "" {
		projectName = result.projectName
	}
	if result.skipped {
		logger.Info(i18n.GetWithParams("LearnWorkspaceProjectNoFileChanges", map[string]interface{}{
			"ProjectName": projectName,
			"Duration":    result.duration.Truncate(time.Second).String(),
		}))
		return
	}
	logger.Info(i18n.GetWithParams("LearnWorkspaceProjectSummary", map[string]interface{}{
		"ProjectName": projectName,
		"Changed":     result.changedCount,
		"Deleted":     result.deletedCount,
		"Skipped":     result.skippedCount,
		"Patterns":    result.patternsCount,
		"Saved":       result.savedCount,
		"Duration":    result.duration.Truncate(time.Second).String(),
	}))
}

func resolveIncrementalFocusPaths(projectRoot string, relPaths []string) []string {
	paths := make([]string, 0, len(relPaths))
	for _, relPath := range relPaths {
		paths = append(paths, filepath.Join(projectRoot, filepath.FromSlash(relPath)))
	}
	return paths
}

func intersectPaths(paths []string, allowed []string) []string {
	allowedSet := make(map[string]bool, len(allowed))
	for _, path := range allowed {
		allowedSet[filepath.ToSlash(filepath.Clean(path))] = true
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized := filepath.ToSlash(filepath.Clean(path))
		if allowedSet[normalized] {
			out = append(out, path)
		}
	}
	return out
}

func runLearnWorkspaceCurrent(cont *container.Container, opts learnCurrentOptions) error {
	ctx := runtimecontext.WithSeedPath(context.Background(), cont.SeedPath)
	ctx = runtimecontext.WithUserContext(ctx, opts.userContext)
	startedAt := time.Now()
	workspaceConfig := cont.ConfigRepo.GetWorkspaceConfig()
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}

	projectRoot := projectConfig.RootPath
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	workspaceName := projectConfig.Name
	if workspaceName == "" {
		workspaceName = filepath.Base(projectRoot)
	}

	if err := commandutil.LockConfiguredMode(ctx, cont); err != nil {
		return err
	}

	parallelism := workspacediscovery.EffectiveParallelism(domain.ModeWorkspace, cont.ConfigRepo.GetAgentConfig().Parallelism, len(workspaceConfig.Projects))
	showChildDetails := parallelism == 1
	var consoleMu sync.Mutex
	var dirtyMu sync.Mutex
	var dirtyProjectIDs []string
	var multiTracker *progress.MultiTracker
	if !showChildDetails {
		multiTracker = progress.NewMulti(commandutil.WorkspaceProjectProgressNames(workspaceConfig.Projects))
		multiTracker.SetLabel(i18n.Get("ProgressLearnWorkspaceProjects"))
		multiTracker.SetTaskTotal(5)
	}
	logger.Info(i18n.GetWithParams("LearnWorkspaceStart", map[string]interface{}{
		"Projects":    len(workspaceConfig.Projects),
		"Parallelism": parallelism,
	}), "projects", len(workspaceConfig.Projects), "parallelism", parallelism)

	runProjects := func() error {
		return workspacediscovery.RunProjectTasks(ctx, workspaceConfig.Projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
			projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
			if err != nil {
				return err
			}
			childCont, err := commandutil.OpenWorkspaceChildContainer(ctx, projectRootPath, project, commandutil.WorkspaceChildErrorKeys{
				NotInitialized: "LearnWorkspaceChildNotInitialized",
				NotGitRepo:     "LearnWorkspaceChildNotGitRepo",
				ModeInvalid:    "LearnWorkspaceChildModeInvalid",
			})
			if err != nil {
				return err
			}
			defer childCont.Close()

			consoleMu.Lock()
			logger.Info(i18n.GetWithParams("LearnWorkspaceProjectStarted", map[string]interface{}{
				"ProjectName": project.ID,
				"LogPath":     workspaceProjectLogDir(childCont),
			}))
			consoleMu.Unlock()

			scope := project.ID
			if scope == "" {
				scope = project.Path
			}
			progressName := commandutil.WorkspaceProjectProgressName(project)
			var childLogPath string
			stepStartedAt := time.Now()
			result, err := runLearnWorkspaceChildProject(ctx, childCont, scope, showChildDetails, func(label string) {
				if multiTracker != nil {
					stepStartedAt = time.Now()
					multiTracker.Start(progressName, label)
				}
			}, func(label string) {
				if multiTracker != nil {
					multiTracker.Update(progressName, label)
				}
			}, func(label string) {
				if multiTracker != nil {
					multiTracker.CompleteStep(progressName, label)
					pauseAfterFastWorkspaceChildStep(stepStartedAt)
				}
			}, &childLogPath, opts)
			if err != nil {
				if multiTracker != nil {
					multiTracker.Fail(progressName, i18n.Get("LearnWorkspaceProjectProgressFailed"))
				}
				return err
			}
			if multiTracker != nil {
				multiTracker.Complete(progressName, i18n.GetWithParams("LearnWorkspaceProjectProgressComplete", map[string]interface{}{
					"Patterns": result.patternsCount,
					"Saved":    result.savedCount,
				}))
			}
			consoleMu.Lock()
			if !showChildDetails {
				logLearnWorkspaceProjectSummary(project.ID, result)
			}
			if result.savedCount > 0 {
				dirtyMu.Lock()
				dirtyProjectIDs = append(dirtyProjectIDs, scope)
				dirtyMu.Unlock()
			}
			agent.FlushTokenUsageScope(result.tokenContext)
			logger.Info(i18n.GetWithParams("LearnWorkspaceProjectDelegated", map[string]interface{}{
				"ProjectName": project.ID,
				"LogPath":     childLogPath,
			}))
			consoleMu.Unlock()
			return nil
		})
	}
	if err := runProjects(); err != nil {
		return err
	}

	relationshipsChanged, err := saveWorkspaceRelationshipArtifacts(ctx, cont, workspaceName, projectRoot, workspaceConfig)
	if err != nil {
		return err
	}
	dirtyTarget := domain.SkillsDirtyTarget{Workspace: relationshipsChanged, Projects: dirtyProjectIDs}
	if !skillsDirtyTargetEmpty(dirtyTarget) {
		if err := markLearnedSkillsDirty(ctx, cont, dirtyTarget); err != nil {
			return err
		}
	}

	if err := commandutil.MarkLearned(ctx, cont); err != nil {
		return err
	}

	logger.Info(i18n.Get("LearnWorkspaceComplete"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_workspace_current",
		"duration", time.Since(startedAt),
		"projects_count", len(workspaceConfig.Projects),
	)
	return nil
}

func markLearnedSkillsDirty(ctx context.Context, cont *container.Container, target domain.SkillsDirtyTarget) error {
	if cont == nil || cont.StateRepo == nil {
		return nil
	}
	return cont.StateRepo.MarkSkillsDirty(ctx, target)
}

func skillsDirtyTargetEmpty(target domain.SkillsDirtyTarget) bool {
	return !target.Project && !target.Workspace && len(target.Projects) == 0
}

func runLearnWorkspaceChildProject(ctx context.Context, childCont *container.Container, scope string, showDetails bool, onStepStart func(label string), onStepUpdate func(label string), onStepComplete func(label string), logPath *string, opts learnCurrentOptions) (*learnCurrentProjectResult, error) {
	loggingConfig := childCont.ConfigRepo.GetLoggingConfig()
	logDir := filepath.Join(childCont.SeedPath, loggingConfig.LogsPath)
	logLevel := logger.ParseLevel(loggingConfig.Level)

	var result *learnCurrentProjectResult
	err := logger.WithScopedLog(ctx, logDir, "learn", logLevel, loggingConfig.MaxLogFiles, func(scopedCtx context.Context, scopedLogPath string) error {
		if logPath != nil {
			*logPath = scopedLogPath
		}
		var err error
		result, err = runLearnCurrentProjectWithOptions(childCont, learnCurrentProjectOptions{
			tokenScope:       scope,
			showProgress:     showDetails,
			showDetailedLogs: showDetails,
			onStepStart:      onStepStart,
			onStepUpdate:     onStepUpdate,
			onStepComplete:   onStepComplete,
			userContext:      opts.userContext,
			language:         opts.language,
			focusPaths:       opts.focusPaths,
			profileMode:      opts.profileMode,
		})
		return err
	})
	return result, err
}

func workspaceProjectLogDir(cont *container.Container) string {
	loggingConfig := cont.ConfigRepo.GetLoggingConfig()
	return filepath.Join(cont.SeedPath, loggingConfig.LogsPath)
}

func pauseAfterFastWorkspaceChildStep(startedAt time.Time) {
	progress.PauseAfterFastStep(startedAt, sleepAfterWorkspaceChildStep)
}

func saveWorkspaceRelationshipArtifacts(ctx context.Context, cont *container.Container, workspaceName, projectRoot string, workspaceConfig config.WorkspaceConfig) (bool, error) {
	if cont.WorkspaceProfileRepo == nil && cont.WorkspaceSpecRepo == nil {
		return false, nil
	}
	generatedAt := time.Now().Format(time.RFC3339)
	baseProfile := workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig)
	if cont.Agent == nil {
		return true, saveWorkspaceRelationshipFallback(ctx, cont, baseProfile, generatedAt)
	}

	input, err := workspaceLearnInput(ctx, cont, workspaceName, projectRoot, workspaceConfig)
	if err != nil {
		return false, err
	}
	userContext := runtimecontext.UserContext(ctx)
	decision, err := domain.PrepareInputFingerprint(ctx, cont.FileTracker, workspaceRelationshipFingerprintScope(), "workspace-relationships.json", workspaceRelationshipFingerprintInput{
		Kind:                "workspace_relationship_learning",
		ProgramVersion:      metadata.ProgramVersion,
		PromptTemplatesHash: metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS)),
		WorkspaceInput:      input,
		UserContext:         userContext,
	})
	if err != nil {
		return false, err
	}
	if decision.ShouldSkip() && workspaceRelationshipArtifactsExist(ctx, cont) {
		logger.Info(i18n.Get("LearnWorkspaceRelationshipsSkipped"))
		return false, nil
	}
	runtimeDir := filepath.Join(projectRoot, ".skills-seed", "memory", "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return false, err
	}
	tmpDir, err := os.MkdirTemp(runtimeDir, "skills-seed-workspace-learn-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath, err := writeJSONInput(filepath.Join(tmpDir, "workspace-input.json"), input)
	if err != nil {
		return false, err
	}
	userContextPath := ""
	if userContext != "" {
		userContextPath = filepath.Join(tmpDir, "user-context.md")
		if err := os.WriteFile(userContextPath, []byte(userContext+"\n"), 0600); err != nil {
			return false, err
		}
	}

	tracker := progress.New(3)
	var profile *domain.WorkspaceProfile
	if err := tracker.RunStep(i18n.Get("ProgressLearnWorkspaceAnalyzeProfile"), func() error {
		var err error
		profile, err = cont.Agent.AnalyzeWorkspaceProfile(ctx, &agent.AnalyzeWorkspaceProfileRequest{
			WorkspaceName:      workspaceName,
			WorkspaceRoot:      projectRoot,
			WorkspaceInputPath: inputPath,
			UserContextPath:    userContextPath,
		})
		if err != nil {
			return err
		}
		profile = workspacediscovery.MergeProfile(baseProfile, profile)
		profile.GeneratedAt = generatedAt
		return nil
	}); err != nil {
		return false, err
	}

	var spec *domain.WorkspaceSpec
	if err := tracker.RunStep(i18n.Get("ProgressLearnWorkspaceAnalyzeSpec"), func() error {
		profilePath, err := writeJSONInput(filepath.Join(tmpDir, "workspace-profile.json"), profile)
		if err != nil {
			return err
		}
		spec, err = cont.Agent.AnalyzeWorkspaceSpec(ctx, &agent.AnalyzeWorkspaceSpecRequest{
			WorkspaceName:        workspaceName,
			WorkspaceRoot:        projectRoot,
			WorkspaceInputPath:   inputPath,
			WorkspaceProfilePath: profilePath,
			UserContextPath:      userContextPath,
		})
		if err != nil {
			return err
		}
		spec = workspacediscovery.MergeSpec(workspacediscovery.SpecFromProfile(profile), spec)
		spec.GeneratedAt = generatedAt
		return nil
	}); err != nil {
		return false, err
	}

	if err := tracker.RunStep(i18n.Get("ProgressLearnWorkspaceSaveArtifacts"), func() error {
		if cont.WorkspaceProfileRepo != nil {
			if err := cont.WorkspaceProfileRepo.Save(ctx, profile); err != nil {
				return err
			}
		}
		if cont.WorkspaceSpecRepo != nil {
			if err := cont.WorkspaceSpecRepo.Save(ctx, spec); err != nil {
				return err
			}
		}
		if err := decision.Commit(ctx, cont.FileTracker); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return false, err
	}
	return true, nil
}

func workspaceRelationshipFingerprintScope() domain.FileAnalysisScope {
	return domain.FileAnalysisScope{ProjectID: "__workspace__", ScopePath: "learn"}
}

func workspaceRelationshipArtifactsExist(ctx context.Context, cont *container.Container) bool {
	if cont.WorkspaceProfileRepo != nil {
		if _, err := cont.WorkspaceProfileRepo.Get(ctx); err != nil {
			return false
		}
	}
	if cont.WorkspaceSpecRepo != nil {
		if _, err := cont.WorkspaceSpecRepo.Get(ctx); err != nil {
			return false
		}
	}
	return true
}

func saveWorkspaceRelationshipFallback(ctx context.Context, cont *container.Container, profile *domain.WorkspaceProfile, generatedAt string) error {
	if profile == nil {
		profile = &domain.WorkspaceProfile{}
	}
	profile.GeneratedAt = generatedAt
	if cont.WorkspaceProfileRepo != nil {
		if err := cont.WorkspaceProfileRepo.Save(ctx, profile); err != nil {
			return err
		}
	}
	if cont.WorkspaceSpecRepo != nil {
		spec := workspacediscovery.SpecFromProfile(profile)
		spec.GeneratedAt = generatedAt
		if err := cont.WorkspaceSpecRepo.Save(ctx, spec); err != nil {
			return err
		}
	}
	return nil
}

func workspaceLearnInput(ctx context.Context, cont *container.Container, workspaceName, projectRoot string, workspaceConfig config.WorkspaceConfig) (workspaceLearnInputData, error) {
	input := workspaceLearnInputData{
		Name:       workspaceName,
		RootPath:   projectRoot,
		Projects:   make([]workspaceLearnInputProject, 0, len(workspaceConfig.Projects)),
		ConfigPath: filepath.ToSlash(filepath.Join(projectRoot, ".skills-seed", "config.yaml")),
	}
	for _, project := range workspaceConfig.Projects {
		projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
		if err != nil {
			return workspaceLearnInputData{}, err
		}
		childSeedPath := filepath.Join(projectRootPath, ".skills-seed")
		projectProfilePath := filepath.Join(childSeedPath, "memory", "project-profile.json")
		projectSpecPath := filepath.Join(childSeedPath, "memory", "project-spec.json")
		skillPath := workspaceChildSkillPath(projectRootPath, childSeedPath, cont.ConfigRepo)
		child := workspaceLearnInputProject{
			ID:                 project.ID,
			Path:               project.Path,
			Type:               project.Type,
			Language:           project.Language,
			SkillPath:          filepath.ToSlash(filepath.Join(project.Path, skillPath, "SKILL.md")),
			ProjectProfilePath: filepath.ToSlash(projectProfilePath),
			ProjectSpecPath:    filepath.ToSlash(projectSpecPath),
		}
		if profile, err := readChildProjectProfile(ctx, cont, project.ID, projectProfilePath); err == nil && profile != nil {
			child.Summary = profile.Summary
			child.Frameworks = append([]string(nil), profile.Frameworks...)
			child.KeyModules = projectProfileModuleSummaries(profile)
		}
		input.Projects = append(input.Projects, child)
	}
	return input, nil
}

func workspaceChildSkillPath(projectRootPath, childSeedPath string, rootConfig config.Reader) string {
	configRepo, err := config.NewRepository(childSeedPath, "")
	if err == nil {
		target := configRepo.GetEffectiveSkillsTarget()
		outputPath := configRepo.GetEffectiveSkillsPath()
		if outputPath == "" {
			outputPath = config.DefaultSkillsPathForTarget(target)
		}
		return filepath.ToSlash(outputPath)
	}

	target := ""
	outputPath := ""
	if rootConfig != nil {
		target = rootConfig.GetEffectiveSkillsTarget()
		outputPath = rootConfig.GetEffectiveSkillsPath()
	}
	if outputPath == "" {
		outputPath = config.DefaultSkillsPathForTarget(target)
	}
	return filepath.ToSlash(outputPath)
}

func readChildProjectProfile(ctx context.Context, cont *container.Container, projectID, profilePath string) (*domain.ProjectProfile, error) {
	if cont.ProfileRepo != nil {
		if profile, err := cont.ProfileRepo.GetForProject(ctx, projectID); err == nil {
			return profile, nil
		}
	}
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}
	var profile domain.ProjectProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func projectProfileModuleSummaries(profile *domain.ProjectProfile) []string {
	if profile == nil {
		return nil
	}
	modules := make([]string, 0, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		if module.Path == "" && module.Description == "" {
			continue
		}
		if module.Description == "" {
			modules = append(modules, module.Path)
			continue
		}
		if module.Path == "" {
			modules = append(modules, module.Description)
			continue
		}
		modules = append(modules, module.Path+": "+module.Description)
	}
	return modules
}

func writeJSONInput(path string, value interface{}) (string, error) {
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

// workspaceSpecFromConfig 基于根仓配置生成只描述跨子仓关系的工作区规范。
func workspaceSpecFromConfig(workspaceName, projectRoot string, workspaceConfig config.WorkspaceConfig, generatedAt string) *domain.WorkspaceSpec {
	profile := workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig)
	routing := make([]domain.WorkspaceRoute, 0, len(profile.Projects))
	for _, project := range profile.Projects {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(project.Path, "**")),
			ProjectIDs:  []string{project.ID},
			Reason:      "子项目路径只路由到该子项目的独立 skill",
		})
	}

	return &domain.WorkspaceSpec{
		Name:        workspaceName,
		RootPath:    projectRoot,
		Projects:    profile.Projects,
		Routing:     routing,
		Rules:       defaultWorkspaceRules(profile.Projects),
		GeneratedAt: generatedAt,
	}
}

func workspaceProjectIDs(projects []domain.WorkspaceProject) []string {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	return ids
}

func defaultWorkspaceRules(projects []domain.WorkspaceProject) []domain.WorkspaceRule {
	projectIDs := workspaceProjectIDs(projects)
	return []domain.WorkspaceRule{
		{
			Title:       "子项目独立学习",
			Description: "工作区根仓只编排子项目学习并维护跨项目关系；子项目模式、画像和文件指纹保存在各自 .skills-seed 中。",
			AppliesTo:   projectIDs,
		},
		{
			Title:       "跨项目改动先定边界",
			Description: "修改契约、共享代码或基础设施前，先确认生产者、消费者和运行时影响，再读取相关子项目 skill。",
			AppliesTo:   projectIDs,
		},
	}
}

func resolveFocusPaths(projectRoot string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	projectAbs = filepath.Clean(projectAbs)

	resolved := make([]string, 0, len(paths))
	seen := make(map[string]bool)
	for _, rawPath := range paths {
		rawPath = strings.TrimSpace(rawPath)
		if rawPath == "" {
			continue
		}

		path, err := utils.ResolvePath(projectAbs, rawPath)
		if err != nil {
			return nil, err
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		path = filepath.Clean(path)

		relPath, err := filepath.Rel(projectAbs, path)
		if err != nil {
			return nil, err
		}
		if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentFocusOutsideRoot", map[string]interface{}{"Path": rawPath}))
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.GetWithParams("LearnCurrentFocusNotAccessible", map[string]interface{}{"Path": rawPath}), err)
		}
		if seen[path] {
			continue
		}
		resolved = append(resolved, path)
		seen[path] = true
	}
	return resolved, nil
}

func shouldRefreshProfile(projectRoot string, focusPaths []string, mode string, profileExists bool) (bool, error) {
	switch mode {
	case "", learnCurrentProfileAuto:
		return !profileExists, nil
	case learnCurrentProfileSkip:
		return false, nil
	case learnCurrentProfileRefresh:
		return true, nil
	default:
		return false, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileModeInvalid", map[string]interface{}{"Mode": mode}))
	}
}

// runLearnHistory 从 Git 历史提交学习
func runLearnHistory(cont *container.Container, opts learnHistoryOptions) error {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return err
	}

	ctx := context.Background()
	startedAt := time.Now()

	logger.Info(i18n.Get("LearnHistoryStart"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "command.learn_history",
		"agent", cont.Agent.Name(),
		"limit", opts.limit,
		"since", opts.since,
		"batch_size", opts.batchSize,
	)
	logger.Info(i18n.GetWithParams("LearnHistoryInfo", map[string]interface{}{
		"Limit":     opts.limit,
		"Since":     opts.since,
		"BatchSize": opts.batchSize,
	}))

	// 调用学习服务
	if err := commandutil.LockConfiguredMode(ctx, cont); err != nil {
		return err
	}
	err := cont.LearnerSvc.Learn(ctx, opts.limit, opts.since, opts.batchSize)
	if err != nil {
		logger.Error(i18n.GetWithParams("LearnHistoryFailed", map[string]interface{}{"Error": err.Error()}))
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_history",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return err
	}

	logger.Info(i18n.Get("LearnHistoryComplete"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_history",
		"duration", time.Since(startedAt),
	)

	// 显示统计信息
	count, err := cont.PatternRepo.Count(ctx)
	if err == nil {
		logger.Info(i18n.GetWithParams("LearnHistoryTotalPatterns", map[string]interface{}{"Count": count}))
	}
	if err := commandutil.MarkLearned(ctx, cont); err != nil {
		return err
	}

	return nil
}
