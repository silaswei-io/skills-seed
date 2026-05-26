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
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/utils"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

const (
	learnCurrentProfileAuto    = "auto"
	learnCurrentProfileSkip    = "skip"
	learnCurrentProfileRefresh = "refresh"
)

var (
	// history 模式参数
	limit     int
	since     string
	batchSize int

	// current 模式参数
	language               string
	focusPaths             []string
	learnCurrentProfileOpt string
)

// Cmd 返回 learn 命令
func Cmd(cont *container.Container) *cobra.Command {
	learnCmd := &cobra.Command{
		Use:   "learn",
		Short: i18n.Get("LearnShort"),
		Long:  i18n.Get("LearnLongDesc"),
	}
	defaultLimit, defaultBatchSize := historyDefaults(cont)

	// learn current 子命令
	currentCmd := &cobra.Command{
		Use:   "current",
		Short: i18n.Get("LearnCurrentShort"),
		Long:  i18n.Get("LearnCurrentLongDesc"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return runLearnCurrent(cont)
		},
	}
	currentCmd.Flags().StringVarP(&language, "language", "l", "", i18n.Get("LearnFlagLanguage"))
	currentCmd.Flags().StringArrayVarP(&focusPaths, "focus", "f", nil, i18n.Get("LearnFlagFocus"))
	currentCmd.Flags().StringVar(&learnCurrentProfileOpt, "profile", learnCurrentProfileAuto, i18n.Get("LearnFlagProfile"))

	// learn history 子命令
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: i18n.Get("LearnHistoryShort"),
		Long:  i18n.Get("LearnHistoryLongDesc"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return runLearnHistory(cont)
		},
	}
	historyCmd.Flags().IntVarP(&limit, "limit", "n", defaultLimit, i18n.Get("LearnFlagLimit"))
	historyCmd.Flags().StringVarP(&since, "since", "s", "", i18n.Get("LearnFlagSince"))
	historyCmd.Flags().IntVarP(&batchSize, "batch-size", "b", defaultBatchSize, i18n.Get("LearnFlagBatchSize"))

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
	if learningConfig.MaxCommits > 0 {
		defaultLimit = learningConfig.MaxCommits
	}
	if learningConfig.BatchSize > 0 {
		defaultBatchSize = learningConfig.BatchSize
	}
	return defaultLimit, defaultBatchSize
}

