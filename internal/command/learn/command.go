package learn

import (
	"context"
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
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
	force          bool
	quiet          bool
	onStepStart    func(label string)
	onStepUpdate   func(label string)
	onStepComplete func(label string)
}

// Cmd 返回 learn 命令
func Cmd(cont *container.Container) *cobra.Command {
	learnCmd := &cobra.Command{
		Use:     "learn",
		Short:   i18n.Get("LearnShort"),
		Long:    i18n.Get("LearnLongDesc"),
		Example: i18n.Get("LearnExample"),
	}

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

	learnCmd.AddCommand(currentCmd)

	return learnCmd
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
	// Quiet 表示作为上层工作区流程的子步骤运行，不直接输出项目级进度和详细日志。
	Quiet          bool
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

// RunLearnCurrentWithStateScopeOptions 从当前代码库学习，并允许调用方指定运行选项。
func RunLearnCurrentWithStateScopeOptions(cont *container.Container, stateScope string, userContext string, opts CurrentRunOptions) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(cont, learnCurrentOptions{
		profileMode:    learnCurrentProfileAuto,
		userContext:    userContext,
		stateScope:     stateScope,
		force:          opts.Force,
		quiet:          opts.Quiet,
		onStepStart:    opts.OnStepStart,
		onStepUpdate:   opts.OnStepUpdate,
		onStepComplete: opts.OnStepComplete,
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
	result, err := runLearnCurrentProjectWithOptions(context.Background(), cont, learnCurrentProjectOptions{
		showProgress:     !opts.quiet,
		showDetailedLogs: !opts.quiet,
		onStepStart:      opts.onStepStart,
		onStepUpdate:     opts.onStepUpdate,
		onStepComplete:   opts.onStepComplete,
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
}
