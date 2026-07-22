// Package curator 提供模式策展与规范入库服务。
package curator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
)

const (
	// OperationLearnHistory 表示从 Git 历史学习得到候选模式。
	OperationLearnHistory Operation = "learn_history"
	// OperationLearnCurrent 表示从当前代码库分析得到候选模式。
	OperationLearnCurrent Operation = "learn_current"
	// OperationLearnCommit 表示从单个提交学习得到候选模式。
	OperationLearnCommit Operation = "learn_commit"
	// OperationLearnStaged 表示从暂存区学习得到候选模式。
	OperationLearnStaged Operation = "learn_staged"
	// OperationUserDefined 表示用户自然语言补充得到候选模式。
	OperationUserDefined Operation = "user_defined"
	// OperationCompact 表示人工触发的模式库整理。
	OperationCompact Operation = "compact"
)

// Operation 标识候选模式进入策展服务的业务来源。
type Operation string

// Valid 报告操作是否具有明确的策展语义。
func (o Operation) Valid() bool {
	switch o {
	case OperationLearnHistory,
		OperationLearnCurrent,
		OperationLearnCommit,
		OperationLearnStaged,
		OperationUserDefined,
		OperationCompact:
		return true
	default:
		return false
	}
}

const (
	// relatedPatternsPerCandidate 控制单个候选模式传给 AI 的相关历史模式上限。
	relatedPatternsPerCandidate = 8
	// deterministicMergeThreshold 是无需 AI 时的保守相似度合并阈值。
	deterministicMergeThreshold = 0.82
)

// CurateRequest 表示候选模式入库请求。
type CurateRequest struct {
	Operation  Operation
	Candidates []domain.Pattern
}

// ProgressHooks 接收策展过程进度事件；为空时服务不写终端输出。
type ProgressHooks struct {
	OnStepStart       func(label string)
	OnStepUpdate      func(label string)
	OnStepComplete    func(label string)
	OnValidationStart func(label string)
	OnStoreStart      func(label string)
}

// CurateResult 表示模式策展入库结果。
type CurateResult struct {
	Written []domain.Pattern
	Dropped []Drop
	Summary Summary
}

// Drop 描述一个明确不应入库的候选模式。
type Drop struct {
	ID     string
	Reason string
}

// Summary 描述一次策展的实际输入和输出规模。
type Summary struct {
	TotalCandidates int
	TotalExisting   int
	TotalWritten    int
	TotalDropped    int
	MergeCount      int
}

// CompactRequest 表示人工整理模式库请求。
type CompactRequest struct {
	Category string
	DryRun   bool
	UseAI    bool
}

// CompactResult 表示人工整理模式库结果。
type CompactResult struct {
	Written []domain.Pattern
	Dropped []Drop
	Summary Summary
}

type retrievalResult struct {
	related             []domain.Pattern
	existingByCandidate map[string][]string
}