// runLearnCurrent 从当前代码库学习
func runLearnCurrent(cont *container.Container) error {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return err
	}
	if cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return runLearnWorkspaceCurrent(cont)
	}

	ctx := agent.WithTokenUsageScope(context.Background(), "")
	startedAt := time.Now()
	tracker := progress.New(5)

	var projectRoot string
	var projectName string
	var currentLanguage string
	var resolvedFocusPaths []string
	var refreshProfile bool
	var existingProfile *domain.ProjectProfile

	logger.Info(i18n.Get("LearnCurrentStart"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "command.learn_current",
		"agent", cont.Agent.Name(),
		"seed_path", cont.SeedPath,
	)

	// 解析项目上下文可能访问 Git 和配置文件，单独作为第一步展示，避免用户以为命令无响应
	prepareStartedAt := time.Now()
	if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentPrepareProject"), func() error {
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

		currentLanguage = language
		if currentLanguage == "" {
			currentLanguage = cont.ConfigRepo.GetProjectConfig().Language
		}
		if currentLanguage == "" {
			currentLanguage = "go"
		}

		resolvedFocusPaths, err = resolveFocusPaths(projectRoot, focusPaths)
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
		refreshProfile, err = shouldRefreshProfile(projectRoot, resolvedFocusPaths, learnCurrentProfileOpt, profileExists)
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
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGetCurrentDir", map[string]interface{}{"Error": err.Error()}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.prepare_project",
		"duration", time.Since(prepareStartedAt),
		"project_root", projectRoot,
		"project_name", projectName,
		"language", currentLanguage,
		"focus_paths", strings.Join(utils.RelativePaths(projectRoot, resolvedFocusPaths), ","),
		"profile_mode", learnCurrentProfileOpt,
		"refresh_profile", refreshProfile,
	)
	logger.Info(i18n.GetWithParams("LearnCurrentInfo", map[string]interface{}{
		"ProjectRoot": projectRoot,
		"ProjectName": projectName,
		"Language":    currentLanguage,
	}))
	if len(resolvedFocusPaths) > 0 {
		logger.Info(i18n.GetWithParams("LearnCurrentFocusInfo", map[string]interface{}{
			"Focus":       strings.Join(utils.RelativePaths(projectRoot, resolvedFocusPaths), ", "),
			"ProfileMode": learnCurrentProfileOpt,
		}))
	}

	var incrementalChanges *incrementalFileChanges
	var effectiveFocusPaths []string
	detectStartedAt := time.Now()
	if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentDetectChanges"), func() error {
		var err error
		incrementalChanges, err = prepareIncrementalFileChanges(ctx, cont.FileTracker, cont.ConfigRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, resolvedFocusPaths)
		if err != nil {
			return err
		}
		effectiveFocusPaths = resolveIncrementalFocusPaths(projectRoot, incrementalChanges.FocusPaths())
		return nil
	}); err != nil {
		return err
	}
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
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.detect_changes",
		"duration", time.Since(detectStartedAt),
		"changed_count", len(incrementalChanges.AddedOrModified),
		"deleted_count", len(incrementalChanges.Deleted),
		"unchanged_count", len(incrementalChanges.Unchanged),
		"skipped_count", len(incrementalChanges.Skipped),
	)
	if learnCurrentProfileOpt != learnCurrentProfileSkip && incrementalChanges.HasChanges() {
		refreshProfile = true
	}

	var patterns []domain.Pattern
	var businessRulesCount int
	var bestPracticesCount int
	var commonPatternsCount int

	if !incrementalChanges.HasChanges() {
		logger.Info(i18n.Get("LearnCurrentNoFileChanges"))
		if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error { return nil }); err != nil {
			return err
		}
		if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error { return nil }); err != nil {
			return err
		}
		if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil }); err != nil {
			return err
		}
		logger.Info(i18n.Get("LearnCurrentComplete"))
		logLearnCurrentNextSteps()
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current",
			"duration", time.Since(startedAt),
			"patterns_count", 0,
			"saved_count", 0,
			"skipped", true,
		)
		agent.FlushTokenUsageScope(ctx)
		return nil
	}

	// AI 分析是 learn current 最耗时的步骤，进度行会持续刷新当前耗时
	analyzeStartedAt := time.Now()
	if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error {
		analyzeResult, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(ctx, projectRoot, projectName, currentLanguage, analyzer.AnalyzeCodebaseOptions{
			FocusPaths: effectiveFocusPaths,
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
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToAnalyzeCodebase", map[string]interface{}{"Error": err.Error()}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.analyze_codebase",
		"duration", time.Since(analyzeStartedAt),
		"patterns_count", len(patterns),
		"business_rules_count", businessRulesCount,
		"best_practices_count", bestPracticesCount,
		"common_patterns_count", commonPatternsCount,
	)

	logger.Info(i18n.GetWithParams("LearnCurrentResult", map[string]interface{}{
		"PatternsCount":       len(patterns),
		"BusinessRulesCount":  businessRulesCount,
		"BestPracticesCount":  bestPracticesCount,
		"CommonPatternsCount": commonPatternsCount,
	}))

	savedCount := 0
	saveStartedAt := time.Now()
	if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error {
		savedCount = cont.LearnerSvc.SavePatterns(ctx, patterns, "learn_current")
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.save_patterns",
			"duration", time.Since(saveStartedAt),
			"patterns_count", len(patterns),
			"saved_count", savedCount,
			"error", err,
		)
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.save_patterns",
		"duration", time.Since(saveStartedAt),
		"patterns_count", len(patterns),
		"saved_count", savedCount,
	)
	if len(patterns) > 0 {
		logger.Info(i18n.GetWithParams("LearnCurrentPatternsSaved", map[string]interface{}{"Count": savedCount}))
	}

	profileStartedAt := time.Now()
	if refreshProfile {
		if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentSaveProfile"), func() error {
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
			return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.save_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", learnCurrentProfileOpt,
			"incremental_profile", existingProfile != nil && len(resolvedFocusPaths) > 0,
		)
		logger.Info(i18n.Get("LearnCurrentProfileSaved"))
	} else {
		if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil }); err != nil {
			return err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.skip_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", learnCurrentProfileOpt,
		)
		logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
	}

	if err := commitIncrementalFileChanges(ctx, cont.FileTracker, incrementalChanges); err != nil {
		return err
	}

	logger.Info(i18n.Get("LearnCurrentComplete"))
	logLearnCurrentNextSteps()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current",
		"duration", time.Since(startedAt),
		"patterns_count", len(patterns),
		"saved_count", savedCount,
	)
	if err := commandutil.MarkLearned(ctx, cont); err != nil {
		return err
	}
	agent.FlushTokenUsageScope(ctx)

	return nil
}

