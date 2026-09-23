// Package domain 提供核心领域模型和业务规则。
package domain

import (
	"strconv"
	"strings"
	"time"
)

// ==================== 模式 ====================

// Category 模式分类
type Category string

const (
	CategoryNaming      Category = "naming"      // 命名模式
	CategoryError       Category = "error"       // 错误处理
	CategoryStructure   Category = "structure"   // 代码结构
	CategoryConcurrency Category = "concurrency" // 并发模式
	CategoryBusiness    Category = "business"    // 产品/领域行为模式
	CategoryAPI         Category = "api"         // 接口/合约模式
	CategoryDatabase    Category = "database"    // 状态/存储模式
	CategoryUtils       Category = "utils"       // 工具方法模式
	CategoryMiddleware  Category = "middleware"  // 拦截/管道模式
	CategoryConfig      Category = "config"      // 配置管理模式
)

// allowedPatternCategories 定义模式库支持的规范分类和展示顺序。
var allowedPatternCategories = []Category{
	CategoryNaming,
	CategoryError,
	CategoryStructure,
	CategoryConcurrency,
	CategoryBusiness,
	CategoryAPI,
	CategoryDatabase,
	CategoryUtils,
	CategoryMiddleware,
	CategoryConfig,
}

// AllowedPatternCategoryNames 返回稳定顺序的合法模式分类名。
func AllowedPatternCategoryNames() []string {
	names := make([]string, 0, len(allowedPatternCategories))
	for _, category := range allowedPatternCategories {
		names = append(names, string(category))
	}
	return names
}

// AllowedPatternCategoriesText 返回提示词可直接展示的合法分类列表。
func AllowedPatternCategoriesText() string {
	descriptions := make([]string, 0, len(allowedPatternCategories))
	for _, category := range allowedPatternCategories {
		descriptions = append(descriptions, patternCategoryPromptDescription(category))
	}
	return strings.Join(descriptions, "; ")
}

func patternCategoryPromptDescription(category Category) string {
	switch category {
	case CategoryNaming:
		return "naming (naming conventions)"
	case CategoryError:
		return "error (failure semantics, error taxonomy, recovery behavior)"
	case CategoryStructure:
		return "structure (ownership, module, component, generated-source, or extension boundaries)"
	case CategoryConcurrency:
		return "concurrency (parallelism, lifecycle synchronization, consistency windows)"
	case CategoryBusiness:
		return "business (product/domain behavior, user or system actions, policies, state transitions)"
	case CategoryAPI:
		return "api (interfaces, contracts, messages, events, adapters, generated artifacts)"
	case CategoryDatabase:
		return "database (state, storage, persistence, schema, migration, cache boundaries)"
	case CategoryUtils:
		return "utils (reusable domain-neutral helpers)"
	case CategoryMiddleware:
		return "middleware (interception, pipelines, filters, hooks, request/event processing)"
	case CategoryConfig:
		return "config (configuration, feature gates, runtime/deployment settings)"
	default:
		return string(category)
	}
}

// NormalizePatternCategory 规范化分类键的大小写和空白。
func NormalizePatternCategory(category Category) Category {
	return canonicalPatternCategory(category)
}

// IsValidPatternCategory 判断分类是否属于内部规范分类集合。
func IsValidPatternCategory(category Category) bool {
	category = canonicalPatternCategory(category)
	for _, allowed := range allowedPatternCategories {
		if category == allowed {
			return true
		}
	}
	return false
}

func canonicalPatternCategory(category Category) Category {
	return Category(strings.ToLower(strings.TrimSpace(string(category))))
}

// Source 模式来源
type Source string

const (
	SourceLearned        Source = "learned"         // 从早期学习入口写入的历史来源
	SourceLearnedCurrent Source = "learned_current" // learn current
	SourceUserDefined    Source = "user_defined"    // 用户自定义 patterns add
	SourceDefault        Source = "default"         // 默认规则
	SourceInit           Source = "init"            // 从初始代码库分析
)

// CodeLocationStatus 表示一段代码位置元数据的当前可信状态。
type CodeLocationStatus string

const (
	CodeLocationStatusUnknown   CodeLocationStatus = "unknown"
	CodeLocationStatusValid     CodeLocationStatus = "valid"
	CodeLocationStatusMoved     CodeLocationStatus = "moved"
	CodeLocationStatusChanged   CodeLocationStatus = "changed"
	CodeLocationStatusMissing   CodeLocationStatus = "missing"
	CodeLocationStatusAmbiguous CodeLocationStatus = "ambiguous"
)

