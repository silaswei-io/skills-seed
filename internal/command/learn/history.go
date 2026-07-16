package learn

import (
	"context"
	"time"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

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
