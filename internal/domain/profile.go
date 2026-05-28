package domain

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
	Summary           string              `json:"summary"`
	GeneratedAt       string              `json:"generated_at"`
}

// ProjectSpec 是由项目画像和已学习模式生成的项目级开发规范
type ProjectSpec struct {
	ProjectID         string                   `json:"project_id,omitempty"`
	ProjectName       string                   `json:"project_name"`
	ScopePath         string                   `json:"scope_path,omitempty"`
	WorkspaceRole     string                   `json:"workspace_role,omitempty"`
	Language          string                   `json:"language"`
	Summary           string                   `json:"summary,omitempty"`
	Boundaries        []ProjectSpecBoundary    `json:"boundaries,omitempty"`
	PatternRules      []ProjectSpecPatternRule `json:"pattern_rules,omitempty"`
	ConfigPatterns    []string                 `json:"config_patterns,omitempty"`
	FrameworkPatterns []string                 `json:"framework_patterns,omitempty"`
	Touchpoints       []ProjectSpecTouchpoint  `json:"touchpoints,omitempty"`
	GeneratedAt       string                   `json:"generated_at"`
}

// ProjectSpecBoundary 描述项目内需要保护的层次、模块或职责边界
type ProjectSpecBoundary struct {
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	Responsibilities []string `json:"responsibilities,omitempty"`
	Paths            []string `json:"paths,omitempty"`
}

// ProjectSpecPatternRule 描述从 patterns 中提炼出的可执行规则
type ProjectSpecPatternRule struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Description string  `json:"description,omitempty"`
	Rule        string  `json:"rule,omitempty"`
	Confidence  float64 `json:"confidence"`
	Frequency   int     `json:"frequency"`
}

// ProjectSpecTouchpoint 描述改动时应优先检查的业务方法、工具或模块入口
type ProjectSpecTouchpoint struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
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
	Path             string   `json:"path"`
	Description      string   `json:"description"`
	Responsibilities []string `json:"responsibilities"`
	Dependencies     []string `json:"dependencies"`
	Dependents       []string `json:"dependents"`
	KeyMethods       []string `json:"key_methods"`
}
