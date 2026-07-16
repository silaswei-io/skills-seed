package learn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/pkg/changelog"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type currentFileSelectionPlan struct {
	Candidates []string
	Eligible   bool
	SkipReason string
}

type learnCurrentProjectRun struct {
	cont      *container.Container
	opts      learnCurrentProjectOptions
	stateRepo *commandstate.Repository
	ctx       context.Context
	startedAt time.Time
	steps     *commandutil.ConsoleStepRunner

	projectRoot        string
	projectName        string
	currentLanguage    string
	learningMode       string
	learningScope      string
	resolvedFocusPaths []string
	refreshProfile     bool
	existingProfile    *domain.ProjectProfile

	incrementalChanges  *fileanalysis.FileChanges
	effectiveFocusPaths []string
	selectedFiles       []domain.FileInfo
	selectionSummary    aiFileSelectionSummary
	selectionPlan       currentFileSelectionPlan
	stateSession        *currentStateSession
	resumeSummary       *learnCurrentResumeSummary
	changeProfile       currentChangeProfile
	analysisState       *commandstate.State
	plannedUnits        []domain.AnalysisUnit

	patterns                  []domain.Pattern
	profileRefreshRecommended agent.ProfileRefreshRecommendation
	codebaseRunContext        *analyzer.CodebaseRunContext
	savedCount                int
	completedAnalysisUnits    []domain.AnalysisUnit
	snapshotCommitMu          sync.Mutex
	progressDetailMu          sync.Mutex
}

func runLearnCurrentProjectWithOptions(cont *container.Container, opts learnCurrentProjectOptions) (*learnCurrentProjectResult, error) {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return nil, err
	}
	run := newLearnCurrentProjectRun(cont, opts)
	return run.execute()
}

func newLearnCurrentProjectRun(cont *container.Container, opts learnCurrentProjectOptions) *learnCurrentProjectRun {
	ctx := agent.WithTokenUsageScope(context.Background(), opts.tokenScope)
	ctx = runtimecontext.WithSeedPath(ctx, cont.SeedPath)
	ctx = runtimecontext.WithUserContext(ctx, opts.userContext)
	steps := commandutil.NewConsoleStepRunner(commandutil.ConsoleStepRunnerOptions{
		TotalSteps:     learnCurrentProjectStepTotal,
		ShowProgress:   opts.showProgress,
		OnStepStart:    opts.onStepStart,
		OnStepComplete: opts.onStepComplete,
		OnStepUpdate:   opts.onStepUpdate,
	})
	ctx = steps.WithContext(ctx)

	return &learnCurrentProjectRun{
		cont:      cont,
		opts:      opts,
		stateRepo: learnCurrentStateRepo(cont.SeedPath, opts.stateScope),
		ctx:       ctx,
		startedAt: time.Now(),
		steps:     steps,
	}
}

func (r *learnCurrentProjectRun) execute() (*learnCurrentProjectResult, error) {
	if r.opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentStart"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "command.learn_current",
		"agent", r.cont.Agent.Name(),
		"seed_path", r.cont.SeedPath,
	)

	if err := r.prepareProject(); err != nil {
		return nil, err
	}
	if err := r.detectChanges(); err != nil {
		return nil, err
	}
	if err := r.selectRelevantFilesWithAI(); err != nil {
		return nil, err
	}
	r.logFileSelectionSummary()
	if !r.incrementalChanges.HasChanges() {
		return r.finishWithoutChanges()
	}
	if err := r.planAnalysisUnits(); err != nil {
		return nil, err
	}
	if err := r.analyzeCodebase(); err != nil {
		return nil, err
	}
	if r.opts.profileMode == learnCurrentProfileAuto && r.profileRefreshRecommended.Needed {
		r.refreshProfile = true
	}
	if err := r.saveProfileIfNeeded(); err != nil {
		return nil, err
	}
	if err := r.savePatternsStep(); err != nil {
		return nil, err
	}

	if err := r.cont.FileTracker.DeleteAnalyzedFiles(r.ctx, r.incrementalChanges.Scope, r.incrementalChanges.Deleted); err != nil {
		return nil, err
	}

	if r.opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentComplete"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current",
		"duration", time.Since(r.startedAt),
		"patterns_count", len(r.patterns),
		"saved_count", r.savedCount,
	)
	if err := commandutil.MarkLearned(r.ctx, r.cont); err != nil {
		return nil, err
	}
	if r.analysisState != nil {
		if err := r.stateRepo.Clear(); err != nil {
			return nil, err
		}
	}

	return r.buildResult(false), nil
}

