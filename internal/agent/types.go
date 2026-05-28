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
	Context       ProjectContext      // 项目上下文
	Patterns      []domain.Pattern    // 已知模式
	RecentCommits []domain.CommitInfo // 最近提交
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

// CommitFileChange pairs commit metadata with changed file paths
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
	GeneratedAt time.Time         // 生成时间
}

// GenerateSkillsRequest AI汇总生成Skills的请求
type GenerateSkillsRequest struct {
	PatternsJSON       string // 模式 JSON（不包含代码示例）
	PatternsPath       string // 模式 JSON 文件路径
	PatternsCount      int    // 模式总数（快速参考）
	ExistingSkillsPath string // 已有 skills 文件路径（如果有）
	ProjectName        string // 项目名称
	Language           string // 项目语言
	UserContext        string // 本次生成传入的一次性用户上下文
	UserContextPath    string // 本次生成的一次性用户上下文文件路径
}

// GenerateSkillsResult AI汇总生成Skills的结果
type GenerateSkillsResult struct {
	CategorySummaries map[string]CategorySummary // 按分类的汇总内容
	KeyPatterns       []PatternSummary           // 关键模式列表
	BusinessRules     []string                   // 业务规则总结
	BestPractices     []string                   // 最佳实践总结
	CommonPatterns    []string                   // 通用模式总结
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

// MergePatternsRequest 模式汇总请求
type MergePatternsRequest struct {
	Category string           // 分类名称
	Patterns []domain.Pattern // 待汇总的模式列表
}

// MergedPattern 合并后的模式
type MergedPattern struct {
	ID          string   // 模式ID
	Name        string   // 模式名称
	Category    string   // 分类
	Description string   // 描述
	Rule        string   // 规则
	Confidence  float64  // 置信度
	MergedFrom  []string // 从哪些模式合并而来
	MergeReason string   // 合并理由
}

// UnchangedPattern 未变更的模式
type UnchangedPattern struct {
	ID     string // 模式ID
	Reason string // 未变更的理由
}

// MergePatternsResult 模式汇总结果
type MergePatternsResult struct {
	MergedPatterns    []MergedPattern    // 合并后的模式
	UnchangedPatterns []UnchangedPattern // 未变更的模式
	Summary           MergeSummary       // 汇总统计
}

// MergeSummary 汇总统计
type MergeSummary struct {
	TotalInput     int // 输入模式总数
	TotalMerged    int // 合并后模式数
	TotalUnchanged int // 未变更模式数
	MergeCount     int // 合并操作次数
}

// AnalyzeProjectRequest 项目分析请求
type AnalyzeProjectRequest struct {
	ProjectName           string   // 项目名称
	RootPath              string   // 项目根路径
	Language              string   // 主要语言
	Structure             string   // 目录结构（tree 输出）
	StructurePath         string   // 目录结构文件路径
	StructuralContext     string   // CodeGraph 等结构化分析上下文
	StructuralContextPath string   // CodeGraph 等结构化分析上下文文件路径
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
	ProjectName       string                   // 项目名称
	Language          string                   // 主要编程语言
	Frameworks        []string                 // 使用的框架
	Architecture      string                   // 架构描述
	Structure         string                   // 目录结构说明
	CommonUtils       []domain.UtilityFunction // 公共工具方法
	KeyModules        []domain.ModuleInfo      // 关键模块
	ConfigPatterns    []string                 // 配置模式
	Dependencies      []string                 // 主要依赖
	Layers            []domain.ArchitectureLayer
	DependencyGraph   string
	DataFlow          string
	FrameworkPatterns []string
	BusinessMethods   []domain.BusinessMethod
	Summary           string // 项目总结
}

// SampleFile 示例文件路径
type SampleFile struct {
	Path string // 文件路径
}

// AnalyzeCurrentCodebaseRequest 分析当前代码库请求
type AnalyzeCurrentCodebaseRequest struct {
	ProjectName           string       // 项目名称
	RootPath              string       // 项目根路径
	Language              string       // 主要语言
	FocusPaths            []string     // 指定扫描范围（相对项目根）
	Structure             string       // 目录结构
	StructurePath         string       // 目录结构文件路径
	StructuralContext     string       // CodeGraph 等结构化分析上下文
	StructuralContextPath string       // CodeGraph 等结构化分析上下文文件路径
	MainFiles             []string     // 主要入口文件路径
	SampleFiles           []SampleFile // 示例文件路径
	KnownPatternsJSON     string       // 已知模式 JSON（不包含代码示例）
	KnownPatternsPath     string       // 已知模式 JSON 文件路径
	KnownPatternsCount    int          // 已知模式数量
	FileCount             int          // 文件总数
	DirCount              int          // 目录总数
	UserContext           string       // 本次学习传入的一次性用户上下文
	UserContextPath       string       // 本次学习传入的一次性用户上下文文件路径
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
	UserContextPath    string // 本次用户补充说明文件路径
}

// AnalyzeWorkspaceSpecRequest 请求生成工作区开发规范
type AnalyzeWorkspaceSpecRequest struct {
	WorkspaceName        string // 工作区名称
	WorkspaceRoot        string // 工作区根路径
	WorkspaceInputPath   string // 本次工作区生成输入文件路径
	WorkspaceProfilePath string // 本次工作区画像文件路径
	UserContextPath      string // 本次用户补充说明文件路径
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

// SkillsGenerator Skills生成接口
type SkillsGenerator interface {
	GenerateSkillsSummary(ctx context.Context, req *GenerateSkillsRequest) (*GenerateSkillsResult, error)
}

// PatternMerger 模式合并接口
type PatternMerger interface {
	MergePatterns(ctx context.Context, req *MergePatternsRequest) (*MergePatternsResult, error)
}

// ProjectAnalyzer 项目分析接口
type ProjectAnalyzer interface {
	AnalyzeProject(ctx context.Context, req *AnalyzeProjectRequest) (*AnalyzeProjectResult, error)
	AnalyzeCurrentCodebase(ctx context.Context, req *AnalyzeCurrentCodebaseRequest) (*AnalyzeCurrentCodebaseResult, error)
	AnalyzeWorkspaceProfile(ctx context.Context, req *AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error)
	AnalyzeWorkspaceSpec(ctx context.Context, req *AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error)
}

// Agent AI Agent 接口（组合所有子接口）
type Agent interface {
	Name() string
	IsAvailable() bool
	CodeAnalyzer
	PatternLearner
	FixGenerator
	SkillsGenerator
	PatternMerger
	ProjectAnalyzer
}
