// Package learner 提供当前学习候选模式的策展入库编排。
//
// 本包只负责把 learn current 产生的候选模式交给 Curator 规范化并保存。
//
// 服务职责
//   - 候选入库：把 AI 学到的候选模式交给 Curator 规范化入库
//
// 不负责
//   - AI 分析（由 Agent 负责）
//   - 模式策展与持久化（由 Curator 负责）
package learner

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
)

// LearnerService 编排当前学习候选模式的策展和保存。
type LearnerService struct {
	curatorSvc *curator.Service
}

// NewLearnerService 创建学习服务
func NewLearnerService(curatorSvc *curator.Service) *LearnerService {
	return &LearnerService{curatorSvc: curatorSvc}
}

// CurateAndSavePatterns 策展并保存多个候选模式。
func (s *LearnerService) CurateAndSavePatterns(ctx context.Context, patterns []domain.Pattern, operation curator.Operation) (int, error) {
	return s.curateAndSavePatterns(ctx, patterns, operation, CurateOptions{})
}

// CurateOptions 描述一次候选模式策展的附加执行能力。
type CurateOptions struct {
	Hooks              curator.ProgressHooks
	DecisionCheckpoint curator.DecisionCheckpoint
	LearningSession    agent.LearningSession
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
		LearningSession:    opts.LearningSession,
	}, opts.Hooks)
	if err != nil {
		return 0, err
	}
	return len(result.Written), nil
}
