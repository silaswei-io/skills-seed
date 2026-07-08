package learn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	"github.com/spf13/cobra"
)

const (
	learnCurrentProfileAuto    = "auto"
	learnCurrentProfileSkip    = "skip"
	learnCurrentProfileRefresh = "refresh"
)

var sleepAfterWorkspaceChildStep = time.Sleep

const (
	learnCurrentProgressSubjectMaxRunes = 36
	learnCurrentRunningSubjectMaxRunes  = 18
	// learnCurrentProjectStepTotal 是项目级 learn current 在控制台展示的顶层阶段数。
	learnCurrentProjectStepTotal = 9
)

type learnCurrentOptions struct {
	language    string
	focusPaths  []string
	profileMode string
	contextText string
	contextPath []string
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
	currentCmd.Flags().StringArrayVar(&currentOpts.contextPath, "context-path", nil, i18n.Get("LearnFlagContextPath"))
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

// CurrentRunOptions 描述外部命令调用 learn current 时允许覆盖的执行选项。
type CurrentRunOptions struct {
	// Force 表示忽略已保存的文件指纹，重新学习当前扫描范围。
	Force bool
}

// RunLearnCurrentWithStateScopeOptions 从当前代码库学习，并允许调用方指定运行选项。
func RunLearnCurrentWithStateScopeOptions(cont *container.Container, stateScope string, userContext string, opts CurrentRunOptions) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(cont, learnCurrentOptions{
		profileMode: learnCurrentProfileAuto,
		userContext: userContext,
		stateScope:  stateScope,
		force:       opts.Force,
	})
}

func runLearnCurrent(cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	if opts.profileMode == "" {
		opts.profileMode = learnCurrentProfileAuto
	}
	if opts.userContext == "" {
		userContext, err := commandutil.ResolveRuntimeContext(opts.contextText, opts.contextPath...)
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

type learnCurrentUnitResult struct {
	index            int
	unit             domain.AnalysisUnit
	patterns         []domain.Pattern
	savedCount       int
	profileDelta     domain.ProjectProfileDelta
	refreshRecommend agent.ProfileRefreshRecommendation
	completed        bool
}

type learnCurrentBatch struct {
	index int
	units []indexedAnalysisUnit
}

type indexedAnalysisUnit struct {
	index int
	unit  domain.AnalysisUnit
}

type learnCurrentRunningUnits struct {
	mu        sync.Mutex
	labels    map[int]string
	order     []int
	completed int
	total     int
}

func newLearnCurrentRunningUnits(state *commandstate.State, plannedUnits []domain.AnalysisUnit) *learnCurrentRunningUnits {
	completed, total := learnCurrentPendingUnitProgress(state, plannedUnits)
	return &learnCurrentRunningUnits{
		labels:    make(map[int]string),
		completed: completed,
		total:     total,
	}
}

func (r *learnCurrentRunningUnits) start(index int, label string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.labels[index]; !exists {
		r.order = append(r.order, index)
		sort.Ints(r.order)
	}
	r.labels[index] = label
	return r.runningTextLocked()
}

func (r *learnCurrentRunningUnits) finish(index int, completed bool) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.labels, index)
	if completed {
		r.completed++
	}
	out := r.order[:0]
	for _, candidate := range r.order {
		if _, exists := r.labels[candidate]; exists {
			out = append(out, candidate)
		}
	}
	r.order = out
	return r.runningTextLocked()
}

func (r *learnCurrentRunningUnits) progressParams(parallelism int) map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.completed + 1
	if current > r.total {
		current = r.total
	}
	if current < 1 {
		current = 1
	}
	return map[string]interface{}{
		"Current":      current,
		"Total":        r.total,
		"Parallelism":  parallelism,
		"Running":      r.runningTextLocked(),
		"RunningCount": len(r.order),
	}
}

func (r *learnCurrentRunningUnits) runningTextLocked() string {
	if len(r.order) == 0 {
		return "-"
	}
	names := make([]string, 0, 2)
	for _, index := range r.order {
		if label := r.labels[index]; label != "" {
			names = append(names, shortenRunes(label, learnCurrentRunningSubjectMaxRunes))
		}
		if len(names) >= 2 {
			break
		}
	}
	if len(names) == 0 {
		return "-"
	}
	if extra := len(r.order) - len(names); extra > 0 {
		names = append(names, fmt.Sprintf("+%d", extra))
	}
	return strings.Join(names, ", ")
}

