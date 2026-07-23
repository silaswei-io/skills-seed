// Package learner 提供学习服务（流程编排层）
//
// 本包实现从 Git 历史学习编码模式的流程编排
//   - Learn: 批量学习 Git 提交历史
//   - LearnFromCommit: 从单个提交学习
//   - LearnFromStaged: 从暂存文件路径学习
//
// 服务职责
//   - 流程编排：协调 Git、Agent、Curator 完成学习
//   - 增量学习：只处理未分析的提交
//   - 候选入库：把 AI 学到的候选模式交给 Curator 规范化入库
//
// 不负责
//   - AI 分析（由 Agent 负责）
//   - 模式策展与持久化（由 Curator 负责）
package learner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
)

// LearnerService 学习服务 - 流程编排层
// 职责：协调 Git、Agent、Curator 完成学习流程
type LearnerService struct {
	agent         agent.Agent
	gitRepo       domain.GitRepository
	patternRepo   domain.PatternRepository
	commitTracker domain.CommitAnalysisTracker
	curatorSvc    *curator.Service
}

// NewLearnerService 创建学习服务
func NewLearnerService(
	ag agent.Agent,
	gitRepo domain.GitRepository,
	patternRepo domain.PatternRepository,
	commitTracker domain.CommitAnalysisTracker,
	curatorSvc *curator.Service,
) *LearnerService {
	return &LearnerService{
		agent:         ag,
		gitRepo:       gitRepo,
		patternRepo:   patternRepo,
		commitTracker: commitTracker,
		curatorSvc:    curatorSvc,
	}
}

// marshalKnownPatterns 序列化已知模式为 JSON
func (s *LearnerService) marshalKnownPatterns(ctx context.Context) (string, int) {
	patterns, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		return "", 0
	}
	if len(patterns) == 0 {
		return "", 0
	}

	exportData := make([]map[string]interface{}, len(patterns))
	for i, p := range patterns {
		exportData[i] = map[string]interface{}{
			"id":                 p.ID,
			"name":               p.Name,
			"category":           string(p.Category),
			"description":        p.Description,
			"rule":               p.Rule,
			"confidence":         p.Confidence,
			"frequency":          p.Frequency,
			"metrics":            p.Metrics,
			"source":             string(p.Source),
			"evidence_locations": p.EvidenceLocations,
			"business_method":    p.BusinessMethod,
		}
	}

	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		logger.Warn(i18n.Get("LearnerMarshalKnownPatternsFailed"), "error", err)
		return "", 0
	}
	return string(jsonBytes), len(patterns)
}

// CurateAndSavePatterns 策展并保存多个候选模式。
func (s *LearnerService) CurateAndSavePatterns(ctx context.Context, patterns []domain.Pattern, operation curator.Operation) (int, error) {
	return s.curateAndSavePatterns(ctx, patterns, operation, CurateOptions{})
}

// CurateOptions 描述一次候选模式策展的附加执行能力。
type CurateOptions struct {
	Hooks              curator.ProgressHooks
	DecisionCheckpoint curator.DecisionCheckpoint
}

// CurateAndSavePatternsWithOptions 策展并保存候选模式。
func (s *LearnerService) CurateAndSavePatternsWithOptions(ctx context.Context, patterns []domain.Pattern, operation curator.Operation, opts CurateOptions) (int, error) {
	return s.curateAndSavePatterns(ctx, patterns, operation, opts)
}

func (s *LearnerService) curateAndSavePatterns(ctx context.Context, patterns []domain.Pattern, operation curator.Operation, opts CurateOptions) (int, error) {
	if s.curatorSvc == nil {
		return 0, fmt.Errorf("pattern curator is not configured")
	}
	result, err := s.curatorSvc.CurateAndStoreWithHooks(ctx, curator.CurateRequest{
		Operation:          operation,
		Candidates:         patterns,
		DecisionCheckpoint: opts.DecisionCheckpoint,
	}, opts.Hooks)
	if err != nil {
		return 0, err
	}
	return len(result.Written), nil
}

// KnownPatternsSnapshot 返回给当前代码学习使用的已知模式摘要。
func (s *LearnerService) KnownPatternsSnapshot(ctx context.Context) (string, int) {
	return s.marshalKnownPatterns(ctx)
}

