package analyzer

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

// FocusAnalysisMode 与 agent 包保持同一组取值，供编排层引用。
type FocusAnalysisMode = agent.FocusAnalysisMode

const (
	FocusAnalysisModeFull  = agent.FocusAnalysisModeFull
	FocusAnalysisModeDelta = agent.FocusAnalysisModeDelta
)

// SelectFocusAnalysisMode 根据是否启用增量 diff 分析返回统一模式枚举。
func SelectFocusAnalysisMode(useDelta bool) FocusAnalysisMode {
	if useDelta {
		return FocusAnalysisModeDelta
	}
	return FocusAnalysisModeFull
}

// AnalyzeCurrentFocusBatchOptions 是编排层调用统一焦点分析的参数。
type AnalyzeCurrentFocusBatchOptions struct {
	Mode              FocusAnalysisMode
	RuntimeLabel      string
	LearningMode      config.LearningMode
	ChangeProfile     string
	RunContext        *CodebaseRunContext
	SharedContextPath string
	// FullFocuses 用于 full 模式。
	FullFocuses []AnalyzeCurrentEvidenceFocus
	// DeltaFocuses 用于 delta 模式。
	DeltaFocuses []AnalyzeCurrentDeltaFocus
}

// AnalyzeCurrentFocusBatchResult 是 analyzer 层统一结果信封。
type AnalyzeCurrentFocusBatchResult struct {
	Mode                      FocusAnalysisMode
	Full                      *AnalyzeCurrentCodebaseBatchResult
	Delta                     *AnalyzeCurrentDeltaBatchResult
	ProfileRefreshRecommended agent.ProfileRefreshRecommendation
}

// AnalyzeCurrentFocusBatch 按 Mode 分发到 full 或 delta 实现，是编排层唯一应调用的焦点分析入口。
func (s *AnalyzerService) AnalyzeCurrentFocusBatch(ctx context.Context, projectRoot, projectName, language string, opts AnalyzeCurrentFocusBatchOptions) (*AnalyzeCurrentFocusBatchResult, error) {
	mode := opts.Mode
	if mode == "" {
		mode = FocusAnalysisModeFull
	}
	out := &AnalyzeCurrentFocusBatchResult{Mode: mode}
	switch mode {
	case FocusAnalysisModeDelta:
		result, err := s.AnalyzeCurrentDeltaBatch(ctx, projectRoot, projectName, language, AnalyzeCurrentDeltaBatchOptions{
			RuntimeLabel:      opts.RuntimeLabel,
			LearningMode:      opts.LearningMode,
			ChangeProfile:     opts.ChangeProfile,
			RunContext:        opts.RunContext,
			SharedContextPath: opts.SharedContextPath,
			Focuses:           opts.DeltaFocuses,
		})
		if err != nil {
			return nil, err
		}
		out.Delta = result
		if result != nil {
			out.ProfileRefreshRecommended = result.ProfileRefreshRecommended
		}
		return out, nil
	default:
		result, err := s.AnalyzeCurrentCodebaseBatch(ctx, projectRoot, projectName, language, AnalyzeCurrentCodebaseBatchOptions{
			RuntimeLabel:      opts.RuntimeLabel,
			LearningMode:      opts.LearningMode,
			ChangeProfile:     opts.ChangeProfile,
			RunContext:        opts.RunContext,
			SharedContextPath: opts.SharedContextPath,
			Focuses:           opts.FullFocuses,
		})
		if err != nil {
			return nil, err
		}
		out.Full = result
		if result != nil {
			for _, focus := range result.Focuses {
				if focus.ProfileRefreshRecommended.Needed {
					out.ProfileRefreshRecommended = focus.ProfileRefreshRecommended
					break
				}
			}
		}
		return out, nil
	}
}

// PatternsFromFocusBatch 从统一结果中提取 full 模式候选模式（delta 返回 nil）。
func PatternsFromFocusBatch(result *AnalyzeCurrentFocusBatchResult) []domain.Pattern {
	if result == nil || result.Full == nil {
		return nil
	}
	patterns := make([]domain.Pattern, 0)
	for _, focus := range result.Full.Focuses {
		patterns = append(patterns, focus.Patterns...)
	}
	return patterns
}
