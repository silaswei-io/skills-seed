// Package patternnorm 提供模式规范化与入库服务。
package patternnorm

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

const (
	// OperationLearnCurrent 表示从当前代码库分析得到候选模式。
	OperationLearnCurrent Operation = "learn_current"
	// OperationUserDefined 表示用户自然语言补充得到候选模式。
	OperationUserDefined Operation = "user_defined"
	// OperationCompact 表示人工触发的模式库整理。
	OperationCompact Operation = "compact"
)

// Operation 标识候选模式进入入库服务的业务来源。
type Operation string

// Valid 报告操作是否具有明确的入库语义。
func (o Operation) Valid() bool {
	switch o {
	case OperationLearnCurrent,
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
)

// NormalizeRequest 表示候选模式规范化入库请求。
type NormalizeRequest struct {
	Operation          Operation
	ProjectName        string
	RootPath           string
	Language           string
	Candidates         []domain.Pattern
	DecisionCheckpoint DecisionCheckpoint
	UserContext        string
}

// DecisionCheckpoint 保存已完成的规范化决策，使本地校验或入库失败后可以直接重放。
type DecisionCheckpoint interface {
	Load(context.Context, string) (*Decision, bool, error)
	Save(context.Context, string, *Decision) error
}

// ProgressHooks 接收规范化入库过程进度事件；为空时服务不写终端输出。
type ProgressHooks struct {
	OnStepStart       func(label string)
	OnStepUpdate      func(label string)
	OnStepComplete    func(label string)
	OnValidationStart func(label string)
	OnStoreStart      func(label string)
}

// NormalizeResult 表示模式规范化入库结果。
type NormalizeResult struct {
	Written []domain.Pattern
	Dropped []Drop
	Summary Summary
}

// Drop 描述一个明确不应入库的候选模式。
type Drop struct {
	ID         string
	ReasonCode DropReasonCode
	Reason     string
}

// Decision 是一次规范化入库的可重放所有权决策。
type Decision struct {
	Patterns []DecisionPattern `json:"patterns"`
	Dropped  []Drop            `json:"dropped"`
}

// DecisionPattern 只记录规范文本和候选来源归属。
type DecisionPattern struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Rule        string   `json:"rule"`
	Confidence  float64  `json:"confidence"`
	SourceIDs   []string `json:"source_ids"`
}

// DropReasonCode 标识候选模式不入库的结构化原因。
type DropReasonCode string

const (
	// DropExactDuplicate 表示候选已被另一个输出完整代表。
	DropExactDuplicate DropReasonCode = "exact_duplicate"
	// DropUnsupportedEvidence 表示候选缺少可验证源码归属。
	DropUnsupportedEvidence DropReasonCode = "unsupported_evidence"
	// DropContradictory 表示候选与输入中的源码或既有知识冲突。
	DropContradictory DropReasonCode = "contradictory"
	// DropUnsafeGuidance 表示候选会把危险历史行为提升为复用建议。
	DropUnsafeGuidance DropReasonCode = "unsafe_guidance"
	// DropNoRouteableValue 表示候选没有未来可路由的实现、验证或维护价值。
	DropNoRouteableValue DropReasonCode = "no_routeable_value"
	// DropLowSignalBoilerplate 表示候选只是无项目决策价值的样板或薄转发。
	DropLowSignalBoilerplate DropReasonCode = "low_signal_boilerplate"
	// DropOverfilteredSourceBacked 表示候选有源码证据但被过度压缩过滤。
	DropOverfilteredSourceBacked DropReasonCode = "overfiltered_source_backed"
)

// Valid 报告丢弃原因码是否属于约定枚举。
func (c DropReasonCode) Valid() bool {
	switch c {
	case DropExactDuplicate,
		DropUnsupportedEvidence,
		DropContradictory,
		DropUnsafeGuidance,
		DropNoRouteableValue,
		DropLowSignalBoilerplate,
		DropOverfilteredSourceBacked:
		return true
	default:
		return false
	}
}

// Summary 描述一次规范化入库的实际输入和输出规模。
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
