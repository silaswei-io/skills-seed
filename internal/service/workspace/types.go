package workspace

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

type workspaceSkillTemplateData struct {
	ProgramVersion      string
	SkillsTemplatesHash string
	SkillName           string
	ProjectName         string
	WorkspaceName       string
	WorkspaceFacts      string
	ProjectCount        int
	Projects            []workspaceProjectTemplateData
	Shared              []domain.WorkspacePath
	Contracts           []domain.WorkspacePath
	Infra               []domain.WorkspacePath
	Dependencies        []domain.WorkspaceDependency
	ImpactRoutes        []domain.WorkspaceRoute
	Routing             []domain.WorkspaceRoute
	Rules               []domain.WorkspaceRule
	ChangeOrder         []string
	ParallelGuidance    []domain.WorkspaceParallelGuidance
	LoadMultipleWhen    []domain.WorkspaceLoadMultipleSkill
	HasWorkspaceFacts   bool
	HasShared           bool
	HasContracts        bool
	HasInfra            bool
	HasDependencies     bool
	HasImpactRoutes     bool
	HasRouting          bool
	HasRules            bool
	HasChangeOrder      bool
	HasParallelGuidance bool
	HasLoadMultipleWhen bool
}

type workspaceProjectTemplateData struct {
	config.WorkspaceProjectConfig
	SkillName             string
	SkillPath             string
	ProjectSpecPath       string
	SkillSummary          string
	Responsibility        string
	Frameworks            []string
	SelfManaged           bool
	SelfManagedConfigPath string
	UsesChildConfig       bool
	HasFrameworks         bool
}

type childSkillTarget struct {
	OutputPath      string
	UsesChildConfig bool
	ConfigPath      string
}

type workspaceGenerateInputData struct {
	Name       string                          `json:"name"`
	RootPath   string                          `json:"root_path"`
	Projects   []workspaceGenerateInputProject `json:"projects"`
	ConfigPath string                          `json:"config_path,omitempty"`
}

type workspaceGenerateInputProject struct {
	ID              string `json:"id"`
	Path            string `json:"path"`
	Type            string `json:"type"`
	Language        string `json:"language"`
	SkillPath       string `json:"skill_path,omitempty"`
	ProjectSpecPath string `json:"project_spec_path,omitempty"`
	ConfigPath      string `json:"config_path,omitempty"`
	SelfManaged     bool   `json:"self_managed,omitempty"`
}

type workspaceSkillsFingerprintInput struct {
	Kind                string                     `json:"kind"`
	ProgramVersion      string                     `json:"program_version"`
	SkillsTemplatesHash string                     `json:"skills_templates_hash"`
	OutputPath          string                     `json:"output_path"`
	TemplateData        workspaceSkillTemplateData `json:"template_data"`
}
