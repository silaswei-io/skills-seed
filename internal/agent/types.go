package agent

import (
	"context"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/knowledge/maintained"
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
	ExistingProfileJSON   string   // 已有项目画像 JSON
	ExistingProfilePath   string   // 已有项目画像 JSON 文件路径
	FocusPaths            []string // 指定增量分析范围
	UserContext           string   // 本次学习传入的一次性用户上下文
	UserContextPath       string   // 本次学习传入的一次性用户上下文文件路径
	MaintainedGuidance    maintained.Snapshot
}

// AnalyzeProjectResult 项目分析结果
type AnalyzeProjectResult struct {
	ProjectName       string              // 项目名称
	Language          string              // 主要编程语言
	Frameworks        []string            // 使用的框架
	Architecture      string              // 架构描述
	Structure         string              // 目录结构说明
	KeyModules        []domain.ModuleInfo // 关键模块
	ConfigPatterns    []string            // 配置模式
	Dependencies      []string            // 主要依赖
	Layers            []domain.ArchitectureLayer
	DependencyGraph   string
	DataFlow          string
	FrameworkPatterns []string
	Summary           string // 项目总结
}

// ExtractAuthorityRequest 描述一次独立的权威知识提取输入。
type ExtractAuthorityRequest struct {
	ProjectName          string
	RootPath             string
	EngineeringKnowledge []string
	AuthoritySections    []AuthoritySection
	UserContext          string
	UserContextPath      string
	MaintainedGuidance   maintained.Snapshot
}

// ExtractAuthorityResult 描述逐章节提取的权威知识结果。
type ExtractAuthorityResult struct {
	AuthoritySections []AuthoritySectionResult `json:"authority_sections"`
}

// ReviewAuthorityRequest 描述对权威规则候选的独立完整性复核输入。
// 复核必须返回完整章节结果，而不是增量补丁，确保后续校验只面对一个事实集。
type ReviewAuthorityRequest struct {
	ExtractAuthorityRequest
	Candidate ExtractAuthorityResult
}

// AuthoritySection 是由程序确定性生成的权威章节目录项。
type AuthoritySection struct {
	ID      string `json:"section_id"`
	Source  string `json:"source"`
	Section string `json:"section,omitempty"`
}

// AuthoritySectionResult 是 Agent 在指定权威章节下提取的规则，不携带归属字段。
type AuthoritySectionResult struct {
	SectionID    string                   `json:"section_id"`
	Rules        []domain.EngineeringRule `json:"rules"`
	NoRuleReason string                   `json:"no_rule_reason,omitempty"`
}

// AuthoritySectionIDs 返回权威章节目录中的稳定 ID 列表，供运行时 Schema 收窄回执范围。
func AuthoritySectionIDs(sections []AuthoritySection) []string {
	ids := make([]string, 0, len(sections))
	for _, section := range sections {
		if section.ID != "" {
			ids = append(ids, section.ID)
		}
	}
	return ids
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
	MaintainedGuidance    maintained.Snapshot
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
	SharedContextPath     string
	Focuses               []AnalyzeCurrentEvidenceFocus
	Structure             string
	StructurePath         string
	StructuralContext     string
	StructuralContextPath string
	MainFiles             []string
	UserContext           string
	UserContextPath       string
	MaintainedGuidance    maintained.Snapshot
	LearningMode          config.LearningMode
	ChangeProfile         string
}

