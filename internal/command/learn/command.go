package learn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/pkg/changelog"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/utils"
	"github.com/spf13/cobra"
)

const (
	learnCurrentProfileAuto    = "auto"
	learnCurrentProfileSkip    = "skip"
	learnCurrentProfileRefresh = "refresh"
)

var sleepAfterWorkspaceChildStep = time.Sleep

const learnCurrentProgressSubjectMaxRunes = 36

type learnCurrentOptions struct {
	language    string
	focusPaths  []string
	profileMode string
	contextText string
	contextFile string
	userContext string
	stateScope  string
	force       bool
}

type learnHistoryOptions struct {
	limit     int
	since     string
	batchSize int
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
			result, err := runLearnCurrent(cont, currentOpts)
			if err != nil {
				return err
			}
			change := changelog.Start(cont.SeedPath, "learn current")
			recordLearnCurrentSummary(change, result)
			return change.Save(i18n.Get("ChangeLogSummaryLearnCurrent"))
		},
	}
	currentCmd.Flags().StringVarP(&currentOpts.language, "language", "l", "", i18n.Get("LearnFlagLanguage"))
	currentCmd.Flags().StringArrayVarP(&currentOpts.focusPaths, "focus", "f", nil, i18n.Get("LearnFlagFocus"))
	currentCmd.Flags().StringVar(&currentOpts.profileMode, "profile", learnCurrentProfileAuto, i18n.Get("LearnFlagProfile"))
	currentCmd.Flags().StringVar(&currentOpts.contextText, "context", "", i18n.Get("LearnFlagContext"))
	currentCmd.Flags().StringVar(&currentOpts.contextFile, "context-file", "", i18n.Get("LearnFlagContextFile"))
	currentCmd.Flags().BoolVar(&currentOpts.force, "force", false, i18n.Get("LearnFlagForce"))

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

// RunLearnCurrent 导出：从当前代码库学习，并返回学习摘要。
func RunLearnCurrent(cont *container.Container) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(cont, learnCurrentOptions{profileMode: learnCurrentProfileAuto})
}

// RunLearnCurrentWithContext 导出：从当前代码库学习，附加一次性用户上下文，并返回学习摘要。
func RunLearnCurrentWithContext(cont *container.Container, userContext string) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(cont, learnCurrentOptions{profileMode: learnCurrentProfileAuto, userContext: userContext})
}

// RunLearnCurrentWithStateScope 从当前代码库学习，并使用指定恢复状态 scope。
func RunLearnCurrentWithStateScope(cont *container.Container, stateScope string, userContext string) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(cont, learnCurrentOptions{profileMode: learnCurrentProfileAuto, userContext: userContext, stateScope: stateScope})
}

func runLearnCurrent(cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	if opts.profileMode == "" {
		opts.profileMode = learnCurrentProfileAuto
	}
	if opts.userContext == "" {
		userContext, err := commandutil.ResolveRuntimeContext(opts.contextText, opts.contextFile)
		if err != nil {
			return domain.LearnCurrentResult{}, err
		}
		opts.userContext = userContext
	}
	if cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return runLearnWorkspaceCurrent(cont, opts)
	}
	return runLearnCurrentProject(cont, opts)
}

