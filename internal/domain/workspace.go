package domain

// WorkspaceProject 描述工作区根目录下的一个子项目
type WorkspaceProject struct {
	ID       string `json:"id" yaml:"id"`
	Path     string `json:"path" yaml:"path"`
	Type     string `json:"type" yaml:"type"`
	Language string `json:"language" yaml:"language"`
}

// WorkspaceProfile 是持久化的工作区事实画像
type WorkspaceProfile struct {
	Name        string             `json:"name"`
	RootPath    string             `json:"root_path"`
	Projects    []WorkspaceProject `json:"projects"`
	Shared      []WorkspacePath    `json:"shared,omitempty"`
	Contracts   []WorkspacePath    `json:"contracts,omitempty"`
	Infra       []WorkspacePath    `json:"infra,omitempty"`
	GeneratedAt string             `json:"generated_at"`
}

// WorkspacePath 描述工作区中非子项目但有特殊职责的路径
type WorkspacePath struct {
	Path        string `json:"path" yaml:"path"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// WorkspaceSpec 是用于渲染根 skills 的工作区级开发规范
type WorkspaceSpec struct {
	Name        string             `json:"name"`
	RootPath    string             `json:"root_path"`
	Projects    []WorkspaceProject `json:"projects"`
	Routing     []WorkspaceRoute   `json:"routing"`
	Rules       []WorkspaceRule    `json:"rules"`
	GeneratedAt string             `json:"generated_at"`
}

// WorkspaceRoute 把路径映射到改动前应读取的子项目 skills
type WorkspaceRoute struct {
	PathPattern string   `json:"path_pattern"`
	ProjectIDs  []string `json:"project_ids"`
	Reason      string   `json:"reason"`
}

// WorkspaceRule 是一条跨项目规则
type WorkspaceRule struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	AppliesTo   []string `json:"applies_to,omitempty"`
}