// Learn 从 Git 历史学习模式（流程编排）
//
// 参数
//   - ctx: 上下文
//   - limit: 最大提交数量
//   - since: 时间范围（如 "30d", "7d"）
//   - batchSize: 批量大小
//
// 返回
//   - 成功返回 nil，失败返回错误
//
// 工作流程
//  1. 获取 Git 提交历史
//  2. 过滤掉已分析的提交（增量学习）
//  3. 批量处理提交（batchSize 个一批）
//  4. 调用 AI 分析每个批次
//  5. 策展并保存候选模式
//  6. 标记提交为已分析
//  7. 发布学习完成事件
//
// 增量学习
//   - 使用 patternRepo.IsCommitAnalyzed() 检查提交是否已分析
//   - 使用 commitTracker.MarkCommitsAnalyzed() 原子标记批次已分析
//
// 模式入库
//   - Learner 只产生候选模式
//   - Curator 负责新增、更新、合并或丢弃候选模式
func (s *LearnerService) Learn(ctx context.Context, limit int, since string, batchSize int) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "learner.learn",
		"limit", limit,
		"since", since,
		"batch_size", batchSize,
	)

	if batchSize <= 0 {
		logger.Warn(i18n.Get("LoggerLearnerInvalidBatchSize"), "batch_size", batchSize)
		batchSize = 1
	}

	// 1. 获取 Git 提交历史
	getCommitsStartedAt := time.Now()
	commits, err := s.gitRepo.GetCommits(ctx, limit, since)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "learner.get_commits",
			"duration", time.Since(getCommitsStartedAt),
			"limit", limit,
			"since", since,
			"error", err,
		)
		return domain.NewDomainError(
			domain.ErrGitOperation,
			i18n.Get("LearnerGetCommitsFailed"),
			err,
		)
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "learner.get_commits",
		"duration", time.Since(getCommitsStartedAt),
		"commits_count", len(commits),
	)

	if len(commits) == 0 {
		logger.Info(i18n.Get("LoggerLearnerNoCommits"),
			"operation", "learn",
		)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "learner.learn",
			"duration", time.Since(startedAt),
			"commits_count", 0,
			"skipped", true,
		)
		return nil
	}

	logger.Info(i18n.Get("LoggerLearnerStart"),
		"operation", "learn",
		"commits", len(commits),
		"batch_size", batchSize,
	)

	// 3. 过滤掉已分析的 commits
	filterStartedAt := time.Now()
	var unanalyzedCommits []domain.CommitInfo
	skippedAnalyzed := 0
	for _, c := range commits {
		analyzed, err := s.commitTracker.IsCommitAnalyzed(ctx, c.Hash)
		if err != nil {
			logger.Warn(i18n.Get("LoggerLearnerCheckCommitStatusFailed"),
				"operation", "learn",
				"commit_hash", shortHash(c.Hash),
				"error", err,
			)
			// 继续处理，不跳过
		} else if analyzed {
			logger.Debug(i18n.Get("LoggerLearnerSkipAnalyzed"),
				"operation", "learn",
				"commit_hash", shortHash(c.Hash),
			)
			skippedAnalyzed++
			continue
		}
		unanalyzedCommits = append(unanalyzedCommits, c)
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "learner.filter_commits",
		"duration", time.Since(filterStartedAt),
		"commits_count", len(commits),
		"unanalyzed_count", len(unanalyzedCommits),
		"skipped_analyzed", skippedAnalyzed,
	)

	if len(unanalyzedCommits) == 0 {
		logger.Info(i18n.Get("LearnNoCommits"))
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "learner.learn",
			"duration", time.Since(startedAt),
			"commits_count", len(commits),
			"unanalyzed_count", 0,
			"skipped", true,
		)
		return nil
	}

	logger.Info(i18n.Get("LoggerLearnerCommitsToLearn"),
		"operation", "learn",
		"count", len(unanalyzedCommits),
	)

	// 4. 获取已知模式 JSON
	marshalStartedAt := time.Now()
	knownPatternsJSON, knownPatternsCount := s.marshalKnownPatterns(ctx)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "learner.marshal_known_patterns",
		"duration", time.Since(marshalStartedAt),
		"known_patterns_count", knownPatternsCount,
		"known_patterns_json_length", len(knownPatternsJSON),
	)
	patternsLearned := 0
	var batchErrors []error
	totalBatches := (len(unanalyzedCommits) + batchSize - 1) / batchSize
	batchProgress := progress.New(totalBatches)
	retryProgress := agent.NewRetryProgressBinder(batchProgress.UpdateStep)
	ctx = retryProgress.WithContext(ctx)

	// 5. 批量处理 commits
	for i := 0; i < len(unanalyzedCommits); i += batchSize {
		end := i + batchSize
		if end > len(unanalyzedCommits) {
			end = len(unanalyzedCommits)
		}

		batch := unanalyzedCommits[i:end]
		commitFiles, collectErr := s.collectCommitFiles(ctx, batch, "learn")
		if collectErr != nil {
			batchErrors = append(batchErrors, fmt.Errorf("batch %d collect commit files: %w", i/batchSize+1, collectErr))
		}
		if len(commitFiles) == 0 {
			err := fmt.Errorf("batch %d: no commit file paths available", i/batchSize+1)
			logger.Warn(i18n.Get("LoggerLearnerBatchFailed"),
				"operation", "learn",
				"batch_id", i/batchSize+1,
				"error", err,
			)
			batchErrors = append(batchErrors, err)
			continue
		}
		learnableBatch := commitInfos(commitFiles)
		batchStartedAt := time.Now()
		logger.Info(i18n.Get("LoggerLearnerBatchStart"),
			"operation", "learn",
			"batch_id", i/batchSize+1,
			"range", fmt.Sprintf("%d-%d/%d", i+1, end, len(unanalyzedCommits)),
		)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
			"operation", "learner.batch",
			"batch_id", i/batchSize+1,
			"total_batches", totalBatches,
			"batch_commits_count", len(learnableBatch),
			"known_patterns_count", knownPatternsCount,
		)

		// 调用 Agent 批量学习
		req := &agent.BatchLearnRequest{
			Commits:            learnableBatch,
			CommitFiles:        commitFiles,
			KnownPatternsJSON:  knownPatternsJSON,
			KnownPatternsCount: knownPatternsCount,
		}

		var result *agent.BatchLearnResult
		progressLabel := i18n.GetWithParams("ProgressLearnHistoryBatch", map[string]interface{}{
			"Current":     i/batchSize + 1,
			"Total":       totalBatches,
			"Start":       i + 1,
			"End":         end,
			"CommitTotal": len(unanalyzedCommits),
		})
		// 单个批次的 AI 调用可能持续较久，使用动态进度提示避免终端长时间无输出
		err := batchProgress.RunStep(progressLabel, func() error {
			retryProgress.StartStep(progressLabel)
			var callErr error
			result, callErr = s.agent.BatchLearnFromCommits(ctx, req)
			retryProgress.FinishStep(progressLabel, callErr == nil)
			if callErr != nil {
				return callErr
			}
			return agent.RequireResult(result, "BatchLearnFromCommits")
		})
		if err != nil {
			logger.Warn(i18n.Get("LoggerLearnerBatchFailed"),
				"operation", "learn",
				"batch_id", i/batchSize+1,
				"error", err,
			)
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "learner.batch",
				"batch_id", i/batchSize+1,
				"duration", time.Since(batchStartedAt),
				"batch_commits_count", len(learnableBatch),
				"error", err,
			)
			batchErrors = append(batchErrors, fmt.Errorf("batch %d agent: %w", i/batchSize+1, err))
			continue
		}

		logger.Info(i18n.Get("LoggerLearnerBatchComplete"),
			"operation", "learn",
			"batch_id", i/batchSize+1,
			"patterns_count", len(result.Patterns),
		)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "learner.batch.agent",
			"batch_id", i/batchSize+1,
			"duration", time.Since(batchStartedAt),
			"batch_commits_count", len(learnableBatch),
			"patterns_count", len(result.Patterns),
		)

		// 6. 策展并保存新模式
		beforeSaveCount := patternsLearned
		savedCount, saveErr := s.CurateAndSavePatterns(ctx, result.Patterns, curator.OperationLearnHistory)
		if saveErr != nil {
			logger.Warn(i18n.Get("LoggerLearnerBatchFailed"),
				"operation", "learn",
				"batch_id", i/batchSize+1,
				"error", saveErr,
			)
			batchErrors = append(batchErrors, fmt.Errorf("batch %d save patterns: %w", i/batchSize+1, saveErr))
			continue
		}
		patternsLearned += savedCount

		// 7. 标记这些 commits 已被分析
		commitHashes := make([]string, 0, len(learnableBatch))
		for _, commit := range learnableBatch {
			commitHashes = append(commitHashes, commit.Hash)
		}
		if err := s.commitTracker.MarkCommitsAnalyzed(ctx, commitHashes); err != nil {
			logger.Warn(i18n.Get("LoggerLearnerMarkAnalyzedFailed"), "operation", "learn", "error", err)
			batchErrors = append(batchErrors, fmt.Errorf("batch %d mark commits analyzed: %w", i/batchSize+1, err))
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "learner.batch",
			"batch_id", i/batchSize+1,
			"duration", time.Since(batchStartedAt),
			"batch_commits_count", len(learnableBatch),
			"patterns_count", len(result.Patterns),
			"saved_patterns_count", patternsLearned-beforeSaveCount,
		)
	}

	logger.Info(i18n.Get("LoggerLearnerComplete"),
		"operation", "learn",
		"total_commits", len(unanalyzedCommits),
	)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "learner.learn",
		"duration", time.Since(startedAt),
		"commits_count", len(commits),
		"unanalyzed_count", len(unanalyzedCommits),
		"patterns_learned", patternsLearned,
		"total_batches", totalBatches,
	)

	if len(batchErrors) > 0 {
		return fmt.Errorf("learn history completed with %d failed batch operations: %w", len(batchErrors), errors.Join(batchErrors...))
	}
	return nil
}

