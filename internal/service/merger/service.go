// Package merger 提供模式合并服务
//
// 本包实现模式合并和去重功能
//   - MergePatterns: 使用 AI 合并相似模式
//   - applyMerge: 应用合并结果到数据库
//
// 合并策略
//   - 支持 dry-run 模式（只预览不执行）
//   - 删除被合并的旧模式
//   - 保存合并后的新模式
//   - 记录合并来源（MergedFrom）
package merger

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
)

// MergerService 模式合并服务
// 职责：合并相似模式，去重，优化模式库
type MergerService struct {
	agent       agent.Agent
	patternRepo domain.PatternRepository
}

// NewMergerService 创建合并服务
func NewMergerService(ag agent.Agent, repo domain.PatternRepository) *MergerService {
	return &MergerService{
		agent:       ag,
		patternRepo: repo,
	}
}

// MergePatternsRequest 合并模式请求
type MergePatternsRequest struct {
	Category string // 要合并的分类（空 = 所有分类）
	DryRun   bool   // 是否只预览不执行
}

// MergePatternsResult 合并模式结果
type MergePatternsResult struct {
	MergedPatterns    []agent.MergedPattern
	UnchangedPatterns []agent.UnchangedPattern
	Summary           agent.MergeSummary
}

// MergePatterns 合并相似模式
// 使用 AI 分析相似模式并合并，减少冗余
//
// 参数
//   - ctx: 上下文
//   - req: 合并请求（Category: 要合并的分类，空 = 所有分类；DryRun: 是否只预览不执行）
//
// 返回
//   - MergedPatterns: 合并后的新模式
//   - UnchangedPatterns: 未改变的模式
//   - Summary: 合并统计信息
//
// 工作流程
//  1. 获取指定分类或所有模式
//  2. 调用 AI 分析并合并相似模式
//  3. 如果是 dry-run，只返回结果不执行
//  4. 否则删除被合并的旧模式，保存新模式
func (s *MergerService) MergePatterns(ctx context.Context, req *MergePatternsRequest) (*MergePatternsResult, error) {
	if req == nil {
		req = &MergePatternsRequest{}
	}
	logger.Info(i18n.Get("LoggerMergerStart"),
		"operation", "merge_patterns",
		"category", req.Category,
		"dry_run", req.DryRun,
	)

	// 1. 获取要合并的模式
	var patterns []domain.Pattern
	var err error

	if req.Category != "" {
		patterns, err = s.patternRepo.GetByCategory(ctx, domain.Category(req.Category))
	} else {
		patterns, err = s.patternRepo.GetAll(ctx)
	}

	if err != nil {
		return nil, domain.NewDomainError(
			domain.ErrInternal,
			"获取模式失败",
			err,
		)
	}

	if len(patterns) == 0 {
		logger.Info(i18n.Get("LoggerMergerNoPatterns"),
			"operation", "merge_patterns",
		)
		return &MergePatternsResult{
			Summary: agent.MergeSummary{
				TotalInput:     0,
				TotalMerged:    0,
				TotalUnchanged: 0,
				MergeCount:     0,
			},
		}, nil
	}

	logger.Info(i18n.Get("LoggerMergerFoundPatterns"),
		"operation", "merge_patterns",
		"count", len(patterns),
	)

	// 2. 转换为 agent 格式

	// 3. 调用 AI 合并
	agentReq := &agent.MergePatternsRequest{
		Category: req.Category,
		Patterns: patterns,
	}

	var result *agent.MergePatternsResult
	mergeProgress := progress.New(1)
	// AI 合并需要读取所有候选模式，数据量大时耗时明显，用进度行显示当前阶段
	err = mergeProgress.RunStep(i18n.Get("ProgressMergePatternsAI"), func() error {
		var callErr error
		result, callErr = s.agent.MergePatterns(ctx, agentReq)
		return callErr
	})
	if err != nil {
		return nil, domain.NewDomainError(
			domain.ErrAIService,
			"AI 合并失败",
			err,
		)
	}

	logger.Info(i18n.Get("LoggerMergerAIComplete"),
		"operation", "merge_patterns",
		"merged_count", len(result.MergedPatterns),
		"unchanged_count", len(result.UnchangedPatterns),
	)

	// 4. 如果是 dry-run，直接返回结果
	if req.DryRun {
		logger.Info(i18n.Get("LoggerMergerDryRun"),
			"operation", "merge_patterns",
		)
		return &MergePatternsResult{
			MergedPatterns:    result.MergedPatterns,
			UnchangedPatterns: result.UnchangedPatterns,
			Summary:           result.Summary,
		}, nil
	}

	// 5. 执行实际合并
	if err := s.applyMerge(ctx, result, patterns); err != nil {
		return nil, err
	}

	logger.Info(i18n.Get("LoggerMergerComplete"),
		"operation", "merge_patterns",
		"total_input", result.Summary.TotalInput,
		"total_merged", result.Summary.TotalMerged,
		"total_unchanged", result.Summary.TotalUnchanged,
	)

	return &MergePatternsResult{
		MergedPatterns:    result.MergedPatterns,
		UnchangedPatterns: result.UnchangedPatterns,
		Summary:           result.Summary,
	}, nil
}

// applyMerge 应用合并结果
func (s *MergerService) applyMerge(ctx context.Context, result *agent.MergePatternsResult, originalPatterns []domain.Pattern) error {
	// 1. 标记被合并的模式
	mergedIDs := make(map[string]bool)
	for _, merged := range result.MergedPatterns {
		for _, sourceID := range merged.MergedFrom {
			mergedIDs[sourceID] = true
		}
	}

	// 2. 删除被合并的旧模式
	for _, pattern := range originalPatterns {
		if mergedIDs[pattern.ID] {
			if err := s.patternRepo.Delete(ctx, pattern.ID); err != nil {
				logger.Warn(i18n.Get("LoggerMergerDeleteOldFailed"),
					"operation", "merge_patterns",
					"pattern_id", pattern.ID,
					"error", err,
				)
				// 继续处理，不中断
			}
		}
	}

	// 3. 保存合并后的新模式
	for _, mergedPattern := range result.MergedPatterns {
		newPattern := domain.NewPattern(
			mergedPattern.ID,
			mergedPattern.Name,
			domain.Category(mergedPattern.Category),
		)
		newPattern.SetDescription(mergedPattern.Description)
		newPattern.SetRule(mergedPattern.Rule)
		newPattern.Confidence = mergedPattern.Confidence
		newPattern.Merged = true
		newPattern.MergedFrom = mergedPattern.MergedFrom

		if err := s.patternRepo.Save(ctx, newPattern); err != nil {
			logger.Warn(i18n.Get("LoggerMergerSaveMergedFailed"),
				"operation", "merge_patterns",
				"pattern_id", newPattern.ID,
				"error", err,
			)
			// 继续处理，不中断
		} else {
			logger.Info(i18n.Get("LoggerMergerPatternSaved"),
				"operation", "merge_patterns",
				"pattern_name", newPattern.Name,
				"merged_from", len(newPattern.MergedFrom),
			)
		}
	}

	return nil
}