type aiFileSelectionSummary struct {
	Applied        bool
	CandidateCount int
	SelectedCount  int
	SkippedCount   int
	Reason         string
}

func currentStateInputSummary(changes *incrementalFileChanges, selectionPlan currentFileSelectionPlan, selectionSummary aiFileSelectionSummary) commandstate.InputSummary {
	localPlanInputs := len(selectionPlan.Candidates)
	aiSelectionInputs := 0
	aiSelectedFiles := 0
	aiSkippedFiles := 0
	if selectionSummary.Applied {
		aiSelectionInputs = selectionSummary.CandidateCount
		aiSelectedFiles = selectionSummary.SelectedCount
		aiSkippedFiles = selectionSummary.SkippedCount
	}
	sourceFiles := 0
	if changes != nil {
		sourceFiles = changes.SourceFileCount
	}
	return commandstate.InputSummary{
		SourceFiles:         sourceFiles,
		LocalPlanInputFiles: localPlanInputs,
		SelectionInputFiles: aiSelectionInputs,
		SelectedFiles:       aiSelectedFiles,
		SkippedFiles:        aiSkippedFiles,
	}
}

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

	incrementalChanges  *incrementalFileChanges
	effectiveFocusPaths []string
	selectedFiles       []domain.FileInfo
	selectionSummary    aiFileSelectionSummary
	selectionPlan       currentFileSelectionPlan
	structuralContext   string
	structuralStats     analyzer.StructuralSelectionStats
	stateSession        *currentStateSession
	resumeSummary       *learnCurrentResumeSummary
	changeProfile       currentChangeProfile
	analysisState       *commandstate.State
	plannedUnits        []domain.AnalysisUnit

	patterns                  []domain.Pattern
	profileDelta              domain.ProjectProfileDelta
	hasProfileDelta           bool
	profileRefreshRecommended agent.ProfileRefreshRecommendation
	mergedProfile             *domain.ProjectProfile
	codebaseRunContext        *analyzer.CodebaseRunContext
	savedCount                int
	patternSaveMu             sync.Mutex
	snapshotCommitMu          sync.Mutex
	progressDetailMu          sync.Mutex
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
	return shortenRunes(subject, learnCurrentProgressSubjectMaxRunes)
}

func shortenRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
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