func (r *learnCurrentProjectRun) prepareProject() error {
	// 解析项目上下文可能访问 Git 和配置文件，单独作为第一步展示，避免用户以为命令无响应
	prepareStartedAt := time.Now()
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentPrepareProject"), func() error {
		var err error
		r.projectRoot, err = r.cont.GitRepo.GetProjectRoot(r.ctx)
		if err != nil {
			r.projectRoot = r.cont.ConfigRepo.GetProjectConfig().RootPath
		}
		if r.projectRoot == "" {
			r.projectRoot, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		r.projectName = filepath.Base(r.projectRoot)
		if configuredName := r.cont.ConfigRepo.GetProjectConfig().Name; configuredName != "" {
			r.projectName = configuredName
		}

		r.currentLanguage = r.opts.language
		if r.currentLanguage == "" {
			r.currentLanguage = r.cont.ConfigRepo.GetProjectConfig().Language
		}
		if r.currentLanguage == "" {
			r.currentLanguage = "unknown"
		}
		currentLearningConfig := r.cont.ConfigRepo.GetCurrentLearningConfig()
		r.learningMode = string(currentLearningConfig.Mode)
		r.learningScope = string(currentLearningConfig.Scope)

		r.resolvedFocusPaths, err = resolveFocusPaths(r.projectRoot, r.opts.focusPaths)
		if err != nil {
			return err
		}
		profileExists := false
		if r.cont.ProfileRepo != nil {
			if profile, getErr := r.cont.ProfileRepo.Get(r.ctx); getErr == nil {
				r.existingProfile = profile
				profileExists = true
			}
		}
		r.refreshProfile, err = shouldRefreshProfile(r.projectRoot, r.resolvedFocusPaths, r.opts.profileMode, profileExists)
		return err
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.prepare_project",
			"duration", time.Since(prepareStartedAt),
			"error", err,
		)
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGetCurrentDir", map[string]interface{}{"Error": err.Error()}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.prepare_project",
		"duration", time.Since(prepareStartedAt),
		"project_root", r.projectRoot,
		"project_name", r.projectName,
		"language", r.currentLanguage,
		"focus_paths", strings.Join(utils.RelativePaths(r.projectRoot, r.resolvedFocusPaths), ","),
		"profile_mode", r.opts.profileMode,
		"refresh_profile", r.refreshProfile,
	)
	if r.opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentInfo", map[string]interface{}{
			"ProjectRoot": r.projectRoot,
			"ProjectName": r.projectName,
			"Language":    r.currentLanguage,
		}))
		if len(r.resolvedFocusPaths) > 0 {
			logger.Info(i18n.GetWithParams("LearnCurrentFocusInfo", map[string]interface{}{
				"Focus":       strings.Join(utils.RelativePaths(r.projectRoot, r.resolvedFocusPaths), ", "),
				"ProfileMode": r.opts.profileMode,
			}))
		}
	}
	return nil
}

func (r *learnCurrentProjectRun) detectChanges() error {
	detectStartedAt := time.Now()
	detectLabel := i18n.Get("ProgressLearnCurrentDetectChanges")
	if r.hasRestorableCurrentState() {
		detectLabel = i18n.Get("ProgressLearnCurrentResumeState")
	}
	if err := r.steps.Run(detectLabel, func() error {
		return r.restoreOrDetectChanges(detectLabel)
	}); err != nil {
		return err
	}
	r.logDetectedChanges(detectStartedAt)
	r.changeProfile = classifyCurrentChangeProfile(r.incrementalChanges)
	r.selectionPlan = r.buildFileSelectionPlan()
	return nil
}

func (r *learnCurrentProjectRun) hasRestorableCurrentState() bool {
	state, err := r.stateRepo.Load(r.ctx)
	if err != nil {
		return false
	}
	return canResumeCurrentState(state, r.projectName, r.currentLanguage, learnCurrentStateMode(r.learningMode, r.learningScope), r.opts.userContext, r.currentStateInvocationHash())
}

func (r *learnCurrentProjectRun) restoreOrDetectChanges(detectLabel string) error {
	r.detail(detectLabel, "ProgressLearnCurrentDetectRestoreState", nil)
	session, err := restoreCurrentState(r.ctx, r.stateRepo, r.cont.FileTracker, r.projectName, r.currentLanguage, learnCurrentStateMode(r.learningMode, r.learningScope), r.opts.userContext, r.currentStateInvocationHash())
	if err != nil {
		return err
	}
	var detected *fileanalysis.FileChanges
	if session != nil {
		detected, err = r.detectCurrentChanges(false)
		if err != nil {
			return err
		}
		if !currentStateInputsMatchProject(r.projectRoot, session.State.Inputs) || !currentChangesCoveredByState(session.State, detected) {
			if err := r.stateRepo.Clear(); err != nil {
				return err
			}
			session = nil
			if r.opts.force {
				detected = nil
			}
		}
	}
	if session != nil {
		r.stateSession = session
		r.incrementalChanges = session.Changes
		focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
		r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, focusRelPaths)
		r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, r.incrementalChanges.AddedOrModified))
		r.resumeSummary = buildLearnCurrentResumeSummary(session)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.resume_state",
			"state_scope", r.stateRepo.Command(),
			"inputs_count", len(session.State.Inputs),
			"pending_count", len(r.incrementalChanges.AddedOrModified)+len(r.incrementalChanges.Deleted),
			"units_count", len(session.State.Units),
		)
		return nil
	}

	r.detail(detectLabel, "ProgressLearnCurrentDetectScanFiles", nil)
	if detected == nil {
		detected, err = r.detectCurrentChanges(r.opts.force)
		if err != nil {
			return err
		}
	}
	r.incrementalChanges = detected
	focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
	r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, focusRelPaths)
	r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, r.incrementalChanges.AddedOrModified))
	return nil
}