// AllowedCategories 返回提示词可展示的合法模式分类列表。
func (r *AnalyzeCurrentCodebaseBatchRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// FocusIDs 返回本次源码分析必须逐一回执的证据焦点 ID。
func (r *AnalyzeCurrentCodebaseBatchRequest) FocusIDs() []string {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.Focuses))
	for _, focus := range r.Focuses {
		if id := strings.TrimSpace(focus.EvidenceFocus.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
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
	Focuses      []AnalyzeCurrentEvidenceResult
	Conversation Conversation
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
	SharedContextPath     string
	Focuses               []AnalyzeCurrentDeltaFocus
	Structure             string
	StructurePath         string
	StructuralContext     string
	StructuralContextPath string
	UserContext           string
	UserContextPath       string
	MaintainedGuidance    maintained.Snapshot
	LearningMode          config.LearningMode
	ChangeProfile         string
}

// AllowedCategories 返回提示词可展示的合法模式分类列表。
func (r *AnalyzeCurrentDeltaBatchRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// FocusIDs 返回本次增量分析必须逐一回执的证据焦点 ID。
func (r *AnalyzeCurrentDeltaBatchRequest) FocusIDs() []string {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.Focuses))
	for _, focus := range r.Focuses {
		if id := strings.TrimSpace(focus.EvidenceFocus.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// AnalyzeCurrentDeltaBatchResult 是 diff 锚定增量学习的结构化结果。
type AnalyzeCurrentDeltaBatchResult struct {
	Changes                   []domain.KnowledgeChange
	ProfileRefreshRecommended ProfileRefreshRecommendation
	Conversation              Conversation
}

// LearningPathSkip 记录规划阶段明确跳过的输入路径及原因。
type LearningPathSkip struct {
	Path   string
	Reason string
}

// PlanningSourceFact 是规划 Agent 用于确认文件密度和声明边界的轻量事实。
type PlanningSourceFact struct {
	Path          string               `json:"path"`
	SizeBytes     int64                `json:"size_bytes"`
	LineCount     int                  `json:"line_count"`
	NonBlankLines int                  `json:"non_blank_lines"`
	SymbolCount   int                  `json:"symbol_count"`
	Symbols       []PlanningSymbolFact `json:"symbols,omitempty"`
}

// PlanningSymbolFact 是源码事实清单中的声明摘要。
type PlanningSymbolFact struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
}

// PlanLearningAgendaRequest 请求按源码证据边界拆分当前待学习文件。
type PlanLearningAgendaRequest struct {
	ProjectName           string
	RootPath              string
	Language              string
	FocusPaths            []string
	SourceFacts           []PlanningSourceFact
	StructuralContext     string // 结构化分析上下文
	StructuralContextPath string // 结构化分析上下文文件路径
	UserContext           string
	MaintainedGuidance    maintained.Snapshot
	LearningMode          config.LearningMode
}

// PlanLearningAgendaResult 是 AI 生成的业务证据焦点计划。
type PlanLearningAgendaResult struct {
	Focuses      []domain.EvidenceFocus
	SkippedPaths []LearningPathSkip
	Reason       string
}

// NormalizePatternsRequest 请求把当前学习得到的候选模式合并为稳定的入库决策。
type NormalizePatternsRequest struct {
	ProjectName        string
	RootPath           string
	Language           string
	Candidates         []domain.Pattern
	RelatedPatterns    []domain.Pattern
	UserContext        string
	UserContextPath    string
	MaintainedGuidance maintained.Snapshot
}

// ReviewKnowledgeRequest 请求独立复核当前源码学习候选。
type ReviewKnowledgeRequest struct {
	ProjectName        string
	RootPath           string
	Language           string
	RuntimeLabel       string
	EvidenceFocus      domain.EvidenceFocus
	Candidates         []domain.Pattern
	UserContext        string
	UserContextPath    string
	MaintainedGuidance maintained.Snapshot
	Conversation       Conversation
}

// KnowledgeRevision 只允许修订知识表述，不改变程序持有的证据和归属。
type KnowledgeRevision struct {
	Name           string
	Category       string
	Description    string
	Rule           string
	Confidence     float64
	KnowledgeFlags []string
}

// KnowledgeReviewDecision 是单个候选的独立审查结论。
type KnowledgeReviewDecision struct {
	CandidateID    string
	Verdict        string
	ReasonCode     string
	Reason         string
	BusinessMethod *domain.BusinessMethod
	Revision       *KnowledgeRevision
}

// ReviewKnowledgeResult 包含每个输入候选的一对一审查回执。
type ReviewKnowledgeResult struct {
	Decisions []KnowledgeReviewDecision
}

// PatternNormalization 描述一个规范化后的模式及其来源归属。
type PatternNormalization struct {
	ID          string
	Name        string
	Category    string
	Description string
	Rule        string
	Confidence  float64
	SourceIDs   []string
}

// PatternDrop 描述一个不应入库的候选模式。
type PatternDrop struct {
	ID         string
	ReasonCode string
	Reason     string
}

// NormalizePatternsResult 是 AI 合并候选模式后的所有权决策。
type NormalizePatternsResult struct {
	Patterns []PatternNormalization
	Dropped  []PatternDrop
}

// ProfileRefreshRecommendation 描述是否需要额外刷新完整项目画像。
type ProfileRefreshRecommendation struct {
	Needed bool   `json:"needed"`
	Reason string `json:"reason,omitempty"`
}

// AnalyzeWorkspaceProfileRequest 请求生成工作区事实画像
type AnalyzeWorkspaceProfileRequest struct {
	WorkspaceName      string   // 工作区名称
	WorkspaceRoot      string   // 工作区根路径
	WorkspaceInputPath string   // 本次工作区生成输入文件路径
	UserContextPath    string   // 本次学习传入的一次性用户上下文文件路径
	ProjectIDs         []string // 配置声明的唯一合法子项目 ID
}

// AnalyzeWorkspaceSpecRequest 请求生成工作区开发规范
type AnalyzeWorkspaceSpecRequest struct {
	WorkspaceName        string   // 工作区名称
	WorkspaceRoot        string   // 工作区根路径
	WorkspaceInputPath   string   // 本次工作区生成输入文件路径
	WorkspaceProfilePath string   // 本次工作区画像文件路径
	UserContextPath      string   // 本次学习传入的一次性用户上下文文件路径
	ProjectIDs           []string // 配置声明的唯一合法子项目 ID
}

// ProjectContext 描述资源优化时可读取的当前项目边界。
type ProjectContext struct {
	Name     string
	RootPath string
	Language string
	Mode     string
}

// OptimizeWorkflowRequest 请求把用户提供的内容整理为可执行工作流。
type OptimizeWorkflowRequest struct {
	Project         ProjectContext
	Name            string
	ExistingContent string
	Content         string
	Overwrite       bool
}

// OptimizeRuleRequest 请求在项目边界内润色用户提供的权威规则。
type OptimizeRuleRequest struct {
	Project          ProjectContext
	Name             string
	ExistingContent  string
	Content          string
	AffectedProjects []string
	Paths            []string
	Overwrite        bool
}

// OptimizeContentResult 是资源优化后的正文和导航元数据。
type OptimizeContentResult struct {
	Content    string
	Summary    string
	RouteTerms []string
}

// UserPatternDefiner 用户自定义模式接口
type UserPatternDefiner interface {
	UserDefinePattern(ctx context.Context, req *UserDefinePatternRequest) (*UserDefinePatternResult, error)
}

// ProjectAnalyzer 项目分析接口
type ProjectAnalyzer interface {
	RefreshProjectProfile(ctx context.Context, req *AnalyzeProjectRequest) (*AnalyzeProjectResult, error)
	ExtractAuthority(ctx context.Context, req *ExtractAuthorityRequest) (*ExtractAuthorityResult, error)
	ReviewAuthority(ctx context.Context, req *ReviewAuthorityRequest) (*ExtractAuthorityResult, error)
	PlanLearningAgenda(ctx context.Context, req *PlanLearningAgendaRequest) (*PlanLearningAgendaResult, error)
	AnalyzeCurrentCodebaseBatch(ctx context.Context, req *AnalyzeCurrentCodebaseBatchRequest) (*AnalyzeCurrentCodebaseBatchResult, error)
	AnalyzeCurrentDeltaBatch(ctx context.Context, req *AnalyzeCurrentDeltaBatchRequest) (*AnalyzeCurrentDeltaBatchResult, error)
	NormalizePatterns(ctx context.Context, req *NormalizePatternsRequest) (*NormalizePatternsResult, error)
	AnalyzeWorkspaceProfile(ctx context.Context, req *AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error)
	AnalyzeWorkspaceSpec(ctx context.Context, req *AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error)
}

// PatternNormalizer 优化当前学习候选模式归并。
type PatternNormalizer interface {
	NormalizePatterns(ctx context.Context, req *NormalizePatternsRequest) (*NormalizePatternsResult, error)
}

// KnowledgeReviewer 独立复核源码学习候选的证据和表述边界。
type KnowledgeReviewer interface {
	ReviewKnowledge(ctx context.Context, req *ReviewKnowledgeRequest) (*ReviewKnowledgeResult, error)
}

// ResourceOptimizer 整理用户维护的工作流和权威规则。
type ResourceOptimizer interface {
	OptimizeWorkflow(ctx context.Context, req *OptimizeWorkflowRequest) (*OptimizeContentResult, error)
	OptimizeRule(ctx context.Context, req *OptimizeRuleRequest) (*OptimizeContentResult, error)
}

// Agent AI Agent 接口（组合所有子接口）
type Agent interface {
	Name() string
	IsAvailable() bool
	UserPatternDefiner
	ProjectAnalyzer
	KnowledgeReviewer
	ResourceOptimizer
}