func learnCurrentPendingUnitProgress(state *commandstate.State, plannedUnits []domain.AnalysisUnit) (int, int) {
	total := len(plannedUnits)
	if state != nil && len(state.Units) > 0 {
		total = len(state.Units)
	}
	completed := total - len(plannedUnits)
	if completed < 0 {
		completed = 0
	}
	return completed, total
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
	if err := r.buildFileSelectionStructuralContext(); err != nil {
		return nil, err
	}
	if err := r.confirmFileSelectionCandidates(); err != nil {
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
	if err := r.savePatternsStep(); err != nil {
		return nil, err
	}
	if r.opts.profileMode == learnCurrentProfileAuto && r.profileRefreshRecommended.Needed {
		r.refreshProfile = true
	}
	if err := r.saveProfileIfNeeded(); err != nil {
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

	return r.buildResult(false), nil
}

func (r *learnCurrentProjectRun) detail(baseLabel, detailKey string, params map[string]interface{}) string {
	r.progressDetailMu.Lock()
	defer r.progressDetailMu.Unlock()
	return r.steps.Detail(baseLabel, learnCurrentProgressDetail(baseLabel, detailKey, params))
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
	return canResumeCurrentState(state, r.projectName, r.currentLanguage, learnCurrentStateMode(r.learningMode, r.learningScope), r.opts.userContext)
}

func (r *learnCurrentProjectRun) restoreOrDetectChanges(detectLabel string) error {
	r.detail(detectLabel, "ProgressLearnCurrentDetectRestoreState", nil)
	session, err := restoreCurrentState(r.ctx, r.stateRepo, r.cont.FileTracker, r.projectName, r.currentLanguage, learnCurrentStateMode(r.learningMode, r.learningScope), r.opts.userContext)
	if err != nil {
		return err
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
	r.incrementalChanges, err = prepareIncrementalFileChangesWithOptions(r.ctx, r.cont.FileTracker, r.cont.ConfigRepo, r.projectRoot, r.projectRoot, domain.FileAnalysisScope{}, r.resolvedFocusPaths, fileanalysis.CurrentChangeOptions{
		Force: r.opts.force,
	})
	if err != nil {
		return err
	}
	focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
	r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, focusRelPaths)
	r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, r.incrementalChanges.AddedOrModified))
	return nil
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

func (r *learnCurrentProjectRun) buildFileSelectionStructuralContext() error {
	if !r.selectionPlan.Eligible {
		return r.steps.Run(i18n.GetWithParams("ProgressLearnCurrentSkipFileSelectionIndex", map[string]interface{}{
			"Reason": r.selectionPlan.SkipReason,
		}), func() error { return nil })
	}

	indexStartedAt := time.Now()
	indexLabel := i18n.Get("ProgressLearnCurrentBuildFileSelectionIndex")
	if err := r.steps.Run(indexLabel, func() error {
		context, err := r.cont.AnalyzerSvc.BuildFileSelectionStructuralContext(r.ctx, r.projectRoot, r.currentLanguage, r.incrementalChanges)
		if err != nil {
			return err
		}
		if context != nil {
			r.structuralContext = context.Text
			r.structuralStats = context.Stats
		}
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.build_file_selection_index",
			"duration", time.Since(indexStartedAt),
			"candidate_count", len(r.selectionPlan.Candidates),
			"error", err,
		)
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.build_file_selection_index",
		"duration", time.Since(indexStartedAt),
		"candidate_count", len(r.selectionPlan.Candidates),
		"indexed_count", r.structuralStats.IndexedFiles,
	)
	return nil
}

func (r *learnCurrentProjectRun) confirmFileSelectionCandidates() error {
	if !r.selectionPlan.Eligible {
		return r.steps.Run(i18n.GetWithParams("ProgressLearnCurrentSkipFileSelectionConfirm", map[string]interface{}{
			"Reason": r.selectionPlan.SkipReason,
		}), func() error { return nil })
	}
	return r.steps.Run(i18n.Get("ProgressLearnCurrentConfirmFileSelectionCandidates"), func() error {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.confirm_file_selection_candidates",
			"candidate_count", len(r.selectionPlan.Candidates),
			"indexed_count", r.structuralStats.IndexedFiles,
			"high_value_count", r.structuralStats.HighValueCandidates,
			"low_value_summarized_count", r.structuralStats.LowValueSummarized,
		)
		return nil
	})
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
		selectionResult, selectErr = fileanalysis.ApplyAIFileSelector(r.ctx, r.cont.Agent, fileanalysis.AISelectorOptions{
			ProjectRoot:       r.projectRoot,
			Candidates:        r.selectionPlan.Candidates,
			Changes:           r.incrementalChanges,
			StructuralContext: r.structuralContext,
			UserContext:       r.opts.userContext,
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
		return nil
	}

	r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, selectionResult.SelectedPaths)
	r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(selectionResult.SelectedPaths, r.incrementalChanges.AddedOrModified))
	r.incrementalChanges.ApplyAISelection(selectionResult.SelectedPaths, selectionResult.Reason)
	r.selectionSummary = aiFileSelectionSummary{
		Applied:        true,
		CandidateCount: len(r.selectionPlan.Candidates),
		SelectedCount:  len(selectionResult.SelectedPaths),
		SkippedCount:   len(selectionResult.SkippedPaths),
		Reason:         selectionResult.Reason,
	}
	if r.opts.showDetailedLogs {
		logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentFingerprintCommitPlan", map[string]interface{}{
			"Records": len(r.incrementalChanges.Records),
		}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.select_relevant_files",
		"duration", time.Since(selectStartedAt),
		"candidate_count", len(r.selectionPlan.Candidates),
		"selected_count", len(selectionResult.SelectedPaths),
		"skipped_count", len(selectionResult.SkippedPaths),
		"reason", selectionResult.Reason,
	)
	return nil
}