// CodeLocationChangeKind 表示位置刷新时识别出的变化类型。
type CodeLocationChangeKind string

const (
	CodeLocationChangeMoved               CodeLocationChangeKind = "moved"
	CodeLocationChangeSignatureChanged    CodeLocationChangeKind = "signature_changed"
	CodeLocationChangeInputsChanged       CodeLocationChangeKind = "inputs_changed"
	CodeLocationChangeOutputsChanged      CodeLocationChangeKind = "outputs_changed"
	CodeLocationChangeBodyChanged         CodeLocationChangeKind = "body_changed"
	CodeLocationChangeDependenciesChanged CodeLocationChangeKind = "dependencies_changed"
)

// SymbolSnapshot 保存语言无关的代码符号快照。
// tree-sitter 刷新位置时可以把 Go/TS/Python/Java 等语言的符号统一写入这些字段。
type SymbolSnapshot struct {
	Language          string   `json:"language,omitempty"`
	Kind              string   `json:"kind,omitempty"`
	Namespace         string   `json:"namespace,omitempty"`
	Receiver          string   `json:"receiver,omitempty"`
	Name              string   `json:"name,omitempty"`
	Signature         string   `json:"signature,omitempty"`
	SignatureHash     string   `json:"signature_hash,omitempty"`
	InputTypes        []string `json:"input_types,omitempty"`
	InputHashes       []string `json:"input_hashes,omitempty"`
	OutputTypes       []string `json:"output_types,omitempty"`
	OutputHashes      []string `json:"output_hashes,omitempty"`
	BodyHash          string   `json:"body_hash,omitempty"`
	DependencySymbols []string `json:"dependency_symbols,omitempty"`
}

// CodeLocationHistory 保存一次位置或快照变化记录。
type CodeLocationHistory struct {
	Location    string                   `json:"location,omitempty"`
	Status      CodeLocationStatus       `json:"status,omitempty"`
	ChangeKinds []CodeLocationChangeKind `json:"change_kinds,omitempty"`
	Snapshot    *SymbolSnapshot          `json:"snapshot,omitempty"`
	ChangedAt   time.Time                `json:"changed_at,omitempty"`
	Note        string                   `json:"note,omitempty"`
}

// CodeLocation 保存能力入口或工具函数的可维护代码位置元数据。
type CodeLocation struct {
	HistoricalLocation string                   `json:"historical_location,omitempty"`
	CurrentLocation    string                   `json:"current_location,omitempty"`
	Status             CodeLocationStatus       `json:"status,omitempty"`
	ChangeKinds        []CodeLocationChangeKind `json:"change_kinds,omitempty"`
	Confidence         float64                  `json:"confidence,omitempty"`
	VerifiedAt         time.Time                `json:"verified_at,omitempty"`
	CreatedAt          time.Time                `json:"created_at,omitempty"`
	UpdatedAt          time.Time                `json:"updated_at,omitempty"`
	Snapshot           *SymbolSnapshot          `json:"snapshot,omitempty"`
	History            []CodeLocationHistory    `json:"history,omitempty"`
}

// BusinessMethod 描述可复用能力入口。
// 名称沿用历史模型，语义覆盖产品/领域行为、交互动作、任务、适配器和工具入口。
type BusinessMethod struct {
	Name          string       // 入口名称（如 GenerateUUID, PublishAction）
	CodeLocation  CodeLocation `json:"code_location,omitempty"` // 可维护的位置元数据
	Description   string       // 功能说明（1-2句话）
	Usage         string       // 使用场景（何时使用）
	Type          string       // 方法类型：domain（领域特定）| common（通用）
	Function      string       // 完整的方法签名（如 func (s *Service) Method(ctx, req) (resp, error)）
	Prerequisites string       // 调用前需要的上下文、初始化状态或外部依赖
	Returns       string       // 返回值说明（包括可能的错误情况）
}

// PatternMetrics 描述一条模式的质量和可排序性。
type PatternMetrics struct {
	SpecificityScore float64 // 项目特有性，0.0-1.0
	EvidenceCount    int     // 代码路径、方法签名、项目符号等证据数量
	GenericPenalty   float64 // 泛化/模板化惩罚，0.0-1.0
	EffectiveScore   float64 // 综合排序分，0.0-1.0
}

// PatternStatus 表示模式在当前代码库中的生命周期状态。
type PatternStatus string

