package domain

// ProjectProfile is the durable project-level knowledge captured by learning
// Generated reference documents are rendered from this profile
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

// ArchitectureLayer describes one logical layer in the project
type ArchitectureLayer struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Responsibilities []string `json:"responsibilities"`
	Files            []string `json:"files"`
}

// UtilityFunction describes a common utility function used by the project
type UtilityFunction struct {
	Name        string `json:"name"`
	File        string `json:"file"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
	Usage       string `json:"usage"`
}

// ModuleInfo describes a key project module
type ModuleInfo struct {
	Name             string   `json:"name"`
	Path             string   `json:"path"`
	Description      string   `json:"description"`
	Responsibilities []string `json:"responsibilities"`
	Dependencies     []string `json:"dependencies"`
	Dependents       []string `json:"dependents"`
	KeyMethods       []string `json:"key_methods"`
}
