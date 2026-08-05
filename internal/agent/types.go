package agent

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

// DiffFileRef 指向 runtime 目录中的文件 diff。
type DiffFileRef struct {
	Path     string // 原文件路径
	DiffPath string // runtime 中的 diff 文件路径
}

// UserDefinePatternRequest 用户自定义模式请求
type UserDefinePatternRequest struct {
	Description string // 用户自然语言描述
	Category    string // 可选，用户指定的分类
	UserContext string // 可选，额外上下文
	WorkDir     string // 项目根目录
	Language    string // 项目语言
}

// AllowedCategories 返回提示词可展示的合法模式分类列表。
func (r *UserDefinePatternRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// UserDefinePatternResult 用户自定义模式结果
type UserDefinePatternResult struct {
	Pattern *domain.Pattern
}

// AnalyzeProjectRequest 项目分析请求
type AnalyzeProjectRequest struct {
	ProjectName           string   // 项目名称
	RootPath              string   // 项目根路径
	Language              string   // 主要语言
	Structure             string   // 目录结构（tree 输出）
	StructurePath         string   // 目录结构文件路径
	StructuralContext     string   // 结构化分析上下文
	StructuralContextPath string   // 结构化分析上下文文件路径
	ReadmePath            string   // README 文件路径（如果存在）
	MainFiles             []string // 主要入口文件路径
	EngineeringKnowledge  []string // 权威工程知识文件路径，不参与业务源码筛选
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
	EngineeringRules   []domain.EngineeringRule
	ValidationCommands []domain.ValidationCommand
	Summary            string // 项目总结
}

// SampleFile 示例文件路径
type SampleFile struct {
	Path string // 文件路径
}

// AnalyzeCurrentCodebaseRequest 描述单个证据焦点的当前代码分析输入。
// 真实 agent provider 不再直接消费该请求；它只作为服务层和测试适配器的内部 DTO。
type AnalyzeCurrentCodebaseRequest struct {
	ProjectName           string
	RootPath              string
	Language              string
	RuntimeLabel          string
	EvidenceFocus         domain.EvidenceFocus
	FocusPaths            []string
	Structure             string
	StructurePath         string
	StructuralContext     string
	StructuralContextPath string
	MainFiles             []string
	SampleFiles           []SampleFile
	DiffFiles             []DiffFileRef
	KnownPatternsJSON     string
	KnownPatternsPath     string
	KnownPatternsCount    int
	FileCount             int
	DirCount              int
	UserContext           string
	UserContextPath       string
	LearningMode          config.LearningMode
	ChangeProfile         string
}

// AnalyzeCurrentCodebaseResult 描述单个证据焦点的当前代码分析结果。
type AnalyzeCurrentCodebaseResult struct {
	Patterns                  []domain.Pattern
	ProfileRefreshRecommended ProfileRefreshRecommendation
}

// AnalyzeCurrentEvidenceFocus 描述批量当前代码学习中的单个证据焦点输入。
type AnalyzeCurrentEvidenceFocus struct {
	EvidenceFocus domain.EvidenceFocus
	FocusPaths    []string
	SampleFiles   []SampleFile
	DiffFiles     []DiffFileRef
}

// AnalyzeCurrentCodebaseBatchRequest 请求在一次 Agent 调用中分析多个证据焦点。
type AnalyzeCurrentCodebaseBatchRequest struct {
	ProjectName           string
	RootPath              string
	Language              string
	RuntimeLabel          string
	Focuses               []AnalyzeCurrentEvidenceFocus
	Structure             string
	StructurePath         string
	StructuralContext     string
	StructuralContextPath string
	MainFiles             []string
	UserContext           string
	UserContextPath       string
	LearningMode          config.LearningMode
	ChangeProfile         string
}

// AllowedCategories 返回提示词可展示的合法模式分类列表。
func (r *AnalyzeCurrentCodebaseBatchRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// AnalyzeCurrentEvidenceResult 是批量当前代码学习返回的单个证据焦点结果。
type AnalyzeCurrentEvidenceResult struct {
	FocusID                   string
	FocusName                 string
	Patterns                  []domain.Pattern
	ProfileRefreshRecommended ProfileRefreshRecommendation
}

// AnalyzeCurrentCodebaseBatchResult 是批量当前代码学习的结果。
type AnalyzeCurrentCodebaseBatchResult struct {
	Focuses []AnalyzeCurrentEvidenceResult
}

// AnalyzeCurrentDeltaFocus 描述增量学习中的单个 diff 锚定证据焦点输入。
type AnalyzeCurrentDeltaFocus struct {
	EvidenceFocus   domain.EvidenceFocus
	FocusPaths      []string
	ContextFiles    []SampleFile
	DiffFiles       []DiffFileRef
	RelatedPatterns []domain.Pattern
}

// AnalyzeCurrentDeltaBatchRequest 请求基于 diff anchor 判断知识变化。
type AnalyzeCurrentDeltaBatchRequest struct {
	ProjectName           string
	RootPath              string
	Language              string
	RuntimeLabel          string
	Focuses               []AnalyzeCurrentDeltaFocus
	Structure             string
	StructurePath         string
	StructuralContext     string
	StructuralContextPath string
	UserContext           string
	UserContextPath       string
	LearningMode          config.LearningMode
	ChangeProfile         string
}

// AllowedCategories 返回提示词可展示的合法模式分类列表。
func (r *AnalyzeCurrentDeltaBatchRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// AnalyzeCurrentDeltaBatchResult 是 diff 锚定增量学习的结构化结果。
type AnalyzeCurrentDeltaBatchResult struct {
	Changes                   []domain.KnowledgeChange
	ProfileRefreshRecommended ProfileRefreshRecommendation
}

// SelectLearningCandidatesRequest 请求从本地候选文件中收敛值得进入议程规划的文件。
type SelectLearningCandidatesRequest struct {
	ProjectName           string
	RootPath              string
	Language              string
	CandidatePaths        []string
	RequiredPaths         []string
	StructuralContext     string
	StructuralContextPath string
	UserContext           string
	UserContextPath       string
	LearningMode          config.LearningMode
	LearningScope         config.LearningScope
}

type LearningCandidateSkip struct {
	Path   string
	Reason string
}

// SelectLearningCandidatesResult 是 AI 候选收敛结果。
type SelectLearningCandidatesResult struct {
	SelectedPaths []string
	SkippedPaths  []LearningCandidateSkip
	Reason        string
}

// PlanLearningAgendaRequest 请求按业务能力拆分当前待学习文件。
type PlanLearningAgendaRequest struct {
	ProjectName           string
	RootPath              string
	Language              string
	FocusPaths            []string
	StructuralContext     string // 结构化分析上下文
	StructuralContextPath string // 结构化分析上下文文件路径
	UserContext           string
	LearningMode          config.LearningMode
	LearningScope         config.LearningScope
}

// PlanLearningAgendaResult 是 AI 生成的业务证据焦点计划。
type PlanLearningAgendaResult struct {
	Focuses []domain.EvidenceFocus `json:"focuses"`
}

// ProfileRefreshRecommendation 描述是否需要额外刷新完整项目画像。
type ProfileRefreshRecommendation struct {
	Needed bool   `json:"needed"`
	Reason string `json:"reason,omitempty"`
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
	Title     string
	Content   string
	Conflicts []string
}

// UserPatternDefiner 用户自定义模式接口
type UserPatternDefiner interface {
	UserDefinePattern(ctx context.Context, req *UserDefinePatternRequest) (*UserDefinePatternResult, error)
}

// ProjectAnalyzer 项目分析接口
type ProjectAnalyzer interface {
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
	LearningSessionProvider
	UserPatternDefiner
	ProjectAnalyzer
	WorkflowOptimizer
}