func (r *learnCurrentProjectRun) logFileSelectionSummary() {
	if !r.opts.showDetailedLogs {
		return
	}
	if r.resumeSummary != nil {
		return
	}
	aiInput := "-"
	aiSelected := "-"
	if r.selectionSummary.Applied {
		aiInput = strconv.Itoa(r.selectionSummary.CandidateCount)
		aiSelected = strconv.Itoa(r.selectionSummary.SelectedCount)
	}
	logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentFileSelectionSummary", map[string]interface{}{
		"SourceFiles":       r.incrementalChanges.SourceFileCount,
		"LocalPlanInputs":   len(r.selectionPlan.Candidates),
		"GotreeIndexed":     r.structuralStats.IndexedFiles,
		"GotreeHighValue":   r.structuralStats.HighValueCandidates,
		"AISelectionInputs": aiInput,
		"AISelectedFiles":   aiSelected,
	}))
}

func (r *learnCurrentProjectRun) logDetectedChanges(startedAt time.Time) {
	if r.opts.showDetailedLogs {
		if r.resumeSummary != nil {
			logger.Info(i18n.GetWithParams("LearnCurrentResumeSummary", map[string]interface{}{
				"Command":             r.resumeSummary.Command,
				"CreatedAt":           r.resumeSummary.CreatedAt,
				"SourceFiles":         r.resumeSummary.SourceFiles,
				"LocalPlanInputs":     r.resumeSummary.LocalPlanInputs,
				"AISelectionInputs":   r.resumeSummary.AISelectionInputs,
				"AISelectedFiles":     r.resumeSummary.AISelectedFiles,
				"PendingAnalyzeFiles": r.resumeSummary.PendingAnalyzeFiles,
				"Units":               r.resumeSummary.Units,
			}))
		} else {
			logger.Info(i18n.GetWithParams("LearnCurrentIncrementalSummary", map[string]interface{}{
				"Changed":   len(r.incrementalChanges.AddedOrModified),
				"Deleted":   len(r.incrementalChanges.Deleted),
				"Unchanged": len(r.incrementalChanges.Unchanged),
				"Skipped":   len(r.incrementalChanges.Skipped),
			}))
			if len(r.incrementalChanges.ExcludedGeneratedSkillDirs) > 0 {
				logger.Info(i18n.GetWithParams("LearnCurrentGeneratedSkillsExcluded", map[string]interface{}{
					"Paths": strings.Join(r.incrementalChanges.ExcludedGeneratedSkillDirs, ", "),
				}))
			}
		}
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.detect_changes",
		"duration", time.Since(startedAt),
		"changed_count", len(r.incrementalChanges.AddedOrModified),
		"deleted_count", len(r.incrementalChanges.Deleted),
		"unchanged_count", len(r.incrementalChanges.Unchanged),
		"skipped_count", len(r.incrementalChanges.Skipped),
	)
}