func (r *learnCurrentProjectRun) detectCurrentChanges(force bool) (*fileanalysis.FileChanges, error) {
	return fileanalysis.PrepareCurrentChangesWithOptions(r.ctx, r.cont.FileTracker, r.cont.ConfigRepo, r.projectRoot, r.projectRoot, domain.FileAnalysisScope{}, r.resolvedFocusPaths, fileanalysis.CurrentChangeOptions{Force: force})
}

func (r *learnCurrentProjectRun) currentStateInvocationHash() string {
	focusPaths := utils.RelativePaths(r.projectRoot, r.resolvedFocusPaths)
	return learnCurrentInvocationHash(r.cont.ConfigRepo, focusPaths, r.opts.profileMode, r.opts.force)
}

func (r *learnCurrentProjectRun) buildFileSelectionPlan() currentFileSelectionPlan {
	focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
	currentLearningConfig := r.cont.ConfigRepo.GetCurrentLearningConfig()
	if r.stateSession != nil {
		return currentFileSelectionPlan{
			Candidates: focusRelPaths,
			SkipReason: i18n.Get("ProgressLearnCurrentFileSelectionSkipRestored"),
		}
	}
	if !currentLearningConfig.SelectRelevantFiles {
		return currentFileSelectionPlan{
			Candidates: focusRelPaths,
			SkipReason: i18n.Get("ProgressLearnCurrentFileSelectionSkipDisabled"),
		}
	}
	if len(focusRelPaths) == 0 {
		return currentFileSelectionPlan{
			Candidates: focusRelPaths,
			SkipReason: i18n.Get("ProgressLearnCurrentFileSelectionSkipNoCandidates"),
		}
	}
	if len(focusRelPaths) < currentLearningConfig.SelectRelevantFilesMinCandidates {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.select_relevant_files",
			"candidate_count", len(focusRelPaths),
			"min_candidates", currentLearningConfig.SelectRelevantFilesMinCandidates,
			"skipped", true,
		)
		return currentFileSelectionPlan{
			Candidates: focusRelPaths,
			SkipReason: i18n.GetWithParams("ProgressLearnCurrentFileSelectionSkipBelowThreshold", map[string]interface{}{
				"Candidates":    len(focusRelPaths),
				"MinCandidates": currentLearningConfig.SelectRelevantFilesMinCandidates,
			}),
		}
	}
	return currentFileSelectionPlan{Candidates: focusRelPaths, Eligible: true}
}

