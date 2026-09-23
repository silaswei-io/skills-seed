package learncurrent

import (
	"context"
	"time"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
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
	// 阶段：准备 → 检测/恢复 → 议程规划 → 分析审查 → 规范化入库 → 画像刷新。
	learnCurrentProjectStepTotal = 6
)

type learnCurrentOptions struct {
	language       string
	focusPaths     []string
	profileMode    string
	contextText    string
	contextPath    []string
	userContext    string
	stateScope     string
	scopeKind      runjournal.ScopeKind
	force          bool
	quiet          bool
	onStepStart    func(label string)
	onStepUpdate   func(label string)
	onStepComplete func(label string)
}

// CurrentRunOptions 描述外部命令调用 learn current 时允许覆盖的执行选项。
type CurrentRunOptions struct {
	// Force 表示忽略已保存的文件指纹，重新学习当前扫描范围。
	Force bool
	// Quiet 表示作为上层工作区流程的子步骤运行，不直接输出项目级进度和详细日志。
	Quiet          bool
	ScopeKind      runjournal.ScopeKind
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

// Flags 是 CLI 入口解析后的 learn current 参数。
type Flags struct {
	Language    string
	FocusPaths  []string
	ProfileMode string
	ContextText string
	ContextPath []string
	Force       bool
}

// RunWithFlags 供 CLI 调用的完整入口。
func RunWithFlags(ctx context.Context, cont *container.Container, flags Flags) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(ctx, cont, learnCurrentOptions{
		language:    flags.Language,
		focusPaths:  flags.FocusPaths,
		profileMode: flags.ProfileMode,
		contextText: flags.ContextText,
		contextPath: flags.ContextPath,
		force:       flags.Force,
		scopeKind:   runjournal.ScopeProject,
	})
}

// RecordSummary 把学习摘要写入 changelog。
func RecordSummary(change *changelog.Builder, result domain.LearnCurrentResult) {
	recordLearnCurrentSummary(change, result)
}

// Run 从当前代码库学习，并返回学习摘要。
func Run(ctx context.Context, cont *container.Container) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(ctx, cont, learnCurrentOptions{
		profileMode: learnCurrentProfileAuto,
		scopeKind:   runjournal.ScopeProject,
	})
}

// RunWithContext 从当前代码库学习，附加一次性用户上下文。
func RunWithContext(ctx context.Context, cont *container.Container, userContext string) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(ctx, cont, learnCurrentOptions{
		profileMode: learnCurrentProfileAuto,
		userContext: userContext,
		scopeKind:   runjournal.ScopeProject,
	})
}

// RunWithStateScope 从当前代码库学习，并使用指定恢复状态 scope。
func RunWithStateScope(ctx context.Context, cont *container.Container, stateScope string, userContext string) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(ctx, cont, learnCurrentOptions{
		profileMode: learnCurrentProfileAuto,
		userContext: userContext,
		stateScope:  stateScope,
		scopeKind:   runjournal.ScopeProject,
	})
}

// RunWithStateScopeOptions 从当前代码库学习，并允许调用方指定运行选项。
func RunWithStateScopeOptions(ctx context.Context, cont *container.Container, stateScope string, userContext string, opts CurrentRunOptions) (domain.LearnCurrentResult, error) {
	return runLearnCurrent(ctx, cont, learnCurrentOptions{
		profileMode:    learnCurrentProfileAuto,
		userContext:    userContext,
		stateScope:     stateScope,
		scopeKind:      opts.ScopeKind,
		force:          opts.Force,
		quiet:          opts.Quiet,
		onStepStart:    opts.OnStepStart,
		onStepUpdate:   opts.OnStepUpdate,
		onStepComplete: opts.OnStepComplete,
	})
}

func runLearnCurrent(ctx context.Context, cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	if opts.profileMode == "" {
		opts.profileMode = learnCurrentProfileAuto
	}
	if opts.scopeKind == "" {
		opts.scopeKind = runjournal.ScopeProject
	}
	if opts.userContext == "" {
		userContext, err := commandutil.ResolveRuntimeContext(opts.contextText, opts.contextPath...)
		if err != nil {
			return domain.LearnCurrentResult{}, err
		}
		opts.userContext = userContext
	}
	if cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return runLearnWorkspaceCurrent(ctx, cont, opts)
	}
	return runLearnCurrentProject(ctx, cont, opts)
}

func runLearnCurrentProject(ctx context.Context, cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	result, err := runLearnCurrentProjectWithOptions(ctx, cont, learnCurrentProjectOptions{
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
		scopeKind:        opts.scopeKind,
		force:            opts.force,
	})
	if err != nil {
		return domain.LearnCurrentResult{}, err
	}
	summary := domain.LearnCurrentSummary{
		ChangedFiles:    result.changedCount,
		DeletedFiles:    result.deletedCount,
		SkippedFiles:    result.skippedCount,
		PatternsFound:   result.patternsCount,
		PatternsSaved:   result.savedCount,
		PatternsRetired: result.retiredCount,
		PatternsDropped: result.droppedCount,
		DropReasons:     dropReasonSummaries(result.dropped),
		NoFileChanges:   result.skipped,
		SkippedStages:   append([]string(nil), result.skippedStages...),
		AnalysisMode:    result.analysisMode,
		Resumed:         result.resumed,
		WallMs:          result.duration.Milliseconds(),
	}
	if result.metrics != nil {
		summary.AgentCallTotal = result.metrics.AgentCallTotal
		if summary.WallMs == 0 {
			summary.WallMs = result.metrics.WallMs
		}
	}
	return domain.LearnCurrentResult{Summary: summary}, nil
}

func dropReasonSummaries(dropped []patternnorm.Drop) []string {
	if len(dropped) == 0 {
		return nil
	}
	out := make([]string, 0, len(dropped))
	for _, item := range dropped {
		reason := string(item.ReasonCode)
		if item.Reason != "" {
			if reason != "" {
				reason = reason + ": " + item.Reason
			} else {
				reason = item.Reason
			}
		}
		if item.ID != "" && reason != "" {
			out = append(out, item.ID+" ("+reason+")")
			continue
		}
		if item.ID != "" {
			out = append(out, item.ID)
			continue
		}
		if reason != "" {
			out = append(out, reason)
		}
	}
	return out
}

type learnCurrentProjectOptions struct {
	showProgress       bool
	showDetailedLogs   bool
	userContext        string
	skipRuntimeCleanup bool
	onStepStart        func(label string)
	onStepComplete     func(label string)
	onStepUpdate       func(label string)
	language           string
	focusPaths         []string
	profileMode        string
	stateScope         string
	scopeKind          runjournal.ScopeKind
	force              bool
}

type learnCurrentProjectResult struct {
	projectName   string
	changedCount  int
	deletedCount  int
	skippedCount  int
	patternsCount int
	savedCount    int
	retiredCount  int
	droppedCount  int
	dropped       []patternnorm.Drop
	skipped       bool
	duration      time.Duration
	focusCount    int
	changeProfile string
	learningMode  string
	analysisMode  string
	resumed       bool
	skippedStages []string
	metrics       *runjournal.Metrics
}
