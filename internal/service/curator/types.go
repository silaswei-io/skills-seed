// Package curator 提供模式策展与规范入库服务。
package curator

import (
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

const (
	// OperationLearnHistory 表示从 Git 历史学习得到候选模式。
	OperationLearnHistory = "learn_history"
	// OperationLearnCurrent 表示从当前代码库分析得到候选模式。
	OperationLearnCurrent = "learn_current"
	// OperationLearnCommit 表示从单个提交学习得到候选模式。
	OperationLearnCommit = "learn_commit"
	// OperationLearnStaged 表示从暂存区学习得到候选模式。
	OperationLearnStaged = "learn_staged"
	// OperationUserDefined 表示用户自然语言补充得到候选模式。
	OperationUserDefined = "user_defined"
	// OperationCompact 表示人工触发的模式库整理。
	OperationCompact = "compact"
)

const (
	// relatedPatternsPerCandidate 控制单个候选模式传给 AI 的相关历史模式上限。
	relatedPatternsPerCandidate = 8
	// deterministicMergeThreshold 是无需 AI 时的保守相似度合并阈值。
	deterministicMergeThreshold = 0.82
)

// CurateRequest 表示候选模式入库请求。
type CurateRequest struct {
	Operation  string
	Candidates []domain.Pattern
}

// ProgressHooks 接收策展过程进度事件；为空时服务不写终端输出。
type ProgressHooks struct {
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

// CurateResult 表示模式策展入库结果。
type CurateResult struct {
	Written []domain.Pattern
	Dropped []agent.CuratedDrop
	Summary agent.CurateSummary
}

// CompactRequest 表示人工整理模式库请求。
type CompactRequest struct {
	Category string
	DryRun   bool
}

// CompactResult 表示人工整理模式库结果。
type CompactResult struct {
	Written []domain.Pattern
	Dropped []agent.CuratedDrop
	Summary agent.CurateSummary
}

type retrievalResult struct {
	related             []domain.Pattern
	existingByCandidate map[string][]string
}