func (r *learnCurrentProjectRun) selectRelevantFilesWithAI() error {
	if !r.selectionPlan.Eligible {
		return r.steps.Run(i18n.GetWithParams("ProgressLearnCurrentSkipAIFileSelection", map[string]interface{}{
			"Reason": r.selectionPlan.SkipReason,
		}), func() error { return nil })
	}

	selectStartedAt := time.Now()
	selectLabel := i18n.Get("ProgressLearnCurrentAIFileSelection")
	var selectionResult *fileanalysis.AISelectorResult
	var selectErr error
	if err := r.steps.Run(selectLabel, func() error {
		structuralContext := r.fileSelectionStructuralContext()
		selectionResult, selectErr = fileanalysis.ApplyAIFileSelector(r.ctx, r.cont.Agent, fileanalysis.AISelectorOptions{
			ProjectRoot:       r.projectRoot,
			Candidates:        r.selectionPlan.Candidates,
			Changes:           r.incrementalChanges,
			UserContext:       r.opts.userContext,
			StructuralContext: structuralContext,
			CachePath:         layout.New(r.cont.SeedPath).Cache("ai-file-selection", r.stateRepo.Command(), "current.json"),
			RequiredPaths:     utils.RelativePaths(r.projectRoot, r.resolvedFocusPaths),
		})
		if selectErr != nil {
			logger.Warn(i18n.Get("LearnCurrentAIFileSelectorFallback"))
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "command.learn_current.select_relevant_files",
				"error", selectErr,
				"candidate_count", len(r.selectionPlan.Candidates),
			)
		}
		return nil
	}); err != nil {
		return err
	}
	if selectErr != nil || selectionResult == nil || len(selectionResult.SelectedPaths) == 0 {
		r.selectionSummary = aiFileSelectionSummary{
			Attempted:      true,
			CandidateCount: len(r.selectionPlan.Candidates),
			Status:         i18n.Get("LearnCurrentAIFileSelectorFallback"),
		}
		return nil
	}

	r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, selectionResult.SelectedPaths)
	r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(selectionResult.SelectedPaths, r.incrementalChanges.AddedOrModified))
	r.incrementalChanges.ApplyAISelection(selectionResult.SelectedPaths, selectionResult.Reason)
	r.selectionSummary = aiFileSelectionSummary{
		Applied:        true,
		Attempted:      true,
		CandidateCount: len(r.selectionPlan.Candidates),
		SelectedCount:  len(selectionResult.SelectedPaths),
		SkippedCount:   len(selectionResult.SkippedPaths),
		Reason:         selectionResult.Reason,
		Status:         i18n.Get("LearnCurrentFileSelectionApplied"),
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.select_relevant_files",
		"duration", time.Since(selectStartedAt),
		"candidate_count", len(r.selectionPlan.Candidates),
		"selected_count", len(selectionResult.SelectedPaths),
		"skipped_count", len(selectionResult.SkippedPaths),
		"fingerprint_record_count", len(r.incrementalChanges.Records),
		"reason", selectionResult.Reason,
	)
	return nil
}

func (r *learnCurrentProjectRun) fileSelectionStructuralContext() string {
	if r.cont == nil || r.cont.AnalyzerSvc == nil {
		return ""
	}
	return r.cont.AnalyzerSvc.CollectFileSelectionContext(r.ctx, r.projectRoot, analyzer.FileSelectionContextRequest{
		ProjectName:    r.projectName,
		Language:       r.currentLanguage,
		FocusPaths:     utils.RelativePaths(r.projectRoot, r.resolvedFocusPaths),
		CandidateCount: len(r.selectionPlan.Candidates),
		UserContext:    r.opts.userContext,
	})
}

func (r *learnCurrentProjectRun) finishWithoutChanges() (*learnCurrentProjectResult, error) {
	recoveredArtifacts := r.stateSession != nil && r.stateSession.State.ArtifactsCommitted
	if r.opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentNoFileChanges"))
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentPlanUnits"), func() error { return nil }); err != nil {
		return nil, err
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error { return nil }); err != nil {
		return nil, err
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error { return nil }); err != nil {
		return nil, err
	}
	profileStartedAt := time.Now()
	profileStep := i18n.Get("ProgressLearnCurrentSkipProfile")
	if r.refreshProfile && !recoveredArtifacts {
		profileStep = i18n.Get("ProgressLearnCurrentSaveProfile")
	}
	if err := r.steps.Run(profileStep, func() error {
		if !r.refreshProfile || recoveredArtifacts {
			return nil
		}
		profile, err := analyzeProjectProfile(r.ctx, r.cont, r.projectRoot, r.projectName, r.currentLanguage, nil, nil)
		if err != nil {
			return err
		}
		return r.cont.ProfileRepo.Save(r.ctx, profile)
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.save_project_profile",
			"duration", time.Since(profileStartedAt),
			"error", err,
		)
		return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
	}
	if r.opts.showDetailedLogs {
		if r.refreshProfile && !recoveredArtifacts {
			logger.Info(i18n.Get("LearnCurrentProfileSaved"))
		} else {
			logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
		}
		logger.Info(i18n.Get("LearnCurrentComplete"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current",
		"duration", time.Since(r.startedAt),
		"patterns_count", 0,
		"saved_count", 0,
		"skipped", true,
	)
	if recoveredArtifacts {
		if err := commandutil.MarkLearned(r.ctx, r.cont); err != nil {
			return nil, err
		}
		if err := r.stateRepo.Clear(); err != nil {
			return nil, err
		}
	}
	return r.buildResult(true), nil
}

func (r *learnCurrentProjectRun) buildResult(skipped bool) *learnCurrentProjectResult {
	result := &learnCurrentProjectResult{
		projectName:   r.projectName,
		changedCount:  len(r.incrementalChanges.AddedOrModified),
		deletedCount:  len(r.incrementalChanges.Deleted),
		skippedCount:  len(r.incrementalChanges.Skipped),
		patternsCount: len(r.patterns),
		savedCount:    r.savedCount,
		skipped:       skipped,
		duration:      time.Since(r.startedAt),
		tokenContext:  r.ctx,
	}
	if r.opts.showDetailedLogs {
		agent.FlushTokenUsageScope(r.ctx)
	}
	return result
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