func (r *learnCurrentProjectRun) finishWithoutChanges() (*learnCurrentProjectResult, error) {
	if r.stateSession != nil {
		_ = r.stateRepo.Clear()
	}
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
	if r.refreshProfile {
		profileStep = i18n.Get("ProgressLearnCurrentSaveProfile")
	}
	if err := r.steps.Run(profileStep, func() error {
		if !r.refreshProfile {
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
		if r.refreshProfile {
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
	return r.buildResult(true), nil
}

func (r *learnCurrentProjectRun) planAnalysisUnits() error {
	planStartedAt := time.Now()
	planLabel := i18n.Get("ProgressLearnCurrentPlanUnits")
	if r.stateSession != nil {
		planLabel = i18n.GetWithParams("ProgressLearnCurrentPlanUnitsRestored", map[string]interface{}{
			"Units": len(r.stateSession.State.Units),
		})
	}
	if err := r.steps.Run(planLabel, func() error {
		focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
		state := (*commandstate.State)(nil)
		if r.stateSession != nil {
			state = r.stateSession.State
		}
		if state == nil {
			var err error
			state, err = loadOrCreateCurrentState(r.ctx, r.stateRepo, r.cont.AnalyzerSvc, r.projectName, r.projectRoot, r.currentLanguage, r.learningMode, r.learningScope, focusRelPaths, r.incrementalChanges, currentStateInputSummary(r.incrementalChanges, r.selectionPlan, r.selectionSummary), r.opts.userContext)
			if err != nil {
				return err
			}
		}
		r.analysisState = state
		r.plannedUnits = pendingAnalysisUnits(state, r.incrementalChanges)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.plan_analysis_units",
			"duration", time.Since(planStartedAt),
			"units_count", len(state.Units),
			"pending_units_count", len(r.plannedUnits),
			"candidate_count", len(focusRelPaths),
		)
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.plan_analysis_units",
			"duration", time.Since(planStartedAt),
			"error", err,
		)
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToAnalyzeCodebase", map[string]interface{}{"Error": err.Error()}))
	}
	return nil
}

func (r *learnCurrentProjectRun) analyzeCodebase() error {
	// AI 分析是 learn current 最耗时的步骤，进度行会持续刷新当前耗时
	analyzeStartedAt := time.Now()
	analyzeLabel := i18n.Get("ProgressLearnCurrentAnalyzeCodebase")
	if err := r.steps.Run(analyzeLabel, func() error {
		if r.analysisState == nil {
			return nil
		}
		if len(r.plannedUnits) == 0 {
			return nil
		}
		if r.existingProfile != nil {
			copyProfile := *r.existingProfile
			r.mergedProfile = &copyProfile
		}
		runContext, err := r.buildCodebaseRunContext()
		if err != nil {
			return err
		}
		r.codebaseRunContext = runContext
		completedUnits, err := r.analyzePlannedUnits(analyzeLabel, r.analysisState, r.plannedUnits)
		if err != nil {
			return err
		}
		if r.mergedProfile != nil {
			r.existingProfile = r.mergedProfile
		}
		if completedUnits == len(r.plannedUnits) {
			_ = r.stateRepo.Clear()
		}
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.analyze_codebase",
			"duration", time.Since(analyzeStartedAt),
			"error", err,
		)
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToAnalyzeCodebase", map[string]interface{}{"Error": err.Error()}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.analyze_codebase",
		"duration", time.Since(analyzeStartedAt),
		"patterns_count", len(r.patterns),
		"profile_delta", !r.profileDelta.IsZero(),
		"profile_refresh_recommended", r.profileRefreshRecommended.Needed,
	)

	if r.opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentResult", map[string]interface{}{
			"PatternsCount": len(r.patterns),
			"ProfileDelta":  !r.profileDelta.IsZero(),
		}))
	}
	return nil
}

func (r *learnCurrentProjectRun) analyzePlannedUnits(analyzeLabel string, state *commandstate.State, plannedUnits []domain.AnalysisUnit) (int, error) {
	batches := r.planAnalysisBatches(plannedUnits)
	parallelism := r.effectiveUnitParallelism(len(batches))
	var (
		completedUnits int
		err            error
	)
	if parallelism <= 1 {
		completedUnits, err = r.analyzePlannedBatchesSerial(analyzeLabel, state, batches)
	} else {
		completedUnits, err = r.analyzePlannedBatchesParallel(analyzeLabel, state, plannedUnits, batches, parallelism)
	}
	if err != nil {
		return completedUnits, err
	}
	logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentAnalyzeUnitsSummary", map[string]interface{}{
		"Completed":   completedUnits,
		"Total":       len(plannedUnits),
		"Batches":     len(batches),
		"Parallelism": parallelism,
	}))
	return completedUnits, nil
}

func (r *learnCurrentProjectRun) effectiveUnitParallelism(unitCount int) int {
	if unitCount <= 1 {
		return 1
	}
	parallelism := r.cont.ConfigRepo.GetCurrentLearningConfig().Parallelism
	if parallelism <= 1 {
		return 1
	}
	if parallelism > unitCount {
		return unitCount
	}
	return parallelism
}

func (r *learnCurrentProjectRun) buildCodebaseRunContext() (*analyzer.CodebaseRunContext, error) {
	return r.cont.AnalyzerSvc.BuildCodebaseRunContext(r.ctx, r.projectRoot, r.currentLanguage, analyzer.AnalyzeCodebaseOptions{
		FocusPaths:       r.effectiveFocusPaths,
		SelectedFiles:    r.selectedFiles,
		SelectedFilesSet: true,
		UseSnapshotDiffs: true,
	})
}

