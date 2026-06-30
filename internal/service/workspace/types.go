package workspace

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
)

// WorkspaceGenerateOptions 控制工作区根 skill 的生成行为。
type WorkspaceGenerateOptions struct {
	RootOutputPath string // 覆盖工作区根 skill 输出目录；为空时使用配置推导路径。
	SkipReferences bool   // 是否跳过 references/ 明细文档生成。
}

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
	WorkflowReferences  []generator.WorkflowReference
	SkipReferences      bool
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
	HasWorkflowRefs     bool
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