func (s *LearnerService) collectCommitFiles(ctx context.Context, commits []domain.CommitInfo, operation string) ([]agent.CommitFileChange, error) {
	commitFiles := make([]agent.CommitFileChange, 0, len(commits))
	var collectErrors []error
	for _, c := range commits {
		files, err := s.gitRepo.GetChangedFiles(ctx, c.Hash)
		if err != nil {
			logger.Warn(i18n.Get("LearnerGetCommitFilesFailed"),
				"operation", operation,
				"commit_hash", shortHash(c.Hash),
				"error", err,
			)
			collectErrors = append(collectErrors, fmt.Errorf("commit %s: %w", shortHash(c.Hash), err))
			continue
		}
		commitFiles = append(commitFiles, agent.CommitFileChange{
			Commit: c,
			Files:  files,
		})
	}
	return commitFiles, errors.Join(collectErrors...)
}

func commitInfos(commitFiles []agent.CommitFileChange) []domain.CommitInfo {
	commits := make([]domain.CommitInfo, 0, len(commitFiles))
	for _, commitFile := range commitFiles {
		commits = append(commits, commitFile.Commit)
	}
	return commits
}

// LearnFromCommit 从单个提交学习
func (s *LearnerService) LearnFromCommit(ctx context.Context, c domain.CommitInfo) error {
	changedFiles, err := s.gitRepo.GetChangedFiles(ctx, c.Hash)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrGitOperation,
			i18n.Get("LearnerGetCommitFilesFailed"),
			err,
		).WithContext("commit_hash", c.Hash)
	}

	knownPatternsJSON, knownPatternsCount := s.marshalKnownPatterns(ctx)

	req := &agent.LearnRequest{
		Commit:             c,
		ChangedFiles:       changedFiles,
		KnownPatternsJSON:  knownPatternsJSON,
		KnownPatternsCount: knownPatternsCount,
	}

	result, err := s.agent.LearnFromCommit(ctx, req)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrAIService,
			i18n.Get("LearnerAILearnFailed"),
			err,
		).WithContext("commit_hash", c.Hash)
	}
	if err := agent.RequireResult(result, "LearnFromCommit"); err != nil {
		return domain.NewDomainError(domain.ErrAIService, i18n.Get("LearnerAILearnFailed"), err)
	}

	_, err = s.CurateAndSavePatterns(ctx, result.Patterns, curator.OperationLearnCommit)
	return err
}