func (r *learnCurrentProjectRun) planAnalysisBatches(plannedUnits []domain.AnalysisUnit) []learnCurrentBatch {
	maxUnits := r.maxUnitsPerBatch()
	if maxUnits < 1 {
		maxUnits = 1
	}
	batches := make([]learnCurrentBatch, 0, (len(plannedUnits)+maxUnits-1)/maxUnits)
	for start := 0; start < len(plannedUnits); start += maxUnits {
		end := start + maxUnits
		if end > len(plannedUnits) {
			end = len(plannedUnits)
		}
		batch := learnCurrentBatch{index: len(batches)}
		for i := start; i < end; i++ {
			batch.units = append(batch.units, indexedAnalysisUnit{index: i, unit: plannedUnits[i]})
		}
		batches = append(batches, batch)
	}
	return batches
}

func (r *learnCurrentProjectRun) maxUnitsPerBatch() int {
	maxUnits := r.cont.ConfigRepo.GetCurrentLearningConfig().MaxUnitsPerCall
	if maxUnits < 1 {
		return 1
	}
	return maxUnits
}

func (r *learnCurrentProjectRun) analyzePlannedBatchesSerial(analyzeLabel string, state *commandstate.State, batches []learnCurrentBatch) (int, error) {
	completedUnits := 0
	for _, batch := range batches {
		results, err := r.analyzeBatch(r.ctx, analyzeLabel, state, batch, true)
		if err != nil {
			return completedUnits, err
		}
		completedUnits += r.mergeUnitResults(results)
	}
	return completedUnits, nil
}

func (r *learnCurrentProjectRun) analyzePlannedBatchesParallel(analyzeLabel string, state *commandstate.State, plannedUnits []domain.AnalysisUnit, batches []learnCurrentBatch, parallelism int) (int, error) {
	running := newLearnCurrentRunningUnits(state, plannedUnits)
	if r.opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism)))
	}
	r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism))

	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	jobs := make(chan learnCurrentBatch)
	results := make(chan []learnCurrentUnitResult, len(batches))
	errs := make(chan error, len(batches))
	var wg sync.WaitGroup

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				for _, job := range batch.units {
					running.start(job.index, learnCurrentProgressSubject(job.unit))
				}
				r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism))
				batchResults, err := r.analyzeBatch(ctx, analyzeLabel, state, batch, false)
				if err != nil {
					errs <- err
					cancel()
					return
				}
				for _, result := range batchResults {
					running.finish(result.index, result.completed)
				}
				r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism))
				results <- batchResults
			}
		}()
	}

	for _, batch := range batches {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			close(results)
			close(errs)
			return r.collectParallelUnitResults(results, errs)
		case jobs <- batch:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	close(errs)
	return r.collectParallelUnitResults(results, errs)
}

func (r *learnCurrentProjectRun) collectParallelUnitResults(results <-chan []learnCurrentUnitResult, errs <-chan error) (int, error) {
	for err := range errs {
		if err != nil {
			return 0, err
		}
	}
	completedUnits := 0
	collected := make([]learnCurrentUnitResult, 0)
	for batchResults := range results {
		for _, result := range batchResults {
			collected = append(collected, result)
			if result.completed {
				completedUnits++
			}
		}
	}
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].index < collected[j].index
	})
	for _, result := range collected {
		r.mergeUnitResult(result)
	}
	return completedUnits, nil
}

func (r *learnCurrentProjectRun) unitProgressParams(state *commandstate.State, unit domain.AnalysisUnit, current, total int) map[string]interface{} {
	currentUnit, allUnits := learnCurrentUnitProgress(state, current, total, unit)
	return map[string]interface{}{
		"Current": currentUnit,
		"Total":   allUnits,
		"Name":    learnCurrentProgressSubject(unit),
	}
}