const (
	PatternStatusActive     PatternStatus = "active"
	PatternStatusStale      PatternStatus = "stale"
	PatternStatusSuperseded PatternStatus = "superseded"
	PatternStatusDeprecated PatternStatus = "deprecated"
)

// NormalizePatternStatus 归一化模式生命周期状态。
func NormalizePatternStatus(status PatternStatus) PatternStatus {
	switch PatternStatus(strings.ToLower(strings.TrimSpace(string(status)))) {
	case PatternStatusStale:
		return PatternStatusStale
	case PatternStatusSuperseded:
		return PatternStatusSuperseded
	case PatternStatusDeprecated:
		return PatternStatusDeprecated
	default:
		return PatternStatusActive
	}
}

// PatternEvidenceLocation 保存一条模式的源码证据位置。
type PatternEvidenceLocation struct {
	Path        string  `json:"path,omitempty"`        // 相对项目根路径
	Line        int     `json:"line,omitempty"`        // 1-based 行号
	Symbol      string  `json:"symbol,omitempty"`      // 相关函数、方法、类型或变量名
	Kind        string  `json:"kind,omitempty"`        // 证据类型，如 function/method/file
	Description string  `json:"description,omitempty"` // 证据说明
	Confidence  float64 `json:"confidence,omitempty"`  // 证据位置置信度，0.0-1.0
}

// PatternDiffAnchor 描述 learn current 增量候选由哪段变更触发。
// 该字段仅用于本次分析验收，不属于长期模式库事实。
type PatternDiffAnchor struct {
	Path        string `json:"path,omitempty"`        // 相对项目根路径
	Line        int    `json:"line,omitempty"`        // 变更后文件中的 1-based 行号；删除文件可为空
	Symbol      string `json:"symbol,omitempty"`      // 变更触达的函数、方法、类型或配置键
	ChangeKind  string `json:"change_kind,omitempty"` // added、modified、deleted
	Description string `json:"description,omitempty"` // 该变更为何触发此候选模式
}

// DisplayLocation 返回适合 CLI 展示的证据位置。
func (l PatternEvidenceLocation) DisplayLocation() string {
	path := strings.TrimSpace(l.Path)
	if path == "" {
		return ""
	}
	if l.Line > 0 {
		return path + ":" + strconv.Itoa(l.Line)
	}
	return path
}

// Pattern 代码模式聚合根
type Pattern struct {
	ID             string
	Name           string
	Category       Category
	Description    string
	GoodExample    string
	BadExample     string
	Rule           string
	Confidence     float64
	Frequency      int
	Metrics        PatternMetrics
	Source         Source
	Merged         bool            // 是否已被汇总
	MergedFrom     []string        // 从哪些模式ID汇总而来
	Generated      bool            // 是否已生成到 skills
	BusinessMethod *BusinessMethod // 能力入口信息（可选，用于可复用入口定位）
	KnowledgeFlags []string        `json:"knowledge_flags,omitempty"` // 经证据审查的受控知识标志
	// DevelopmentFocus 复用学习阶段已审查的证据焦点，用于生成稳定的开发导航入口。
	DevelopmentFocus *DevelopmentFocus `json:"development_focus,omitempty"`
	// EvidenceLocations 是模式对应的通用源码证据位置，不等同于 BusinessMethod 的可调用位置。
	EvidenceLocations []PatternEvidenceLocation `json:"evidence_locations,omitempty"`
	ProjectID         string                    `json:"project_id,omitempty"`     // workspace 模式下的子项目 ID
	ScopePath         string                    `json:"scope_path,omitempty"`     // workspace 模式下的路径范围
	WorkspaceRole     string                    `json:"workspace_role,omitempty"` // workspace 子项目角色，如 app、library、service、shared 等
	DiffAnchors       []PatternDiffAnchor       `json:"-"`                        // 仅用于增量学习候选验收，禁止持久化
	Status            PatternStatus             `json:"status,omitempty"`
	LastSeenAt        time.Time                 `json:"last_seen_at,omitempty"`
	StaleReason       string                    `json:"stale_reason,omitempty"`
	SupersededBy      string                    `json:"superseded_by,omitempty"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"` // 最后更新时间
}

