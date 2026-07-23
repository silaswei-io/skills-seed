// Package domain 提供核心领域模型和业务规则
//
// 本包定义了 skills-seed 项目的核心领域对象
//   - Pattern: 代码模式聚合根
//   - Category: 模式分类
//   - Source: 模式来源
//   - CommitInfo: Git 提交信息
//   - FileInfo: 文件信息
//
// 领域对象以稳定数据结构和基础行为为主，应用层策略和默认值不放在这里
package domain

import (
	"strconv"
	"strings"
	"time"
)

const (
	// ModeProject 表示把初始化根目录作为单个项目处理
	ModeProject = "project"
	// ModeWorkspace 表示把初始化根目录作为包含多个子项目的工作区处理
	ModeWorkspace = "workspace"
)

// ==================== 模式 ====================

// Category 模式分类
type Category string

const (
	CategoryNaming      Category = "naming"      // 命名模式
	CategoryError       Category = "error"       // 错误处理
	CategoryStructure   Category = "structure"   // 代码结构
	CategoryConcurrency Category = "concurrency" // 并发模式
	CategoryTesting     Category = "testing"     // 测试模式
	CategoryBusiness    Category = "business"    // 业务逻辑模式
	CategoryAPI         Category = "api"         // API设计模式
	CategoryDatabase    Category = "database"    // 数据库操作模式
	CategoryUtils       Category = "utils"       // 工具方法模式
	CategoryMiddleware  Category = "middleware"  // 中间件模式
	CategoryConfig      Category = "config"      // 配置管理模式
)

// allowedPatternCategories 定义模式库支持的规范分类和展示顺序。
var allowedPatternCategories = []Category{
	CategoryNaming,
	CategoryError,
	CategoryStructure,
	CategoryConcurrency,
	CategoryTesting,
	CategoryBusiness,
	CategoryAPI,
	CategoryDatabase,
	CategoryUtils,
	CategoryMiddleware,
	CategoryConfig,
}

// patternCategoryAliases 记录历史或模型常见输出到规范分类的兼容映射。
var patternCategoryAliases = map[Category]Category{
	Category("security"):           CategoryUtils,
	Category("security-hardening"): CategoryUtils,
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
	return strings.Join(AllowedPatternCategoryNames(), ", ")
}