func (r *learnCurrentProjectRun) analyzeBatch(ctx context.Context, analyzeLabel string, state *commandstate.State, batch learnCurrentBatch, showDetails bool) ([]learnCurrentUnitResult, error) {
	var batchUnits []analyzer.AnalyzeCurrentCodebaseBatchUnit
	results := make([]learnCurrentUnitResult, 0, len(batch.units))
	pendingByID := make(map[string]indexedAnalysisUnit, len(batch.units))
	pendingByName := make(map[string]indexedAnalysisUnit, len(batch.units))
	progressLabelByID := make(map[string]string, len(batch.units))
	for _, indexed := range batch.units {
		unitFocusRelPaths := unitFocusPaths(indexed.unit, r.incrementalChanges)
		if len(unitFocusRelPaths) == 0 {
			results = append(results, learnCurrentUnitResult{index: indexed.index, unit: indexed.unit})
			continue
		}
		params := r.batchUnitProgressParams(state, indexed)
		progressLabel := learnCurrentProgressDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeUnit", params)
		if showDetails {
			progressLabel = r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeUnit", params)
		}
		batchUnits = append(batchUnits, analyzer.AnalyzeCurrentCodebaseBatchUnit{
			AnalysisUnit:  indexed.unit,
			FocusAbsPaths: resolveIncrementalFocusPaths(r.projectRoot, unitFocusRelPaths),
		})
		pendingByID[indexed.unit.ID] = indexed
		pendingByName[indexed.unit.Name] = indexed
		progressLabelByID[indexed.unit.ID] = progressLabel
	}
	if len(batchUnits) == 0 {
		return results, nil
	}

	batchLabel := fmt.Sprintf("batch-%03d", batch.index+1)
	analyzeResult, err := r.cont.AnalyzerSvc.AnalyzeCurrentCodebaseBatch(ctx, r.projectRoot, r.projectName, r.currentLanguage, analyzer.AnalyzeCurrentCodebaseBatchOptions{
		RuntimeLabel:   batchLabel,
		LearningMode:   r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
		ChangeProfile:  string(r.changeProfile),
		LearningBudget: r.cont.ConfigRepo.GetCurrentLearningConfig().Budget,
		RunContext:     r.codebaseRunContext,
		Units:          batchUnits,
	})
	if err != nil {
		if len(batchUnits) == 1 {
			unitID := batchUnits[0].AnalysisUnit.ID
			if progressLabel := progressLabelByID[unitID]; progressLabel != "" {
				return nil, fmt.Errorf("%s: %w", progressLabel, err)
			}
		}
		return nil, err
	}

	seen := make(map[string]bool, len(analyzeResult.Units))
	for _, unitResult := range analyzeResult.Units {
		indexed, ok := pendingByID[unitResult.AnalysisUnit.ID]
		if !ok && unitResult.AnalysisUnit.Name != "" {
			indexed, ok = pendingByName[unitResult.AnalysisUnit.Name]
		}
		if !ok {
			return nil, fmt.Errorf("batch returned unknown analysis unit %q", unitResult.AnalysisUnit.ID)
		}
		params := r.batchUnitProgressParams(state, indexed)
		result, err := r.saveAnalyzedUnit(ctx, analyzeLabel, indexed.unit, indexed.index, params, unitResult.Patterns, unitResult.ProfileDelta, unitResult.ProfileRefreshRecommended, showDetails)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
		seen[indexed.unit.ID] = true
	}
	for _, indexed := range batch.units {
		unitFocusRelPaths := unitFocusPaths(indexed.unit, r.incrementalChanges)
		if len(unitFocusRelPaths) == 0 {
			continue
		}
		if !seen[indexed.unit.ID] {
			return nil, fmt.Errorf("batch missed analysis unit %q", indexed.unit.ID)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
	return results, nil
}

func (r *learnCurrentProjectRun) batchUnitProgressParams(state *commandstate.State, indexed indexedAnalysisUnit) map[string]interface{} {
	total := indexed.index + 1
	if state != nil && len(state.Units) > 0 {
		total = len(state.Units)
	}
	return r.unitProgressParams(state, indexed.unit, indexed.index+1, total)
}

func (r *learnCurrentProjectRun) saveAnalyzedUnit(ctx context.Context, analyzeLabel string, unit domain.AnalysisUnit, index int, params map[string]interface{}, learnedPatterns []domain.Pattern, profileDelta domain.ProjectProfileDelta, refreshRecommend agent.ProfileRefreshRecommendation, showDetails bool) (learnCurrentUnitResult, error) {
	saved := 0
	if len(learnedPatterns) > 0 {
		saveProgressLabel := learnCurrentProgressDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeSaveUnitPatterns", params)
		if showDetails {
			saveProgressLabel = r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeSaveUnitPatterns", params)
		}
		r.patternSaveMu.Lock()
		var err error
		learnedPatterns = r.admitLearnedPatterns(learnedPatterns)
		if len(learnedPatterns) > 0 {
			saved, err = r.cont.LearnerSvc.SavePatternsStrictWithMetadata(ctx, learnedPatterns, "learn_current", unit)
		}
		r.patternSaveMu.Unlock()
		if err != nil {
			return learnCurrentUnitResult{}, fmt.Errorf("%s: %w", saveProgressLabel, err)
		}
	}

	commitProgressLabel := learnCurrentProgressDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeCommitUnit", params)
	if showDetails {
		commitProgressLabel = r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeCommitUnit", params)
	}
	if err := r.commitUnitSuccess(ctx, unit); err != nil {
		return learnCurrentUnitResult{}, fmt.Errorf("%s: %w", commitProgressLabel, err)
	}

	result := learnCurrentUnitResult{
		index:            index,
		unit:             unit,
		patterns:         learnedPatterns,
		savedCount:       saved,
		refreshRecommend: refreshRecommend,
		completed:        true,
	}
	if !profileDelta.IsZero() {
		result.profileDelta = profileDelta
	}
	return result, nil
}

func (r *learnCurrentProjectRun) mergeUnitResults(results []learnCurrentUnitResult) int {
	completed := 0
	for _, result := range results {
		if result.completed {
			completed++
		}
		r.mergeUnitResult(result)
	}
	return completed
}

func (r *learnCurrentProjectRun) commitUnitSuccess(ctx context.Context, unit domain.AnalysisUnit) error {
	if err := commitUnitFileRecords(ctx, r.cont.FileTracker, unitCommittedRecords(unit, r.incrementalChanges)); err != nil {
		return err
	}
	if r.codebaseRunContext == nil || r.codebaseRunContext.SnapshotFlow == nil {
		return nil
	}
	r.snapshotCommitMu.Lock()
	defer r.snapshotCommitMu.Unlock()
	return r.codebaseRunContext.SnapshotFlow.CommitScoped(unitFocusPaths(unit, r.incrementalChanges))
}

func (r *learnCurrentProjectRun) mergeUnitResult(result learnCurrentUnitResult) {
	if len(result.patterns) > 0 {
		r.patterns = append(r.patterns, result.patterns...)
	}
	r.savedCount += result.savedCount
	if result.profileDelta.HasMergeableFacts() {
		r.profileDelta = result.profileDelta
		r.hasProfileDelta = true
		r.mergedProfile = domain.ApplyProjectProfileDelta(r.mergedProfile, result.profileDelta, r.projectName, r.currentLanguage)
	}
	if result.refreshRecommend.Needed {
		r.profileRefreshRecommended = result.refreshRecommend
	}
}

func (r *learnCurrentProjectRun) savePatternsStep() error {
	saveStartedAt := time.Now()
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error { return nil }); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.save_patterns",
		"duration", time.Since(saveStartedAt),
		"patterns_count", len(r.patterns),
		"saved_count", r.savedCount,
	)
	if r.opts.showDetailedLogs && len(r.patterns) > 0 {
		logger.Info(i18n.GetWithParams("LearnCurrentPatternsSaved", map[string]interface{}{"Count": r.savedCount}))
	}
	return nil
}

