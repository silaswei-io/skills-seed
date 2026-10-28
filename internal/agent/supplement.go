package agent

import (
	"context"
	"errors"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// SupplementInputLimit 限制异常大的自动补充输入，允许独立审查复用完整证据与候选知识。
const SupplementInputLimit = 192 * 1024

// ErrSupplementUnresolved 表示证据不足，不能推进语义学习基线。
var ErrSupplementUnresolved = errors.New("semantic gap remains unresolved")

// LearningSupplement 是本地组批的关联语义问题及完整、有限的只读证据。
type LearningSupplement struct {
	Focus        domain.EvidenceFocus
	Evidence     string
	SourceHashes map[string]string // 本轮实际提供的源码摘要，用于证据有效性判断。
	SourceRanges map[string][]SourceRange
	Symbols      map[string][]SupplementSymbol // CodeGraph 已确认的可调用入口。
}

type SupplementSymbol struct {
	Name      string
	Line      int
	Signature string
}

// SemanticScreenCandidate 只包含 CodeGraph 已确认的紧凑结构事实。
type SemanticScreenCandidate struct {
	Path    string   `json:"path"`
	Symbols []string `json:"symbols"`
	Callers []string `json:"callers,omitempty"`
	Callees []string `json:"callees,omitempty"`
}

type SemanticScreenDecision struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type SemanticScreenRequest struct {
	Candidates []SemanticScreenCandidate `json:"candidates"`
}

type SemanticScreenResult struct {
	Skip []SemanticScreenDecision `json:"skip"`
}

// SemanticScreener 只决定是否省去深度分析，不将筛选判断写成知识。
type SemanticScreener interface {
	ScreenSemanticGaps(context.Context, *SemanticScreenRequest) (*SemanticScreenResult, error)
}

// SourceRange 是本轮实际提供给 Agent 的源码行范围，避免引用未提供的实现。
type SourceRange struct {
	Start int
	End   int
}

type supplementKey struct{}

// WithLearningSupplement 为既有学习链路提供本地计划与事实，避免再次全仓探索。
func WithLearningSupplement(ctx context.Context, input *LearningSupplement) context.Context {
	return context.WithValue(ctx, supplementKey{}, input)
}

// CurrentLearningSupplement 返回受证据边界限制的补充任务。
func CurrentLearningSupplement(ctx context.Context) *LearningSupplement {
	input, _ := ctx.Value(supplementKey{}).(*LearningSupplement)
	return input
}