// NewPattern 创建新的模式
func NewPattern(id, name string, category Category) *Pattern {
	now := time.Now()
	return &Pattern{
		ID:         id,
		Name:       name,
		Category:   category,
		Confidence: 0.0,
		Frequency:  0,
		Source:     SourceLearned,
		Merged:     false,
		MergedFrom: []string{},
		Generated:  false,
		Status:     PatternStatusActive,
		LastSeenAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// SetBusinessMethod 设置能力入口信息。
func (p *Pattern) SetBusinessMethod(method *BusinessMethod) {
	p.BusinessMethod = method
	p.UpdatedAt = time.Now()
	p.RefreshMetrics()
}

// NormalizeForSave 补齐持久化时需要稳定保存的字段。
func (p *Pattern) NormalizeForSave(previous *Pattern, now time.Time) {
	p.KnowledgeFlags = CanonicalKnowledgeFlags(p.KnowledgeFlags)
	p.Status = NormalizePatternStatus(p.Status)
	p.DevelopmentFocus = p.DevelopmentFocus.Clone()
	// DiffAnchors 仅服务增量学习验收，禁止进入长期模式库。
	p.DiffAnchors = nil
	if previous != nil && !previous.CreatedAt.IsZero() {
		p.CreatedAt = previous.CreatedAt
	} else if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.LastSeenAt.IsZero() {
		if previous != nil && !previous.LastSeenAt.IsZero() {
			p.LastSeenAt = previous.LastSeenAt
		} else {
			p.LastSeenAt = now
		}
	}
	p.UpdatedAt = now
	if p.BusinessMethod != nil {
		var previousMethod *BusinessMethod
		if previous != nil {
			previousMethod = previous.BusinessMethod
		}
		p.BusinessMethod.NormalizeCodeLocation(previousMethod, now)
	}
}

// NormalizeAfterLoad 补齐旧 DB 记录缺失的派生字段，不改变更新时间。
func (p *Pattern) NormalizeAfterLoad() {
	p.KnowledgeFlags = CanonicalKnowledgeFlags(p.KnowledgeFlags)
	p.Status = NormalizePatternStatus(p.Status)
	p.DevelopmentFocus = p.DevelopmentFocus.Clone()
	if p.LastSeenAt.IsZero() {
		p.LastSeenAt = p.UpdatedAt
		if p.LastSeenAt.IsZero() {
			p.LastSeenAt = p.CreatedAt
		}
	}
	if p.BusinessMethod != nil {
		p.BusinessMethod.NormalizeCodeLocation(nil, time.Time{})
	}
}

// IsActive 判断模式是否应参与 check 和 generate 等默认消费流程。
func (p Pattern) IsActive() bool {
	return NormalizePatternStatus(p.Status) == PatternStatusActive
}

// CanBeRetiredFromCurrentLearning 判断模式是否允许由当前代码增量学习自动删除。
// 用户定义和默认模式属于显式约束，不能由模型根据局部 diff 自动撤销。
func (p Pattern) CanBeRetiredFromCurrentLearning() bool {
	switch p.Source {
	case SourceLearned, SourceLearnedCurrent, SourceInit:
		return true
	default:
		return false
	}
}

// NormalizeCodeLocation 规范化能力入口的结构化代码位置。
func (m *BusinessMethod) NormalizeCodeLocation(previous *BusinessMethod, now time.Time) {
	if m == nil {
		return
	}

	previousLocation := CodeLocation{}
	if previous != nil {
		previousLocation = previous.CodeLocation
	}

	location := strings.TrimSpace(m.CodeLocation.CurrentLocation)
	if location == "" {
		location = strings.TrimSpace(m.CodeLocation.HistoricalLocation)
	}
	if location == "" {
		location = strings.TrimSpace(previousLocation.CurrentLocation)
	}

	if m.CodeLocation.HistoricalLocation == "" {
		if previousLocation.HistoricalLocation != "" {
			m.CodeLocation.HistoricalLocation = previousLocation.HistoricalLocation
		} else {
			m.CodeLocation.HistoricalLocation = location
		}
	}
	if m.CodeLocation.CurrentLocation == "" {
		m.CodeLocation.CurrentLocation = location
	}
	if m.CodeLocation.Status == "" {
		if previousLocation.Status != "" {
			m.CodeLocation.Status = previousLocation.Status
		} else if location != "" {
			m.CodeLocation.Status = CodeLocationStatusValid
		} else {
			m.CodeLocation.Status = CodeLocationStatusUnknown
		}
	}
	if !previousLocation.CreatedAt.IsZero() {
		m.CodeLocation.CreatedAt = previousLocation.CreatedAt
	} else if m.CodeLocation.CreatedAt.IsZero() && !now.IsZero() {
		m.CodeLocation.CreatedAt = now
	}
	if !now.IsZero() {
		m.CodeLocation.UpdatedAt = now
	}
}

// DisplayLocation 返回模板和命令展示时优先使用的当前位置。
func (m BusinessMethod) DisplayLocation() string {
	if m.CodeLocation.CurrentLocation != "" {
		return m.CodeLocation.CurrentLocation
	}
	return m.CodeLocation.HistoricalLocation
}

// HistoricalDisplayLocation 返回与当前位置不同的历史位置。
func (m BusinessMethod) HistoricalDisplayLocation() string {
	historical := m.CodeLocation.HistoricalLocation
	if historical == "" || historical == m.DisplayLocation() {
		return ""
	}
	return historical
}

// LocationStatus 返回位置状态字符串。
func (m BusinessMethod) LocationStatus() string {
	if m.CodeLocation.Status != "" {
		return string(m.CodeLocation.Status)
	}
	if m.DisplayLocation() != "" {
		return string(CodeLocationStatusUnknown)
	}
	return ""
}

// IsValid 验证模式是否有效
func (p *Pattern) IsValid() bool {
	return strings.TrimSpace(p.ID) != "" &&
		strings.TrimSpace(p.Name) != "" &&
		strings.TrimSpace(p.Rule) != "" &&
		IsValidPatternCategory(p.Category) &&
		ValidKnowledgeFlags(p.KnowledgeFlags) &&
		p.Confidence >= 0.0 &&
		p.Confidence <= 1.0
}

// UpdateConfidence 更新置信度（基于频率加权平均）
func (p *Pattern) UpdateConfidence(newConfidence float64) {
	p.Confidence = (p.Confidence*float64(p.Frequency) + newConfidence) / float64(p.Frequency+1)
	p.Frequency++
	p.RefreshMetrics()
}

// SetExamples 设置示例
func (p *Pattern) SetExamples(good, bad string) {
	p.GoodExample = good
	p.BadExample = bad
}

// SetDescription 设置描述
func (p *Pattern) SetDescription(desc string) {
	p.Description = desc
}

// SetRule 设置规则
func (p *Pattern) SetRule(rule string) {
	p.Rule = rule
}

// Merge 合并另一个模式到当前模式
// 会合并示例、更新置信度和频率
func (p *Pattern) Merge(other *Pattern) {
	p.KnowledgeFlags = MergeKnowledgeFlags(p.KnowledgeFlags, other.KnowledgeFlags)
	// 如果当前模式没有示例，使用另一个模式的示例
	if p.GoodExample == "" && other.GoodExample != "" {
		p.GoodExample = other.GoodExample
	}
	if p.BadExample == "" && other.BadExample != "" {
		p.BadExample = other.BadExample
	}

	// 如果当前模式的描述为空，使用另一个模式的描述
	if p.Description == "" && other.Description != "" {
		p.Description = other.Description
	}

	// 使用加权平均更新置信度
	// 新置信度 = (当前置信度 * 当前频率 + 新置信度 * 新频率) / (当前频率 + 新频率)
	totalFrequency := p.Frequency + other.Frequency
	if totalFrequency > 0 {
		p.Confidence = (p.Confidence*float64(p.Frequency) + other.Confidence*float64(other.Frequency)) / float64(totalFrequency)
	}
	p.Frequency = totalFrequency
	p.RefreshMetrics()
}

// RefreshMetrics 只依据可确定验证的证据量和置信度刷新质量指标。
func (p *Pattern) RefreshMetrics() {
	evidence := p.evidenceCount()
	specificity := 0.0
	if evidence > 0 {
		specificity = float64(evidence) / float64(evidence+1)
	}
	effective := clamp01((specificity + clamp01(p.Confidence)) / 2)

	p.Metrics = PatternMetrics{
		SpecificityScore: roundScore(specificity),
		EvidenceCount:    evidence,
		GenericPenalty:   0,
		EffectiveScore:   roundScore(effective),
	}
	p.UpdatedAt = time.Now()
}

func (p *Pattern) evidenceCount() int {
	return PatternEvidenceFileCount(p.EvidenceLocations)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func roundScore(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// IsSimilar 判断两个模式是否相似
// 通过名称和分类来判断
func (p *Pattern) IsSimilar(other *Pattern) bool {
	return p.Name == other.Name &&
		p.Category == other.Category &&
		p.ProjectID == other.ProjectID &&
		p.ScopePath == other.ScopePath
}