// NormalizePatternCategory 把兼容别名归一化为内部规范分类。
func NormalizePatternCategory(category Category) Category {
	normalized := canonicalPatternCategory(category)
	if alias, ok := patternCategoryAliases[normalized]; ok {
		return alias
	}
	return normalized
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
	SourceLearned        Source = "learned"         // 从 commit 历史学习（向后兼容）
	SourceLearnedHistory Source = "learned_history" // learn history
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

// CodeLocation 保存业务方法或工具函数的可维护代码位置元数据。
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

// BusinessMethod 业务方法信息
type BusinessMethod struct {
	Name          string       // 方法名称（如 GenerateUUID, CallUserRPC）
	CodeLocation  CodeLocation `json:"code_location,omitempty"` // 可维护的位置元数据
	Description   string       // 功能说明（1-2句话）
	Usage         string       // 使用场景（何时使用）
	Type          string       // 方法类型：domain（领域特定）| common（通用）
	Function      string       // 完整的方法签名（如 func (s *Service) Method(ctx, req) (resp, error)）
	Prerequisites string       // 调用前需要的设置或依赖（如 context, 已初始化的 service）
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
	BusinessMethod *BusinessMethod // 业务方法信息（可选，仅用于 utils 和 business 分类）
	// EvidenceLocations 是模式对应的通用源码证据位置，不等同于 BusinessMethod 的可调用位置。
	EvidenceLocations []PatternEvidenceLocation `json:"evidence_locations,omitempty"`
	ProjectID         string                    `json:"project_id,omitempty"`     // workspace 模式下的子项目 ID
	ScopePath         string                    `json:"scope_path,omitempty"`     // workspace 模式下的路径范围
	WorkspaceRole     string                    `json:"workspace_role,omitempty"` // frontend/backend/middleware/shared 等
	AnalysisUnitID    string                    `json:"analysis_unit_id,omitempty"`
	AnalysisUnitName  string                    `json:"analysis_unit_name,omitempty"`
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

// SetBusinessMethod 设置业务方法信息
func (p *Pattern) SetBusinessMethod(method *BusinessMethod) {
	p.BusinessMethod = method
	p.UpdatedAt = time.Now()
	p.RefreshMetrics()
}

// NormalizeForSave 补齐持久化时需要稳定保存的字段。
func (p *Pattern) NormalizeForSave(previous *Pattern, now time.Time) {
	p.Status = NormalizePatternStatus(p.Status)
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
	p.Status = NormalizePatternStatus(p.Status)
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

// NormalizeCodeLocation 规范化业务方法的结构化代码位置。
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

// RefreshMetrics 使用确定性启发式刷新模式质量指标。
func (p *Pattern) RefreshMetrics() {
	evidence := p.evidenceCount()
	genericPenalty := p.genericPenalty()
	specificity := clamp01(float64(evidence)/6.0 + categorySpecificityBonus(p) - genericPenalty*0.35)
	effective := clamp01(specificity*0.6 + p.Confidence*0.3 - genericPenalty*0.1)

	p.Metrics = PatternMetrics{
		SpecificityScore: roundScore(specificity),
		EvidenceCount:    evidence,
		GenericPenalty:   roundScore(genericPenalty),
		EffectiveScore:   roundScore(effective),
	}
	p.UpdatedAt = time.Now()
}

func (p *Pattern) evidenceCount() int {
	return PatternEvidenceFileCount(p.EvidenceLocations)
}

func (p *Pattern) genericPenalty() float64 {
	text := strings.ToLower(strings.Join([]string{p.Name, p.Description, p.Rule}, " "))
	terms := []string{
		"best practice", "clean architecture", "layered architecture", "repository pattern",
		"最佳实践", "分层架构", "注意错误处理", "代码规范", "保持一致", "遵守规范", "合理命名",
	}
	hits := 0
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			hits++
		}
	}
	if len([]rune(strings.TrimSpace(p.Description))) < 24 {
		hits++
	}
	if len([]rune(strings.TrimSpace(p.Rule))) < 16 {
		hits++
	}
	return clamp01(float64(hits) * 0.18)
}

func categorySpecificityBonus(p *Pattern) float64 {
	switch p.Category {
	case CategoryBusiness:
		if p.BusinessMethod != nil {
			return 0.18
		}
		return 0.08
	case CategoryUtils:
		if p.BusinessMethod != nil {
			return 0.12
		}
	case CategoryAPI, CategoryDatabase:
		return 0.05
	}
	return 0
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
		p.ScopePath == other.ScopePath &&
		p.AnalysisUnitID == other.AnalysisUnitID
}

// ==================== 问题 ====================

// Severity 问题严重程度
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Issue 问题实体
type Issue struct {
	File       string   // 文件路径
	Line       int      // 行号
	Column     int      // 列号（可选）
	Severity   Severity // 严重程度
	Message    string   // 问题描述
	Suggestion string   // 修复建议
	PatternID  string   // 关联的模式ID
	Confidence float64  // 置信度（0.0-1.0）
}

// PatternHit 表示一次 check 对某条模式的命中。
type PatternHit struct {
	PatternID  string
	File       string
	Line       int
	Severity   Severity
	Confidence float64
	CheckRunID string
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PatternHitStats 表示模式质量指标与 check 命中统计的聚合视图。
type PatternHitStats struct {
	Pattern   Pattern
	HitCount  int
	LastHitAt time.Time
}

// ReviewComment 表示导入的代码评审评论。
type ReviewComment struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	ReviewID  string    `json:"review_id"`
	Commit    string    `json:"commit"`
	File      string    `json:"file"`
	Line      int       `json:"line"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Resolved  bool      `json:"resolved"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AnalyzedCommitRecord 保存 learn history 已分析提交的持久化记录。
type AnalyzedCommitRecord struct {
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReviewMatchedPatternStats 表示评审评论命中的模式统计。
type ReviewMatchedPatternStats struct {
	PatternID    string
	CommentCount int
}

// ReviewStats 表示评审评论与 check 命中的匹配统计。
type ReviewStats struct {
	TotalComments     int
	PreventedComments int
	MissedComments    int
	MatchedPatterns   []ReviewMatchedPatternStats
}

// NewIssue 创建新问题
func NewIssue(file string, line int, severity Severity, message string) *Issue {
	return &Issue{
		File:     file,
		Line:     line,
		Severity: severity,
		Message:  message,
	}
}

// IsError 是否是错误级别
func (i *Issue) IsError() bool {
	return i.Severity == SeverityError
}

// IsWarning 是否是警告级别
func (i *Issue) IsWarning() bool {
	return i.Severity == SeverityWarning
}

// SetSuggestion 设置修复建议
func (i *Issue) SetSuggestion(suggestion string) {
	i.Suggestion = suggestion
}

// SetPatternID 设置关联的模式ID
func (i *Issue) SetPatternID(patternID string) {
	i.PatternID = patternID
}

// ==================== 提交 ====================

// CommitInfo 提交值对象
type CommitInfo struct {
	Hash    string    // 提交哈希
	Author  string    // 作者
	Date    time.Time // 提交时间
	Message string    // 提交消息
}

// NewCommitInfo 创建提交信息
func NewCommitInfo(hash, author, message string, date time.Time) CommitInfo {
	return CommitInfo{
		Hash:    hash,
		Author:  author,
		Date:    date,
		Message: message,
	}
}

// IsEmpty 是否为空
func (c CommitInfo) IsEmpty() bool {
	return c.Hash == ""
}

// ShortHash 获取短哈希（前7位）
func (c CommitInfo) ShortHash() string {
	if len(c.Hash) <= 7 {
		return c.Hash
	}
	return c.Hash[:7]
}

// Summary 获取提交摘要（第一行消息）
func (c CommitInfo) Summary() string {
	for i, ch := range c.Message {
		if ch == '\n' {
			return c.Message[:i]
		}
	}
	return c.Message
}

// ==================== 文件 ====================

// Status 文件状态
type Status string

const (
	StatusAdded    Status = "added"
	StatusModified Status = "modified"
	StatusDeleted  Status = "deleted"
)

// FileInfo 文件值对象
type FileInfo struct {
	Path     string // 文件路径
	Content  string // 文件内容
	Language string // 语言类型
	Status   Status // 状态
}

// NewFileInfo 创建文件信息
func NewFileInfo(path, content string) FileInfo {
	return FileInfo{
		Path:     path,
		Content:  content,
		Language: detectLanguage(path),
		Status:   StatusModified,
	}
}

// IsGoFile 是否是 Go 文件
func (f FileInfo) IsGoFile() bool {
	return f.Language == "go"
}

// IsTestFile 是否是测试文件
func (f FileInfo) IsTestFile() bool {
	return len(f.Path) > 8 && f.Path[len(f.Path)-8:] == "_test.go"
}

// IsEmpty 是否为空
func (f FileInfo) IsEmpty() bool {
	return f.Content == ""
}

// LineCount 获取行数
func (f FileInfo) LineCount() int {
	count := 0
	for _, ch := range f.Content {
		if ch == '\n' {
			count++
		}
	}
	return count + 1
}

// detectLanguage 根据文件扩展名检测语言
func detectLanguage(path string) string {
	if len(path) == 0 {
		return ""
	}

	// 获取扩展名
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext := path[i+1:]
			switch ext {
			case "go":
				return "go"
			case "js", "jsx":
				return "javascript"
			case "ts", "tsx":
				return "typescript"
			case "py":
				return "python"
			case "java":
				return "java"
			case "cpp", "cc", "cxx":
				return "cpp"
			case "c":
				return "c"
			case "rs":
				return "rust"
			case "rb":
				return "ruby"
			case "php":
				return "php"
			case "swift":
				return "swift"
			case "kt":
				return "kotlin"
			case "scala":
				return "scala"
			case "md":
				return "markdown"
			case "yaml", "yml":
				return "yaml"
			case "json":
				return "json"
			case "xml":
				return "xml"
			case "sql":
				return "sql"
			case "sh":
				return "shell"
			case "dockerfile":
				return "dockerfile"
			case "makefile":
				return "makefile"
			default:
				return ext
			}
		}
		if path[i] == '/' {
			break
		}
	}

	// 检查特殊文件名
	switch path {
	case "Dockerfile":
		return "dockerfile"
	case "Makefile":
		return "makefile"
	case "go.mod", "go.sum":
		return "go"
	}

	return ""
}
