package learn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/changelog"
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
	learnCurrentProjectStepTotal = 7
)

type learnCurrentOptions struct {
	language       string
	focusPaths     []string
	profileMode    string
	contextText    string
	contextPath    []string
	userContext    string
	stateScope     string
	curationOutput string
	force          bool
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
	currentCmd.Flags().StringVar(&currentOpts.curationOutput, "curation-output", "", i18n.Get("LearnFlagCurationOutput"))
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
	// CurationOutput 指向已完成的 CuratePatterns 输出，用于恢复时跳过 AI 策展。
	CurationOutput string
}

// RunLearnCurrentWithStateScopeOptions 从当前代码库学习，并允许调用方指定运行选项。
func RunLearnCurrentWithStateScopeOptions(cont *container.Container, stateScope string, userContext string, opts CurrentRunOptions) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(cont, learnCurrentOptions{
		profileMode:    learnCurrentProfileAuto,
		userContext:    userContext,
		stateScope:     stateScope,
		curationOutput: opts.CurationOutput,
		force:          opts.Force,
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
		if strings.TrimSpace(opts.curationOutput) != "" {
			return domain.LearnCurrentResult{}, fmt.Errorf("curation output must be resumed from the matching child project")
		}
		return runLearnWorkspaceCurrent(cont, opts)
	}
	return runLearnCurrentProject(cont, opts)
}

func runLearnCurrentProject(cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	result, err := runLearnCurrentProjectWithOptions(context.Background(), cont, learnCurrentProjectOptions{
		showProgress:     true,
		showDetailedLogs: true,
		userContext:      opts.userContext,
		language:         opts.language,
		focusPaths:       opts.focusPaths,
		profileMode:      opts.profileMode,
		stateScope:       opts.stateScope,
		curationOutput:   opts.curationOutput,
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
	curationOutput   string
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
