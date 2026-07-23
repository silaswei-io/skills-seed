// Package learner 提供学习服务（流程编排层）
//
// 本包实现历史证据增强和显式提交学习的流程编排
//   - Learn: 本地批量增强 Git 历史证据
//   - LearnFromCommit: 处理单个显式提交
//   - LearnFromStaged: 处理暂存文件路径
//
// 服务职责
//   - 流程编排：协调 Git、模式仓储和可选 Agent 完成学习
//   - 增量学习：只处理未分析的提交
//   - 历史增强：把 commit 变更路径关联到已有模式证据
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
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
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
	backend       config.LearningBackend
}

// NewLearnerService 创建学习服务
func NewLearnerService(
	ag agent.Agent,
	gitRepo domain.GitRepository,
	patternRepo domain.PatternRepository,
	commitTracker domain.CommitAnalysisTracker,
	curatorSvc *curator.Service,
) *LearnerService {
	return NewLearnerServiceWithBackend(ag, gitRepo, patternRepo, commitTracker, curatorSvc, config.LearningBackendAgent)
}

// NewLearnerServiceWithBackend 创建使用指定学习后端的学习服务。
func NewLearnerServiceWithBackend(
	ag agent.Agent,
	gitRepo domain.GitRepository,
	patternRepo domain.PatternRepository,
	commitTracker domain.CommitAnalysisTracker,
	curatorSvc *curator.Service,
	backend config.LearningBackend,
) *LearnerService {
	return &LearnerService{
		agent:         ag,
		gitRepo:       gitRepo,
		patternRepo:   patternRepo,
		commitTracker: commitTracker,
		curatorSvc:    curatorSvc,
		backend:       config.NormalizeLearningBackend(string(backend)),
	}
}

type historyCommit struct {
	Commit domain.CommitInfo
	Paths  []string
}

