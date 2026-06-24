package agent

import (
	"context"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// ProjectContext 项目上下文
type ProjectContext struct {
	Name         string   // 项目名称
	Language     string   // 主要语言
	Frameworks   []string // 使用的框架
	Dependencies []string // 依赖项
}

// AnalyzeRequest 分析请求
type AnalyzeRequest struct {
	Files         []domain.FileInfo   // 待分析文件
	DiffFiles     []DiffFileRef       // 变更文件 diff 引用
	Context       ProjectContext      // 项目上下文
	Patterns      []domain.Pattern    // 已知模式
	RecentCommits []domain.CommitInfo // 最近提交
}

// DiffFileRef 指向 runtime 目录中的文件 diff。
type DiffFileRef struct {
	Path     string // 原文件路径
	DiffPath string // runtime 中的 diff 文件路径
}

// AnalyzeResult 分析结果
type AnalyzeResult struct {
	Issues      []domain.Issue // 发现的问题
	Suggestions []string       // 改进建议
	Confidence  float64        // 置信度
	AnalyzedAt  time.Time      // 分析时间
}

// LearnRequest 学习请求
type LearnRequest struct {
	Commit             domain.CommitInfo // 提交信息
	ChangedFiles       []string          // 变更文件路径
	KnownPatternsJSON  string            // 已知模式 JSON（不包含代码示例）
	KnownPatternsPath  string            // 已知模式 JSON 文件路径
	KnownPatternsCount int               // 已知模式数量（快速参考）
}

// CommitFileChange 关联提交元数据和对应的变更文件路径
type CommitFileChange struct {
	Commit domain.CommitInfo
	Files  []string
}

// LearnResult 学习结果
type LearnResult struct {
	Patterns  []domain.Pattern // 新学习的模式
	LearnedAt time.Time        // 学习时间
}

// BatchLearnRequest 批量学习请求
type BatchLearnRequest struct {
	Commits            []domain.CommitInfo // 批量提交信息
	CommitFiles        []CommitFileChange  // 批量提交变更文件路径
	KnownPatternsJSON  string              // 已知模式 JSON（不包含代码示例）
	KnownPatternsPath  string              // 已知模式 JSON 文件路径
	KnownPatternsCount int                 // 已知模式数量
}

// BatchLearnResult 批量学习结果
type BatchLearnResult struct {
	Patterns  []domain.Pattern // 新学习的模式（从所有 commits 中提取）
	LearnedAt time.Time        // 学习时间
}

// GenerateFixesRequest 生成修复请求
type GenerateFixesRequest struct {
	Issues  []domain.Issue    // 问题列表（直接内嵌）
	Files   []domain.FileInfo // 相关文件（直接内嵌）
	Context ProjectContext    // 项目上下文
}

// GenerateFixesResult 生成修复结果
type GenerateFixesResult struct {
	Fixes       map[string]string // 文件路径 -> 修复后的内容
	Confidence  float64           // 置信度
	Summary     string            // 修复摘要
	Warnings    []string          // 需要人工审查的警告
	GeneratedAt time.Time         // 生成时间
}

// GenerateSkillsResult AI汇总生成Skills的结果
type GenerateSkillsResult struct {
	CategorySummaries      map[string]CategorySummary // 按分类的汇总内容
	KeyPatterns            []PatternSummary           // 关键模式列表
	BusinessRules          []string                   // 业务规则总结
	BestPractices          []string                   // 最佳实践总结
	CommonPatterns         []string                   // 通用模式总结
	KeyInsights            []string                   // 关键洞察
	ImprovementSuggestions []string                   // 改进建议
}

// CategorySummary 分类汇总
type CategorySummary struct {
	Category        string                   // 分类名称
	Summary         string                   // 汇总描述
	Patterns        []string                 // 模式列表
	UsageScenes     []string                 // 使用场景
	Priority        int                      // 优先级（1-5，5最高）
	BusinessMethods []*domain.BusinessMethod // 业务方法（仅 business 分类）
}

// PatternSummary 模式摘要
type PatternSummary struct {
	Name       string // 模式名称
	Category   string // 分类
	Importance string // 重要性（high/medium/low）
	Summary    string // 简短摘要
	WhenToUse  string // 何时使用
}

// UserDefinePatternRequest 用户自定义模式请求
type UserDefinePatternRequest struct {
	Description string   // 用户自然语言描述
	Category    string   // 可选，用户指定的分类
	Files       []string // 可选，关联的文件路径
	UserContext string   // 可选，额外上下文
	WorkDir     string   // 项目根目录
	Language    string   // 项目语言
}

// AllowedCategories 返回提示词可展示的合法模式分类列表。
func (r *UserDefinePatternRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// UserDefinePatternResult 用户自定义模式结果
type UserDefinePatternResult struct {
	Pattern *domain.Pattern
}

// CuratePatternsRequest 模式策展请求。
type CuratePatternsRequest struct {
	Operation           string              // 操作来源，如 learn_current、learn_history、manual_compact
	CandidatePatterns   []domain.Pattern    // 本次分析产出的候选模式
	ExistingPatterns    []domain.Pattern    // 与候选相关的既有规范模式
	AllExisting         bool                // ExistingPatterns 是否代表整个模式集合
	ExistingByCandidate map[string][]string // 候选模式 ID -> 相关既有模式 ID
}

// CuratedPattern 表示 AI 策展后建议写入的规范模式。
type CuratedPattern struct {
	ID                string                           // 模式ID
	Name              string                           // 模式名称
	Category          string                           // 分类
	Description       string                           // 描述
	GoodExample       string                           // 正例代码
	BadExample        string                           // 反例代码
	Rule              string                           // 规则
	Confidence        float64                          // 置信度
	Frequency         int                              // 频率
	MergedFrom        []string                         // 被合并的候选或既有模式ID
	MergeReason       string                           // 合并理由
	SimilarityScore   float64                          // 合并相似度
	Source            string                           // 来源
	BusinessMethod    *domain.BusinessMethod           // 业务方法信息
	EvidenceLocations []domain.PatternEvidenceLocation // 通用源码证据位置
	ProjectID         string                           // workspace 子项目 ID
	ScopePath         string                           // workspace 路径范围
	WorkspaceRole     string                           // workspace 角色
}

// CuratedDrop 表示不入库的候选模式。
type CuratedDrop struct {
	ID     string // 候选模式 ID
	Reason string // 丢弃原因
}

// CuratePatternsResult 模式策展结果。
type CuratePatternsResult struct {
	Patterns []CuratedPattern // 应写入模式库的规范模式
	Dropped  []CuratedDrop    // 明确不入库的候选模式
	Summary  CurateSummary    // 策展统计
}

// CurateSummary 策展统计。
type CurateSummary struct {
	TotalCandidates int // 候选模式总数
	TotalExisting   int // 参与判断的既有模式总数
	TotalWritten    int // 输出规范模式数
	TotalDropped    int // 丢弃候选数
	MergeCount      int // 合并操作数
}

// FileSelectionCandidate 是 AI 文件选择器可见的候选文件元数据。
type FileSelectionCandidate struct {
	Path    string `json:"path"`              // 相对项目根路径
	Status  string `json:"status,omitempty"`  // 文件状态，如 added、modified、deleted
	Size    int64  `json:"size,omitempty"`    // 文件大小（字节）
	Kind    string `json:"kind,omitempty"`    // 文件类型，如 source、config、schema
	Changed bool   `json:"changed,omitempty"` // 是否属于本次新增或修改
}

// SelectFilesRequest 请求 AI 基于候选文件树选择本次应分析的文件。
type SelectFilesRequest struct {
	FileTree     string                   // 候选文件树，不包含源码内容
	Candidates   []FileSelectionCandidate // 候选文件元数据
	UserContext  string                   // 一次性用户上下文
	CandidateNum int                      // 候选文件数量
}

// SelectFilesResult 是 AI 文件选择器返回的结构化范围。
type SelectFilesResult struct {
	Include       []string // 需要纳入的相对路径或 glob
	Exclude       []string // 需要从 include 中剔除的相对路径或 glob
	SelectedPaths []string // 可选，明确选择的相对文件路径
	Reason        string   // 简短通用理由
}

// AnalyzeProjectRequest 项目分析请求
type AnalyzeProjectRequest struct {
	ProjectName           string   // 项目名称
	RootPath              string   // 项目根路径
	Language              string   // 主要语言
	Structure             string   // 目录结构（tree 输出）
	StructurePath         string   // 目录结构文件路径
	StructuralContext     string   // tree-sitter 结构化分析上下文
	StructuralContextPath string   // tree-sitter 结构化分析上下文文件路径
	ReadmePath            string   // README 文件路径（如果存在）
	MainFiles             []string // 主要入口文件路径
	ExistingProfileJSON   string   // 已有项目画像 JSON
	ExistingProfilePath   string   // 已有项目画像 JSON 文件路径
	FocusPaths            []string // 指定增量分析范围
	UserContext           string   // 本次学习传入的一次性用户上下文
	UserContextPath       string   // 本次学习传入的一次性用户上下文文件路径
}

// AnalyzeProjectResult 项目分析结果
type AnalyzeProjectResult struct {
	ProjectName        string                   // 项目名称
	Language           string                   // 主要编程语言
	Frameworks         []string                 // 使用的框架
	Architecture       string                   // 架构描述
	Structure          string                   // 目录结构说明
	CommonUtils        []domain.UtilityFunction // 公共工具方法
	KeyModules         []domain.ModuleInfo      // 关键模块
	ConfigPatterns     []string                 // 配置模式
	Dependencies       []string                 // 主要依赖
	Layers             []domain.ArchitectureLayer
	DependencyGraph    string
	DataFlow           string
	FrameworkPatterns  []string
	BusinessMethods    []domain.BusinessMethod
	ValidationCommands []domain.ValidationCommand
	Summary            string // 项目总结
}

// SampleFile 示例文件路径
type SampleFile struct {
	Path string // 文件路径
}

// AnalyzeCurrentCodebaseRequest 分析当前代码库请求
type AnalyzeCurrentCodebaseRequest struct {
	ProjectName           string        // 项目名称
	RootPath              string        // 项目根路径
	Language              string        // 主要语言
	FocusPaths            []string      // 指定扫描范围（相对项目根）
	Structure             string        // 目录结构
	StructurePath         string        // 目录结构文件路径
	StructuralContext     string        // tree-sitter 结构化分析上下文
	StructuralContextPath string        // tree-sitter 结构化分析上下文文件路径
	MainFiles             []string      // 主要入口文件路径
	SampleFiles           []SampleFile  // 示例文件路径
	DiffFiles             []DiffFileRef // 变更文件 diff 引用
	KnownPatternsJSON     string        // 已知模式 JSON（不包含代码示例）
	KnownPatternsPath     string        // 已知模式 JSON 文件路径
	KnownPatternsCount    int           // 已知模式数量
	FileCount             int           // 文件总数
	DirCount              int           // 目录总数
	UserContext           string        // 本次学习传入的一次性用户上下文
	UserContextPath       string        // 本次学习传入的一次性用户上下文文件路径
}

// AllowedCategories 返回提示词可展示的合法模式分类列表。
func (r *AnalyzeCurrentCodebaseRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// AnalyzeCurrentCodebaseResult 分析当前代码库结果
type AnalyzeCurrentCodebaseResult struct {
	Patterns          []domain.Pattern           // 提取的编码模式
	CategorySummaries map[string]CategorySummary // 按分类的汇总内容
	BusinessRules     []string                   // 业务规则
	BestPractices     []string                   // 最佳实践
	CommonPatterns    []string                   // 通用模式
	Summary           string                     // 总结
}

// AnalyzeWorkspaceProfileRequest 请求生成工作区事实画像
type AnalyzeWorkspaceProfileRequest struct {
	WorkspaceName      string // 工作区名称
	WorkspaceRoot      string // 工作区根路径
	WorkspaceInputPath string // 本次工作区生成输入文件路径
	UserContextPath    string // 本次学习传入的一次性用户上下文文件路径
}

// AnalyzeWorkspaceSpecRequest 请求生成工作区开发规范
type AnalyzeWorkspaceSpecRequest struct {
	WorkspaceName        string // 工作区名称
	WorkspaceRoot        string // 工作区根路径
	WorkspaceInputPath   string // 本次工作区生成输入文件路径
	WorkspaceProfilePath string // 本次工作区画像文件路径
	UserContextPath      string // 本次学习传入的一次性用户上下文文件路径
}

// OptimizeWorkflowRequest 请求把用户口语化说明整理为标准工作流。
type OptimizeWorkflowRequest struct {
	ID              string // 工作流 ID
	Name            string // 工作流名称
	Context         string // 本次用户输入
	ExistingContent string // 已有工作流正文；默认合并时用于去重整合
	Overwrite       bool   // 是否重写同名工作流
	Language        string // 项目主要语言
}

// OptimizeWorkflowResult 是 AI 优化后的标准工作流。
type OptimizeWorkflowResult struct {
	Title       string
	Content     string
	Summary     string
	Suggestions []string
}

// CodeAnalyzer 代码分析接口
type CodeAnalyzer interface {
	AnalyzeCode(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResult, error)
}

// PatternLearner 模式学习接口
type PatternLearner interface {
	LearnFromCommit(ctx context.Context, req *LearnRequest) (*LearnResult, error)
	BatchLearnFromCommits(ctx context.Context, req *BatchLearnRequest) (*BatchLearnResult, error)
}

// FixGenerator 修复生成接口
type FixGenerator interface {
	GenerateFixes(ctx context.Context, req *GenerateFixesRequest) (*GenerateFixesResult, error)
}

// PatternCurator 模式策展接口
type PatternCurator interface {
	CuratePatterns(ctx context.Context, req *CuratePatternsRequest) (*CuratePatternsResult, error)
}

// UserPatternDefiner 用户自定义模式接口
type UserPatternDefiner interface {
	UserDefinePattern(ctx context.Context, req *UserDefinePatternRequest) (*UserDefinePatternResult, error)
}

// FileSelector 基于候选文件树选择当前代码学习范围。
type FileSelector interface {
	SelectFiles(ctx context.Context, req *SelectFilesRequest) (*SelectFilesResult, error)
}

// ProjectAnalyzer 项目分析接口
type ProjectAnalyzer interface {
	AnalyzeProject(ctx context.Context, req *AnalyzeProjectRequest) (*AnalyzeProjectResult, error)
	AnalyzeCurrentCodebase(ctx context.Context, req *AnalyzeCurrentCodebaseRequest) (*AnalyzeCurrentCodebaseResult, error)
	AnalyzeWorkspaceProfile(ctx context.Context, req *AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error)
	AnalyzeWorkspaceSpec(ctx context.Context, req *AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error)
}

// WorkflowOptimizer 工作流优化接口。
type WorkflowOptimizer interface {
	OptimizeWorkflow(ctx context.Context, req *OptimizeWorkflowRequest) (*OptimizeWorkflowResult, error)
}

// Agent AI Agent 接口（组合所有子接口）
type Agent interface {
	Name() string
	IsAvailable() bool
	CodeAnalyzer
	PatternLearner
	FixGenerator
	PatternCurator
	UserPatternDefiner
	FileSelector
	ProjectAnalyzer
	WorkflowOptimizer
}