// LearnFromStaged 从暂存文件路径学习
func (s *LearnerService) LearnFromStaged(ctx context.Context, commitInfo domain.CommitInfo) error {
	knownPatternsJSON, knownPatternsCount := s.marshalKnownPatterns(ctx)
	stagedFiles, err := s.gitRepo.GetStagedFiles(ctx)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrGitOperation,
			i18n.Get("GitDiffCachedFailed"),
			err,
		)
	}
	changedFiles := make([]string, 0, len(stagedFiles))
	for _, file := range stagedFiles {
		if file.Path != "" {
			changedFiles = append(changedFiles, file.Path)
		}
	}

	req := &agent.LearnRequest{
		Commit:             commitInfo,
		ChangedFiles:       changedFiles,
		KnownPatternsJSON:  knownPatternsJSON,
		KnownPatternsCount: knownPatternsCount,
	}

	result, err := s.agent.LearnFromCommit(ctx, req)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrAIService,
			i18n.Get("LearnerAILearnFailed"),
			err,
		)
	}
	if err := agent.RequireResult(result, "LearnFromStaged"); err != nil {
		return domain.NewDomainError(domain.ErrAIService, i18n.Get("LearnerAILearnFailed"), err)
	}

	_, err = s.CurateAndSavePatterns(ctx, result.Patterns, curator.OperationLearnStaged)
	return err
}

// shortHash 安全截取 hash 前 7 位
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