func runLearnCurrentProject(cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	result, err := runLearnCurrentProjectWithOptions(cont, learnCurrentProjectOptions{
		showProgress:     true,
		showDetailedLogs: true,
		userContext:      opts.userContext,
		language:         opts.language,
		focusPaths:       opts.focusPaths,
		profileMode:      opts.profileMode,
		stateScope:       opts.stateScope,
		force:            opts.force,
	})
	if err != nil {
		return domain.LearnCurrentResult{}, err
	}
	summary := domain.LearnCurrentSummary{
		ChangedFiles:  result.changedCount,
		DeletedFiles:  result.deletedCount,
		SkippedFiles:  result.skippedCount,
		PatternsFound: result.patternsCount,
		PatternsSaved: result.savedCount,
		NoFileChanges: result.skipped,
	}
	return domain.LearnCurrentResult{Summary: summary}, nil
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
	stateScope       string
	force            bool
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

func learnCurrentProgressDetail(baseLabel, detailKey string, params map[string]interface{}) string {
	if params == nil {
		params = map[string]interface{}{}
	}
	params["Label"] = baseLabel
	return i18n.GetWithParams(detailKey, params)
}

func learnCurrentProgressSubject(unit domain.AnalysisUnit) string {
	subject := strings.TrimSpace(unit.Name)
	if subject == "" {
		subject = strings.TrimSpace(unit.ID)
	}
	if subject == "" {
		subject = "unit"
	}
	runes := []rune(subject)
	if len(runes) <= learnCurrentProgressSubjectMaxRunes {
		return subject
	}
	return string(runes[:learnCurrentProgressSubjectMaxRunes]) + "..."
}

func learnCurrentUnitProgress(state *commandstate.State, fallbackIndex, fallbackTotal int, unit domain.AnalysisUnit) (int, int) {
	if state == nil || len(state.Units) == 0 {
		return fallbackIndex, fallbackTotal
	}
	for index, planned := range state.Units {
		if analysisUnitSame(planned, unit) {
			return index + 1, len(state.Units)
		}
	}
	return fallbackIndex, len(state.Units)
}

func analysisUnitSame(a, b domain.AnalysisUnit) bool {
	if a.ID != "" || b.ID != "" {
		return a.ID == b.ID
	}
	return a.Name == b.Name
}

func runLearnCurrentProjectWithOptions(cont *container.Container, opts learnCurrentProjectOptions) (*learnCurrentProjectResult, error) {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return nil, err
	}
	stateRepo := learnCurrentStateRepo(cont.SeedPath, opts.stateScope)

	ctx := agent.WithTokenUsageScope(context.Background(), opts.tokenScope)
	ctx = runtimecontext.WithSeedPath(ctx, cont.SeedPath)
	ctx = runtimecontext.WithUserContext(ctx, opts.userContext)
	startedAt := time.Now()
	steps := commandutil.NewConsoleStepRunner(commandutil.ConsoleStepRunnerOptions{
		TotalSteps:     5,
		ShowProgress:   opts.showProgress,
		OnStepStart:    opts.onStepStart,
		OnStepComplete: opts.onStepComplete,
		OnStepUpdate:   opts.onStepUpdate,
	})
	ctx = steps.WithContext(ctx)
	setStepDetail := func(baseLabel, detailKey string, params map[string]interface{}) string {
		return steps.Detail(baseLabel, learnCurrentProgressDetail(baseLabel, detailKey, params))
	}

	var projectRoot string
	var projectName string
	var currentLanguage string
	var learningMode string
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
	if err := steps.Run(i18n.Get("ProgressLearnCurrentPrepareProject"), func() error {
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
		learningMode = string(cont.ConfigRepo.GetCurrentLearningConfig().Mode)

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
	var stateSession *currentStateSession
	var resumeSummary *learnCurrentResumeSummary
	detectStartedAt := time.Now()
	detectLabel := i18n.Get("ProgressLearnCurrentDetectChanges")
	if err := steps.Run(detectLabel, func() error {
		setStepDetail(detectLabel, "ProgressLearnCurrentDetectRestoreState", nil)
		session, err := restoreCurrentState(ctx, stateRepo, cont.FileTracker, projectName, currentLanguage, learningMode, opts.userContext)
		if err != nil {
			return err
		}
		if session != nil {
			stateSession = session
			incrementalChanges = session.Changes
			focusRelPaths := analysisCandidatePaths(incrementalChanges)
			effectiveFocusPaths = resolveIncrementalFocusPaths(projectRoot, focusRelPaths)
			selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, incrementalChanges.AddedOrModified))
			resumeSummary = buildLearnCurrentResumeSummary(session)
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
				"operation", "command.learn_current.resume_state",
				"state_scope", stateRepo.Command(),
				"inputs_count", len(session.State.Inputs),
				"pending_count", len(incrementalChanges.AddedOrModified)+len(incrementalChanges.Deleted),
				"units_count", len(session.State.Units),
			)
			return nil
		}
		setStepDetail(detectLabel, "ProgressLearnCurrentDetectScanFiles", nil)
		incrementalChanges, err = prepareIncrementalFileChangesWithOptions(ctx, cont.FileTracker, cont.ConfigRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, resolvedFocusPaths, fileanalysis.CurrentChangeOptions{
			Force: opts.force,
		})
		if err != nil {
			return err
		}
		focusRelPaths := analysisCandidatePaths(incrementalChanges)
		effectiveFocusPaths = resolveIncrementalFocusPaths(projectRoot, focusRelPaths)
		selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, incrementalChanges.AddedOrModified))
		currentLearningConfig := cont.ConfigRepo.GetCurrentLearningConfig()
		shouldSelectRelevantFiles := currentLearningConfig.SelectRelevantFiles &&
			len(focusRelPaths) >= currentLearningConfig.SelectRelevantFilesMinCandidates
		if shouldSelectRelevantFiles {
			setStepDetail(detectLabel, "ProgressLearnCurrentDetectSelectFiles", map[string]interface{}{
				"Candidates": len(focusRelPaths),
			})
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
		if resumeSummary != nil {
			logger.Info(i18n.GetWithParams("LearnCurrentResumeSummary", map[string]interface{}{
				"Command":   resumeSummary.Command,
				"CreatedAt": resumeSummary.CreatedAt,
				"Inputs":    resumeSummary.Inputs,
				"Pending":   resumeSummary.Pending,
				"Units":     resumeSummary.Units,
				"AISkipped": resumeSummary.AISkipped,
			}))
		} else {
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
	var profileDelta domain.ProjectProfileDelta
	var profileRefreshRecommended agent.ProfileRefreshRecommendation
	var mergedProfile *domain.ProjectProfile
	savedCount := 0

	if !incrementalChanges.HasChanges() {
		if stateSession != nil {
			_ = stateRepo.Clear()
		}
		if opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentNoFileChanges"))
		}
		if err := steps.Run(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error { return nil }); err != nil {
			return nil, err
		}
		if err := steps.Run(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error { return nil }); err != nil {
			return nil, err
		}
		profileStep := i18n.Get("ProgressLearnCurrentSkipProfile")
		if refreshProfile {
			profileStep = i18n.Get("ProgressLearnCurrentSaveProfile")
		}
		if err := steps.Run(profileStep, func() error {
			if !refreshProfile {
				return nil
			}
			profile, err := analyzeProjectProfile(ctx, cont, projectRoot, projectName, currentLanguage, nil, nil)
			if err != nil {
				return err
			}
			return cont.ProfileRepo.Save(ctx, profile)
		}); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "command.learn_current.save_project_profile",
				"duration", time.Since(detectStartedAt),
				"error", err,
			)
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
		}
		if opts.showDetailedLogs {
			if refreshProfile {
				logger.Info(i18n.Get("LearnCurrentProfileSaved"))
			} else {
				logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
			}
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
	analyzeLabel := i18n.Get("ProgressLearnCurrentAnalyzeCodebase")
	if err := steps.Run(analyzeLabel, func() error {
		focusRelPaths := analysisCandidatePaths(incrementalChanges)
		state := (*commandstate.State)(nil)
		if stateSession != nil {
			state = stateSession.State
		}
		if state == nil {
			setStepDetail(analyzeLabel, "ProgressLearnCurrentAnalyzePlanUnits", nil)
			var err error
			state, err = loadOrCreateCurrentState(ctx, stateRepo, cont.AnalyzerSvc, projectName, projectRoot, currentLanguage, learningMode, focusRelPaths, incrementalChanges, opts.userContext)
			if err != nil {
				return err
			}
		} else {
			setStepDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeResumePlan", map[string]interface{}{
				"Units": len(state.Units),
			})
		}

		plannedUnits := pendingAnalysisUnits(state, incrementalChanges)
		if len(plannedUnits) == 0 {
			return nil
		}
		if existingProfile != nil {
			copyProfile := *existingProfile
			mergedProfile = &copyProfile
		}
		completedUnits := 0
		totalUnits := len(plannedUnits)
		for unitIndex, unit := range plannedUnits {
			unitFocusRelPaths := unitFocusPaths(unit, incrementalChanges)
			if len(unitFocusRelPaths) == 0 {
				continue
			}
			currentUnit, allUnits := learnCurrentUnitProgress(state, unitIndex+1, totalUnits, unit)
			unitProgressParams := map[string]interface{}{
				"Current": currentUnit,
				"Total":   allUnits,
				"Name":    learnCurrentProgressSubject(unit),
			}
			unitProgressLabel := setStepDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeUnit", unitProgressParams)
			unitFocusAbsPaths := resolveIncrementalFocusPaths(projectRoot, unitFocusRelPaths)
			unitSelected := unitSelectedFiles(unit, selectedFiles, incrementalChanges)
			unitLabel := agent.RuntimeLabelFromAnalysisUnit(unit.ID, unit.Name)
			analyzeResult, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(ctx, projectRoot, projectName, currentLanguage, analyzer.AnalyzeCodebaseOptions{
				FocusPaths:       unitFocusAbsPaths,
				RuntimeLabel:     unitLabel,
				AnalysisUnit:     unit,
				LearningMode:     cont.ConfigRepo.GetCurrentLearningConfig().Mode,
				SelectedFiles:    unitSelected,
				SelectedFilesSet: true,
				UseSnapshotDiffs: true,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", unitProgressLabel, err)
			}
			if len(learnedPatterns) > 0 {
				saveProgressLabel := setStepDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeSaveUnitPatterns", unitProgressParams)
				saved, err := cont.LearnerSvc.SavePatternsStrictWithMetadata(ctx, learnedPatterns, "learn_current", unit)
				if err != nil {
					return fmt.Errorf("%s: %w", saveProgressLabel, err)
				}
				savedCount += saved
			}
			if !analyzeResult.ProfileDelta.IsZero() {
				profileDelta = analyzeResult.ProfileDelta
				mergedProfile = domain.ApplyProjectProfileDelta(mergedProfile, analyzeResult.ProfileDelta, projectName, currentLanguage)
			}
			if analyzeResult.ProfileRefreshRecommended.Needed {
				profileRefreshRecommended = analyzeResult.ProfileRefreshRecommended
			}
			patterns = append(patterns, learnedPatterns...)
			commitProgressLabel := setStepDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeCommitUnit", unitProgressParams)
			if err := commitUnitFileRecords(ctx, cont.FileTracker, unitCommittedRecords(unit, incrementalChanges)); err != nil {
				return fmt.Errorf("%s: %w", commitProgressLabel, err)
			}
			completedUnits++
		}
		if mergedProfile != nil {
			existingProfile = mergedProfile
		}
		if completedUnits == len(plannedUnits) {
			_ = stateRepo.Clear()
		}
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
		"profile_delta", !profileDelta.IsZero(),
		"profile_refresh_recommended", profileRefreshRecommended.Needed,
	)

	if opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentResult", map[string]interface{}{
			"PatternsCount": len(patterns),
			"ProfileDelta":  !profileDelta.IsZero(),
		}))
	}

	saveStartedAt := time.Now()
	if err := steps.Run(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error { return nil }); err != nil {
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
	if opts.profileMode == learnCurrentProfileAuto && profileRefreshRecommended.Needed && profileDelta.IsZero() {
		refreshProfile = true
	}

	profileStartedAt := time.Now()
	saveProfileFromDelta := !profileDelta.IsZero() && opts.profileMode != learnCurrentProfileRefresh
	if refreshProfile || saveProfileFromDelta {
		if err := steps.Run(i18n.Get("ProgressLearnCurrentSaveProfile"), func() error {
			if saveProfileFromDelta {
				return cont.ProfileRepo.Save(ctx, mergedProfile)
			}
			profile, err := analyzeProjectProfile(ctx, cont, projectRoot, projectName, currentLanguage, effectiveFocusPaths, existingProfile)
			if err != nil {
				return err
			}
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
			"profile_delta", saveProfileFromDelta,
			"incremental_profile", existingProfile != nil && len(resolvedFocusPaths) > 0,
		)
		if opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSaved"))
		}
	} else {
		if err := steps.Run(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil }); err != nil {
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

	if err := cont.FileTracker.DeleteAnalyzedFiles(ctx, incrementalChanges.Scope, incrementalChanges.Deleted); err != nil {
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

func analyzeProjectProfile(
	ctx context.Context,
	cont *container.Container,
	projectRoot string,
	projectName string,
	language string,
	focusPaths []string,
	existingProfile *domain.ProjectProfile,
) (*domain.ProjectProfile, error) {
	projectOptions := analyzer.AnalyzeProjectOptions{}
	if existingProfile != nil && len(focusPaths) > 0 {
		projectOptions.ExistingProfile = existingProfile
		projectOptions.FocusPaths = focusPaths
	}
	result, err := cont.AnalyzerSvc.AnalyzeProjectFullWithOptions(ctx, projectRoot, projectName, language, projectOptions)
	if err != nil {
		return nil, err
	}
	return analyzer.NewProjectProfile(result, projectName, language), nil
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

func recordLearnCurrentSummary(change *changelog.Builder, result domain.LearnCurrentResult) {
	summary := result.Summary
	if summary.Projects > 0 {
		change.Detail(i18n.GetWithParams("ChangeLogLearnWorkspaceSummary", map[string]interface{}{
			"Projects":        summary.Projects,
			"ChangedProjects": summary.ChangedProjects,
		}))
		if summary.WorkspaceChanged {
			change.Detail(i18n.Get("ChangeLogWorkspaceRelationshipsChanged"))
		}
		return
	}
	if summary.NoFileChanges {
		change.Detail(i18n.Get("ChangeLogLearnNoFileChanges"))
		return
	}
	change.Detail(i18n.GetWithParams("ChangeLogLearnProjectSummary", map[string]interface{}{
		"Changed":  summary.ChangedFiles,
		"Deleted":  summary.DeletedFiles,
		"Skipped":  summary.SkippedFiles,
		"Patterns": summary.PatternsFound,
		"Saved":    summary.PatternsSaved,
	}))
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
