// Package domain 提供核心领域模型和业务规则
//
// 本包定义了 skills-seed 项目的核心领域对象
//   - Pattern: 代码模式聚合根
//   - Category: 模式分类
//   - Source: 模式来源
//   - CommitInfo: Git 提交信息
//   - FileInfo: 文件信息
//
// 遵循领域驱动设计（DDD）原则，所有领域对象都是不可变的（通过方法修改）
package domain

import (
	"regexp"
	"strings"
	"time"
)

// DefaultLocale 默认语言设置
const DefaultLocale = "zh-CN"

const (
	// ModeProject 表示把初始化根目录作为单个项目处理
	ModeProject = "project"
	// ModeWorkspace 表示把初始化根目录作为包含多个子项目的工作区处理
	ModeWorkspace = "workspace"
)

// ==================== Pattern ====================

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

// Source 模式来源
type Source string

const (
	SourceLearned Source = "learned" // 从 commit 历史学习
	SourceDefault Source = "default" // 默认规则
	SourceInit    Source = "init"    // 从初始代码库分析
)

// BusinessMethod 业务方法信息
type BusinessMethod struct {
	Name          string // 方法名称（如 GenerateUUID, CallUserRPC）
	Location      string // 方法位置（如 internal/utils/uuid.go:15）
	Description   string // 功能说明（1-2句话）
	Usage         string // 使用场景（何时使用）
	Type          string // 方法类型：domain（领域特定）| common（通用）
	Function      string // 完整的方法签名（如 func (s *Service) Method(ctx, req) (resp, error)）
	Prerequisites string // 调用前需要的设置或依赖（如 context, 已初始化的 service）
	Returns       string // 返回值说明（包括可能的错误情况）
}

// PatternMetrics 描述一条模式的质量和可排序性。
type PatternMetrics struct {
	SpecificityScore float64 // 项目特有性，0.0-1.0
	EvidenceCount    int     // 代码路径、方法签名、项目符号等证据数量
	GenericPenalty   float64 // 泛化/模板化惩罚，0.0-1.0
	EffectiveScore   float64 // 综合排序分，0.0-1.0
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
	ProjectID      string          `json:"project_id,omitempty"`     // workspace 模式下的子项目 ID
	ScopePath      string          `json:"scope_path,omitempty"`     // workspace 模式下的路径范围
	WorkspaceRole  string          `json:"workspace_role,omitempty"` // frontend/backend/middleware/shared 等
	CreatedAt      time.Time
	UpdatedAt      time.Time // 最后更新时间
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

// IsValid 验证模式是否有效
func (p *Pattern) IsValid() bool {
	return p.ID != "" &&
		p.Name != "" &&
		p.Category != "" &&
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
	text := strings.Join([]string{
		p.ID,
		p.Name,
		p.Description,
		p.Rule,
		p.GoodExample,
		p.BadExample,
		p.ProjectID,
		p.ScopePath,
		p.WorkspaceRole,
	}, "\n")

	count := 0
	count += countMatches(text, regexp.MustCompile(`(?:^|[\s"'])[\w./-]+\.go(?::\d+)?(?:[\s"',)]|$)`))
	count += countMatches(text, regexp.MustCompile(`\b(?:internal|cmd|pkg|api|service|repository|handler|domain)/[\w./-]+`))
	count += countMatches(text, regexp.MustCompile(`\b[A-Z][A-Za-z0-9_]*\.[A-Za-z0-9_]+\b`))
	count += countMatches(text, regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s*)?[A-Za-z0-9_]+\s*\(`))
	count += countMatches(text, regexp.MustCompile("`[^`]+`"))

	if p.BusinessMethod != nil {
		count++
		if p.BusinessMethod.Location != "" {
			count++
		}
		if p.BusinessMethod.Function != "" {
			count++
		}
		if p.BusinessMethod.Type == "domain" {
			count++
		}
	}
	if p.ProjectID != "" {
		count++
	}
	if p.ScopePath != "" {
		count++
	}
	return count
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

func countMatches(text string, re *regexp.Regexp) int {
	return len(re.FindAllString(text, -1))
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

// ==================== Issue ====================

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
	CreatedAt  time.Time
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

// ==================== Commit ====================

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

// ==================== File ====================

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