func (r *learnCurrentProjectRun) saveProfileIfNeeded() error {
	profileStartedAt := time.Now()
	saveProfileFromDelta := r.hasProfileDelta && r.opts.profileMode != learnCurrentProfileRefresh && !r.refreshProfile && r.existingProfile != nil
	if r.hasProfileDelta && r.opts.profileMode == learnCurrentProfileAuto && r.existingProfile == nil {
		r.refreshProfile = true
	}
	if r.refreshProfile || saveProfileFromDelta {
		if err := r.steps.Run(i18n.Get("ProgressLearnCurrentSaveProfile"), func() error {
			if saveProfileFromDelta {
				return r.cont.ProfileRepo.Save(r.ctx, r.mergedProfile)
			}
			profile, err := analyzeProjectProfile(r.ctx, r.cont, r.projectRoot, r.projectName, r.currentLanguage, r.effectiveFocusPaths, r.existingProfile)
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
			return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.save_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", r.opts.profileMode,
			"profile_delta", saveProfileFromDelta,
			"incremental_profile", r.existingProfile != nil && len(r.resolvedFocusPaths) > 0,
		)
		if r.opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSaved"))
		}
	} else {
		if err := r.steps.Run(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil }); err != nil {
			return err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.skip_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", r.opts.profileMode,
		)
		if r.opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
		}
	}
	return nil
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
