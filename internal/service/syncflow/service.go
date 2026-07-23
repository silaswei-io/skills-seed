package syncflow

import (
	"context"
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

// ChangeRecorder 记录 sync 流程产生的变更摘要。
type ChangeRecorder interface {
	Detail(message string)
}

// LearnCurrentFunc 执行当前代码学习。
type LearnCurrentFunc func(ctx context.Context, req LearnRequest) (domain.LearnCurrentResult, error)

// LearnRequest 描述 sync 传给学习阶段的恢复参数。
type LearnRequest struct {
	StateScope     string
	UserContext    string
	Force          bool
	CurationOutput string
}

// GenerateFunc 执行 skills 生成。
type GenerateFunc func(ctx context.Context) error

// OutputMissingFunc 判断目标 skills 输出是否缺失。
type OutputMissingFunc func() bool

// Service 编排 sync 的学习与生成流程。
type Service struct {
	LearnCurrent  LearnCurrentFunc
	Generate      GenerateFunc
	OutputMissing OutputMissingFunc
}

// Request 描述一次 sync 运行的输入。
type Request struct {
	Learn  LearnRequest
	Change ChangeRecorder
}

// Run 执行当前代码学习，并在必要时生成 skills。
func (s Service) Run(ctx context.Context, req Request) error {
	startedAt := time.Now()
	if s.LearnCurrent == nil {
		return fmt.Errorf("sync learn dependency is not configured")
	}
	if s.Generate == nil {
		return fmt.Errorf("sync generate dependency is not configured")
	}

	logger.Info(i18n.Get("SyncStepLearn"))
	result, err := s.LearnCurrent(ctx, req.Learn)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
	}
	logger.InfoAfterProgress(i18n.GetWithParams("SyncLearnCompleted", map[string]interface{}{
		"Changed":  result.Summary.ChangedFiles,
		"Deleted":  result.Summary.DeletedFiles,
		"Patterns": result.Summary.PatternsFound,
		"Saved":    result.Summary.PatternsSaved,
		"Duration": time.Since(startedAt).Round(time.Second),
	}))
	RecordLearnSummary(req.Change, result)

	outputMissing := false
	if s.OutputMissing != nil {
		outputMissing = s.OutputMissing()
	}
	return RunAfterLearn(result, outputMissing, func() error {
		return s.Generate(ctx)
	}, req.Change)
}

// RunAfterLearn 根据学习结果决定是否继续生成 skills。
func RunAfterLearn(result domain.LearnCurrentResult, outputMissing bool, generate func() error, change ChangeRecorder) error {
	startedAt := time.Now()
	if !ShouldGenerateAfterLearn(result) && !outputMissing {
		if change != nil {
			change.Detail(i18n.Get("ChangeLogGenerateSkippedNoChanges"))
		}
		logger.Info(i18n.Get("SyncGenerateSkippedNoChanges"))
		logger.Info(i18n.Get("SyncComplete"))
		return nil
	}

	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := generate(); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}
	logger.InfoAfterProgress(i18n.GetWithParams("SyncGenerateCompleted", map[string]interface{}{
		"Duration": time.Since(startedAt).Round(time.Second),
	}))
	if change != nil {
		change.Detail(i18n.Get("ChangeLogGenerateCompletedAll"))
	}

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}

// ShouldGenerateAfterLearn 判断学习结果是否需要触发生成。
func ShouldGenerateAfterLearn(result domain.LearnCurrentResult) bool {
	summary := result.Summary
	if summary.Projects > 0 {
		return summary.ChangedProjects > 0 || summary.WorkspaceChanged
	}
	if summary.NoFileChanges {
		return false
	}
	return summary.ChangedFiles > 0 || summary.DeletedFiles > 0 || summary.PatternsFound > 0 || summary.PatternsSaved > 0
}

// RecordLearnSummary 写入学习阶段的变更摘要。
func RecordLearnSummary(change ChangeRecorder, result domain.LearnCurrentResult) {
	if change == nil {
		return
	}
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
