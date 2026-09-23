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
	"time"
)

const (
	// ModeProject 表示把初始化根目录作为单个项目处理
	ModeProject = "project"
	// ModeWorkspace 表示把初始化根目录作为包含多个子项目的工作区处理
	ModeWorkspace = "workspace"
)

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

// PatternStats 表示模式质量指标聚合视图。
type PatternStats struct {
	Pattern Pattern
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

// languageByExtension 记录常见文件扩展名到语言名的稳定映射。
var languageByExtension = map[string]string{
	"go":         "go",
	"js":         "javascript",
	"jsx":        "javascript",
	"ts":         "typescript",
	"tsx":        "typescript",
	"py":         "python",
	"java":       "java",
	"cpp":        "cpp",
	"cc":         "cpp",
	"cxx":        "cpp",
	"c":          "c",
	"rs":         "rust",
	"rb":         "ruby",
	"php":        "php",
	"swift":      "swift",
	"kt":         "kotlin",
	"scala":      "scala",
	"md":         "markdown",
	"yaml":       "yaml",
	"yml":        "yaml",
	"json":       "json",
	"xml":        "xml",
	"sql":        "sql",
	"sh":         "shell",
	"dockerfile": "dockerfile",
	"makefile":   "makefile",
}

// languageByFilename 记录没有常规扩展名但需要识别的文件名。
var languageByFilename = map[string]string{
	"Dockerfile": "dockerfile",
	"Makefile":   "makefile",
	"go.mod":     "go",
	"go.sum":     "go",
}

// detectLanguage 根据文件扩展名检测语言
func detectLanguage(path string) string {
	if len(path) == 0 {
		return ""
	}

	if ext := extensionBeforeSlash(path); ext != "" {
		if language, ok := languageByExtension[ext]; ok {
			return language
		}
		return ext
	}

	if language, ok := languageByFilename[path]; ok {
		return language
	}
	return ""
}

func extensionBeforeSlash(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		switch path[i] {
		case '.':
			if i == len(path)-1 {
				return ""
			}
			return path[i+1:]
		case '/':
			return ""
		}
	}
	return ""
}