func enrichPatternsWithHistory(patterns []domain.Pattern, commits []historyCommit) []domain.Pattern {
	updated := map[string]domain.Pattern{}
	for _, change := range commits {
		changed := normalizedHistoryPaths(change.Paths)
		for index := range patterns {
			if !patternTouchesPaths(patterns[index], changed) || containsString(patterns[index].HistoryEvidence.CommitHashes, change.Commit.Hash) {
				continue
			}
			evidence := &patterns[index].HistoryEvidence
			evidence.CommitHashes = append(evidence.CommitHashes, change.Commit.Hash)
			evidence.CommitCount = len(evidence.CommitHashes)
			if evidence.FirstSeenAt.IsZero() || change.Commit.Date.Before(evidence.FirstSeenAt) {
				evidence.FirstSeenAt = change.Commit.Date
			}
			if change.Commit.Date.After(evidence.LastSeenAt) {
				evidence.LastSeenAt = change.Commit.Date
			}
			evidence.CoChangedPaths = mergeHistoryPaths(evidence.CoChangedPaths, changed)
			updated[patterns[index].ID] = patterns[index]
		}
	}
	result := make([]domain.Pattern, 0, len(updated))
	for _, pattern := range updated {
		result = append(result, pattern)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func patternTouchesPaths(pattern domain.Pattern, changed map[string]bool) bool {
	for _, location := range pattern.EvidenceLocations {
		if changed[filepath.ToSlash(filepath.Clean(location.Path))] {
			return true
		}
	}
	return false
}

func normalizedHistoryPaths(paths []string) map[string]bool {
	result := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if path != "" && path != "." && !filepath.IsAbs(path) && !strings.HasPrefix(path, "../") {
			result[path] = true
		}
	}
	return result
}

func mergeHistoryPaths(existing []string, additions map[string]bool) []string {
	seen := make(map[string]bool, len(existing)+len(additions))
	for _, path := range existing {
		seen[path] = true
	}
	for path := range additions {
		seen[path] = true
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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

// CurateAndSavePatternsWithMetadata 策展并保存候选模式，同时补充学习范围。
func (s *LearnerService) CurateAndSavePatternsWithMetadata(ctx context.Context, patterns []domain.Pattern, operation curator.Operation, unit domain.AnalysisUnit) (int, error) {
	return s.curateAndSavePatterns(ctx, patterns, operation, CurateOptions{Unit: unit})
}

// CurateOptions 描述一次候选模式策展的附加执行能力。
type CurateOptions struct {
	Unit               domain.AnalysisUnit
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
	patterns = attachAnalysisUnit(patterns, opts.Unit)
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

func attachAnalysisUnit(patterns []domain.Pattern, unit domain.AnalysisUnit) []domain.Pattern {
	if unit.ID == "" && unit.Name == "" {
		return patterns
	}
	out := append([]domain.Pattern(nil), patterns...)
	for i := range out {
		if out[i].AnalysisUnitID == "" {
			out[i].AnalysisUnitID = unit.ID
		}
		if out[i].AnalysisUnitName == "" {
			out[i].AnalysisUnitName = unit.Name
		}
	}
	return out
}

// KnownPatternsSnapshot 返回给当前代码学习使用的已知模式摘要。
func (s *LearnerService) KnownPatternsSnapshot(ctx context.Context) (string, int) {
	return s.marshalKnownPatterns(ctx)
}

// Learn 使用 Git 历史为当前模式补充源码证据，不生成新的语义模式。
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
//  4. 将变更路径与当前模式的源码证据匹配
//  5. 原子保存历史证据
//  6. 标记提交为已分析
//
// 增量学习
//   - 使用 patternRepo.IsCommitAnalyzed() 检查提交是否已分析
//   - 使用 commitTracker.MarkCommitsAnalyzed() 原子标记批次已分析
//
// 历史证据只增强已有模式，不参与规则生成和模式策展。
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

	patterns, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("load patterns for history evidence: %w", err)
	}
	evidenceUpdated := 0
	var batchErrors []error
	totalBatches := (len(unanalyzedCommits) + batchSize - 1) / batchSize

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
			"patterns_count", len(patterns),
		)
		updated := enrichPatternsWithHistory(patterns, commitFiles)
		if len(updated) > 0 {
			save := make([]*domain.Pattern, 0, len(updated))
			for index := range updated {
				save = append(save, &updated[index])
			}
			if saveErr := s.patternRepo.ApplyPatternMutation(ctx, domain.PatternMutation{Save: save}); saveErr != nil {
				logger.Warn(i18n.Get("LoggerLearnerBatchFailed"),
					"operation", "learn",
					"batch_id", i/batchSize+1,
					"error", saveErr,
				)
				batchErrors = append(batchErrors, fmt.Errorf("batch %d save history evidence: %w", i/batchSize+1, saveErr))
				continue
			}
			evidenceUpdated += len(updated)
		}

		// 证据持久化完成后再提交 commit 状态。
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
			"updated_patterns_count", len(updated),
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
		"patterns_enriched", evidenceUpdated,
		"total_batches", totalBatches,
	)

	if len(batchErrors) > 0 {
		return fmt.Errorf("learn history completed with %d failed batch operations: %w", len(batchErrors), errors.Join(batchErrors...))
	}
	return nil
}

func (s *LearnerService) collectCommitFiles(ctx context.Context, commits []domain.CommitInfo, operation string) ([]historyCommit, error) {
	commitFiles := make([]historyCommit, 0, len(commits))
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
		commitFiles = append(commitFiles, historyCommit{
			Commit: c,
			Paths:  files,
		})
	}
	return commitFiles, errors.Join(collectErrors...)
}

func commitInfos(commitFiles []historyCommit) []domain.CommitInfo {
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
	if s.backend == config.LearningBackendLocal {
		return s.enrichSingleCommit(ctx, c, changedFiles)
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
	if s.backend == config.LearningBackendLocal {
		return nil
	}
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

func (s *LearnerService) enrichSingleCommit(ctx context.Context, commit domain.CommitInfo, paths []string) error {
	patterns, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	updated := enrichPatternsWithHistory(patterns, []historyCommit{{Commit: commit, Paths: paths}})
	save := make([]*domain.Pattern, 0, len(updated))
	for index := range updated {
		save = append(save, &updated[index])
	}
	if len(save) == 0 {
		return nil
	}
	return s.patternRepo.ApplyPatternMutation(ctx, domain.PatternMutation{Save: save})
}

// shortHash 安全截取 hash 前 7 位
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