func logLearnCurrentNextSteps() {
	logger.Info(i18n.Get("LearnCurrentNextSteps"))
	logger.Info(i18n.Get("LearnCurrentNextViewPatterns"))
	logger.Info(i18n.Get("LearnCurrentNextGenerateSkills"))
	logger.Info(i18n.Get("LearnCurrentNextGenerateMergedSkills"))
	logger.Info(i18n.Get("LearnCurrentNextCheck"))
}

func resolveIncrementalFocusPaths(projectRoot string, relPaths []string) []string {
	paths := make([]string, 0, len(relPaths))
	for _, relPath := range relPaths {
		paths = append(paths, filepath.Join(projectRoot, filepath.FromSlash(relPath)))
	}
	return paths
}

func runLearnWorkspaceCurrent(cont *container.Container) error {
	ctx := context.Background()
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

	if cont.WorkspaceProfileRepo != nil {
		profile := workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig)
		profile.GeneratedAt = time.Now().Format(time.RFC3339)
		if err := cont.WorkspaceProfileRepo.Save(ctx, profile); err != nil {
			return err
		}
	}

	parallelism := workspacediscovery.EffectiveParallelism(domain.ModeWorkspace, cont.ConfigRepo.GetAgentConfig().Parallelism, len(workspaceConfig.Projects))
	logger.Info(i18n.Get("LearnWorkspaceStart"), "projects", len(workspaceConfig.Projects), "parallelism", parallelism)

	var mu sync.Mutex
	logProjectInfo := func(message string, params map[string]interface{}) {
		mu.Lock()
		defer mu.Unlock()
		logger.Info(i18n.GetWithParams(message, params))
	}
	finishProjectLog := func(tokenCtx context.Context, message string, params map[string]interface{}) {
		mu.Lock()
		defer mu.Unlock()
		logger.Info(i18n.GetWithParams(message, params))
		agent.FlushTokenUsageScope(tokenCtx)
	}
	totalPatterns := 0
	totalSaved := 0
	err := workspacediscovery.RunProjectTasks(ctx, workspaceConfig.Projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
		tokenScope := project.ID
		if tokenScope == "" {
			tokenScope = project.Path
		}
		projectCtx := agent.WithTokenUsageScope(ctx, tokenScope)

		projectRootPath := workspacediscovery.ProjectRoot(projectRoot, project)
		logProjectInfo("LearnWorkspaceProjectInfo", map[string]interface{}{
			"ProjectRoot": projectRootPath,
			"ProjectName": project.ID,
			"Language":    project.Language,
			"Path":        project.Path,
		})

		result, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(projectCtx, projectRootPath, project.ID, project.Language, analyzer.AnalyzeCodebaseOptions{})
		if err != nil {
			finishProjectLog(projectCtx, "LearnWorkspaceProjectFailed", map[string]interface{}{
				"ProjectName": project.ID,
				"Error":       err.Error(),
			})
			return err
		}
		logProjectInfo("LearnWorkspaceProjectResult", map[string]interface{}{
			"ProjectName":        project.ID,
			"PatternsCount":      len(learnedPatterns),
			"BusinessRulesCount": len(result.BusinessRules),
			"BestPracticesCount": len(result.BestPractices),
		})
		if len(learnedPatterns) == 0 {
			logProjectInfo("LearnWorkspaceProjectPatternsSkipped", map[string]interface{}{
				"ProjectName": project.ID,
			})
		}

		for i := range learnedPatterns {
			learnedPatterns[i].ProjectID = project.ID
			learnedPatterns[i].ScopePath = project.Path
			learnedPatterns[i].WorkspaceRole = project.Type
		}
		saved := cont.LearnerSvc.SavePatterns(projectCtx, learnedPatterns, "learn_workspace_current:"+project.ID)
		if len(learnedPatterns) > 0 {
			logProjectInfo("LearnWorkspaceProjectPatternsSaved", map[string]interface{}{
				"ProjectName": project.ID,
				"Count":       saved,
			})
		}

		projectProfile := analyzer.NewProjectProfile(&analyzer.AnalyzeProjectResult{
			Language: project.Language,
			Summary:  result.Summary,
		}, project.ID, project.Language)
		projectAnalysis, err := cont.AnalyzerSvc.AnalyzeProjectFullWithLanguage(projectCtx, projectRootPath, project.ID, project.Language)
		if err == nil {
			projectProfile = analyzer.NewProjectProfile(projectAnalysis, project.ID, project.Language)
		} else {
			logProjectInfo("LearnWorkspaceProjectProfileFallback", map[string]interface{}{
				"ProjectName": project.ID,
				"Error":       err.Error(),
			})
		}
		if cont.ProfileRepo != nil {
			if err := cont.ProfileRepo.SaveForProject(projectCtx, project.ID, projectProfile); err != nil {
				finishProjectLog(projectCtx, "LearnWorkspaceProjectFailed", map[string]interface{}{
					"ProjectName": project.ID,
					"Error":       err.Error(),
				})
				return err
			}
			finishProjectLog(projectCtx, "LearnWorkspaceProjectProfileSaved", map[string]interface{}{
				"ProjectName": project.ID,
			})
		} else {
			finishProjectLog(projectCtx, "LearnWorkspaceProjectProfileSkipped", map[string]interface{}{
				"ProjectName": project.ID,
			})
		}

		mu.Lock()
		totalPatterns += len(learnedPatterns)
		totalSaved += saved
		mu.Unlock()
		return nil
	})
	if err != nil {
		return err
	}

	if err := commandutil.MarkLearned(ctx, cont); err != nil {
		return err
	}

	logger.Info(i18n.Get("LearnWorkspaceComplete"), "patterns", totalPatterns, "saved", totalSaved)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_workspace_current",
		"duration", time.Since(startedAt),
		"projects_count", len(workspaceConfig.Projects),
		"patterns_count", totalPatterns,
		"saved_count", totalSaved,
	)
	return nil
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
		if !profileExists || len(focusPaths) == 0 {
			return true, nil
		}
		if hasRootFocus(projectRoot, focusPaths) {
			return true, nil
		}
		if hasCriticalFocus(projectRoot, focusPaths) {
			return true, nil
		}
		return focusModuleCount(projectRoot, focusPaths) > 1, nil
	case learnCurrentProfileSkip:
		return false, nil
	case learnCurrentProfileRefresh:
		return true, nil
	default:
		return false, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileModeInvalid", map[string]interface{}{"Mode": mode}))
	}
}

