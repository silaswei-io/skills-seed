package domain

import "strings"

// ProjectProfile 是学习阶段沉淀的持久化项目级知识。
// 生成的参考文档会基于该画像渲染。
type ProjectProfile struct {
	ProjectName       string              `json:"project_name"`
	Language          string              `json:"language"`
	Frameworks        []string            `json:"frameworks"`
	Architecture      string              `json:"architecture"`
	Structure         string              `json:"structure"`
	CommonUtils       []UtilityFunction   `json:"common_utils"`
	KeyModules        []ModuleInfo        `json:"key_modules"`
	ConfigPatterns    []string            `json:"config_patterns"`
	Dependencies      []string            `json:"dependencies"`
	Layers            []ArchitectureLayer `json:"layers"`
	DependencyGraph   string              `json:"dependency_graph"`
	DataFlow          string              `json:"data_flow"`
	FrameworkPatterns []string            `json:"framework_patterns"`
	BusinessMethods   []BusinessMethod    `json:"business_methods"`
	EngineeringRules  []EngineeringRule   `json:"engineering_rules,omitempty"`
	AuthorityCoverage []AuthorityCoverage `json:"authority_coverage,omitempty"`
	AuthorityRevision string              `json:"authority_revision,omitempty"`
	Summary           string              `json:"summary"`
	GeneratedAt       string              `json:"generated_at"`
}

// EngineeringRule 描述来自权威工程知识文件或用户上下文的显式约束。
type EngineeringRule struct {
	Title         string   `json:"title"`
	Rule          string   `json:"rule"`
	Source        string   `json:"source"`
	Section       string   `json:"section,omitempty"`
	AppliesTo     []string `json:"applies_to,omitempty"`
	CommandPolicy string   `json:"command_policy,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

const (
	// CommandPolicyForbidden 表示权威规则禁止执行匹配命令。
	CommandPolicyForbidden = "forbidden"
	// CommandPolicyDescribeOnly 表示只能说明命令，不得执行。
	CommandPolicyDescribeOnly = "describe_only"
	// CommandPolicyRequiresAuthorization 表示必须取得用户当轮明确授权后才能执行。
	CommandPolicyRequiresAuthorization = "requires_authorization"
	// CommandPolicyAllowed 表示权威规则明确允许执行；仍受当前用户指令和运行环境约束。
	CommandPolicyAllowed = "allowed"
)

// AuthorityCoverage 记录一次画像同步纳入的权威输入文件及 Markdown 章节。
type AuthorityCoverage struct {
	Source   string   `json:"source"`
	Sections []string `json:"sections,omitempty"`
}

// IsRouteableBusinessMethod 判断能力入口是否具备未来 Agent 自主复用所需的完整契约。
func IsRouteableBusinessMethod(method BusinessMethod) bool {
	return strings.TrimSpace(method.Name) != "" &&
		strings.TrimSpace(method.DisplayLocation()) != "" &&
		strings.TrimSpace(method.Function) != "" &&
		strings.TrimSpace(method.Description) != "" &&
		strings.TrimSpace(method.Usage) != "" &&
		strings.TrimSpace(method.Prerequisites) != "" &&
		strings.TrimSpace(method.Returns) != ""
}

// ProjectSpec 是由项目画像和已学习模式生成的项目级开发规范
type ProjectSpec struct {
	ProjectID         string              `json:"project_id,omitempty"`
	ProjectName       string              `json:"project_name"`
	ScopePath         string              `json:"scope_path,omitempty"`
	WorkspaceRole     string              `json:"workspace_role,omitempty"`
	Language          string              `json:"language"`
	EngineeringRules  []EngineeringRule   `json:"engineering_rules,omitempty"`
	AuthorityCoverage []AuthorityCoverage `json:"authority_coverage,omitempty"`
	GeneratedAt       string              `json:"generated_at"`
}

// ArchitectureLayer 描述项目中的一个逻辑分层
type ArchitectureLayer struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Responsibilities []string `json:"responsibilities"`
	Files            []string `json:"files"`
}

// UtilityFunction 描述项目中常用的工具函数
type UtilityFunction struct {
	Name        string `json:"name"`
	File        string `json:"file"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
	Usage       string `json:"usage"`
}

// ModuleInfo 描述项目中的关键模块
type ModuleInfo struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"display_name,omitempty"`
	Path             string   `json:"path"`
	Description      string   `json:"description"`
	Responsibilities []string `json:"responsibilities"`
	Dependencies     []string `json:"dependencies"`
	Dependents       []string `json:"dependents"`
	KeyMethods       []string `json:"key_methods"`
}
