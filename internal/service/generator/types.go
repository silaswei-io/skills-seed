package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

const GenerateProjectStepTotal = 6

type GenerateProgressHooks struct {
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

// GenerateOptions 控制生成行为的可选参数
type GenerateOptions struct {
	SkipReferences bool
}

type projectOverviewTemplateData struct {
	domain.ProjectProfile
	OverviewReferences []skills.ReferenceItem
}

type profileReferenceTemplateData struct {
	domain.ProjectProfile
	HasBusinessPatterns bool
	HasUtilityPatterns  bool
}

type projectSpecTemplateData struct {
	domain.ProjectSpec
	References ReferenceAvailability
}

type categoryReferenceMeta struct {
	Group       string
	Title       string
	Description string
}

type ReferenceAvailability struct {
	Enabled          bool
	ProjectSpec      bool
	ProjectOverview  bool
	BusinessMethods  bool
	KeyModules       bool
	CommonUtils      bool
	BusinessPatterns bool
}

type projectSkillsFingerprintInput struct {
	Kind                string                           `json:"kind"`
	ProgramVersion      string                           `json:"program_version"`
	PromptTemplatesHash string                           `json:"prompt_templates_hash"`
	SkillsTemplatesHash string                           `json:"skills_templates_hash"`
	OutputPath          string                           `json:"output_path"`
	ProjectConfig       config.ProjectConfig             `json:"project_config"`
	AgentConfig         config.AgentConfig               `json:"agent_config"`
	SkillsConfig        config.SkillsConfig              `json:"skills_config"`
	Patterns            []domain.Pattern                 `json:"patterns"`
	PatternInsights     map[string]domain.PatternInsight `json:"pattern_insights,omitempty"`
	Profile             *domain.ProjectProfile           `json:"profile,omitempty"`
}

// Stats 统计信息
type Stats struct {
	Total          int
	AvgConfidence  float64
	HighConfidence []domain.Pattern
	Frequent       []domain.Pattern
	ByCategory     map[string][]domain.Pattern
}