func hasRootFocus(projectRoot string, focusPaths []string) bool {
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return false
	}
	projectAbs = filepath.Clean(projectAbs)
	for _, path := range focusPaths {
		absPath, err := filepath.Abs(path)
		if err == nil && filepath.Clean(absPath) == projectAbs {
			return true
		}
	}
	return false
}

func hasCriticalFocus(projectRoot string, focusPaths []string) bool {
	for _, relPath := range utils.RelativePaths(projectRoot, focusPaths) {
		if isCriticalFocusPath(relPath) {
			return true
		}
	}
	return false
}

func isCriticalFocusPath(relPath string) bool {
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	criticalPrefixes := []string{
		"cmd",
		"internal/bootstrap",
		"internal/container",
		"internal/domain",
		"internal/infra/config",
	}
	for _, prefix := range criticalPrefixes {
		if relPath == prefix || strings.HasPrefix(relPath, prefix+"/") {
			return true
		}
	}
	return false
}

func focusModuleCount(projectRoot string, focusPaths []string) int {
	modules := make(map[string]bool)
	for _, relPath := range utils.RelativePaths(projectRoot, focusPaths) {
		key := focusModuleKey(relPath)
		if key != "" {
			modules[key] = true
		}
	}
	return len(modules)
}

func focusModuleKey(relPath string) string {
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || relPath == "" {
		return relPath
	}
	parts := strings.Split(relPath, "/")
	if len(parts) >= 2 && parts[0] == "internal" {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// runLearnHistory 从 Git 历史提交学习
func runLearnHistory(cont *container.Container) error {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return err
	}

	ctx := context.Background()
	startedAt := time.Now()

	logger.Info(i18n.Get("LearnHistoryStart"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "command.learn_history",
		"agent", cont.Agent.Name(),
		"limit", limit,
		"since", since,
		"batch_size", batchSize,
	)
	logger.Info(i18n.GetWithParams("LearnHistoryInfo", map[string]interface{}{
		"Limit":     limit,
		"Since":     since,
		"BatchSize": batchSize,
	}))

	// 调用学习服务
	if err := commandutil.LockConfiguredMode(ctx, cont); err != nil {
		return err
	}
	err := cont.LearnerSvc.Learn(ctx, limit, since, batchSize)
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
