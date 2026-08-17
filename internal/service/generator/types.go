package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

const GenerateProjectStepTotal = 5

type GenerateProgressHooks struct {
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

// GenerateOptions 描述单次 Skill 生成的显式输入。
type GenerateOptions struct {
	Progress       GenerateProgressHooks
	ProjectedRules []domain.Rule
}

type generationSummary struct {
	CategorySummaries map[string]categorySummary
	KeyInsights       []string
}

type categorySummary struct {
	Category string
	Summary  string
	Patterns []string
}

type skillTemplateData struct {
	ProgramVersion      string
	SkillsTemplatesHash string
	ProjectName         string
	SkillName           string
	SkillDescription    string
	Language            string
	HasKnowledge        bool
	HasPatterns         bool
	PatternCount        int
	Categories          int
	LastUpdated         string
	KeyInsights         []string
	References          ReferenceAvailability
	OverviewReferences  []skills.ReferenceItem
	ReferenceGroups     []skills.ReferenceGroup
	DevelopmentFocuses  []developmentFocusView
	WorkflowReferences  []WorkflowReference
	RuleReferences      []RuleReference
	StateSummaries      []string
	CommandRules        []domain.EngineeringRule
}

type categoryPatternTemplateData struct {
	Category          string
	Summary           string
	PatternObjects    []patternRenderModel
	ClaimGroups       []knowledge.ClaimGroup
	PatternCount      int
	LastUpdated       string
	CodeFenceLanguage string
	RelatedReferences []PatternReferenceLink
}

type businessIndexTemplateData struct {
	Category          string
	Summary           string
	PatternCount      int
	LastUpdated       string
	DetailGroups      []patternGroup
	InlineGroups      []patternGroup
	RelatedReferences []PatternReferenceLink
}

type businessDetailTemplateData struct {
	Category          string
	GroupTitle        string
	GroupSummary      knowledge.BusinessGroupSummary
	PatternObjects    []patternRenderModel
	PatternCount      int
	LastUpdated       string
	CodeFenceLanguage string
	RelatedReferences []PatternReferenceLink
}

type projectOverviewTemplateData struct {
	domain.ProjectProfile
	OverviewReferences  []skills.ReferenceItem
	OverviewSummary     string
	ArchitectureSummary string
}

type profileReferenceTemplateData struct {
	domain.ProjectProfile
	HasBusinessPatterns bool
	HasUtilityPatterns  bool
	CodeFenceLanguage   string
	BusinessMethodIndex businessMethodIndex
}

type moduleReferenceTemplateData struct {
	domain.ProjectProfile
	KeyModules []domain.ModuleInfo
}

type projectSpecTemplateData struct {
	domain.ProjectSpec
	References     ReferenceAvailability
	RuleReferences []RuleReference
}

type categoryReferenceMeta struct {
	Group       string
	Title       string
	Description string
}

type PatternReferenceLink struct {
	Title  string
	Path   string
	Reason string
}

type patternRenderModel struct {
	domain.Pattern
	HardConstraint      bool
	HighRiskOperational bool
}

func (p patternRenderModel) AllowsHardConstraint() bool {
	return p.HardConstraint
}

type patternGroup struct {
	knowledge.BusinessGroup
	Patterns []patternRenderModel
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

type WorkflowReference struct {
	ID          string
	Name        string
	Path        string
	Description string
}

// RuleReference 描述 Skill 入口可路由的用户规则。
type RuleReference struct {
	ID               string
	Name             string
	Path             string
	AffectedProjects []string
	Paths            []string
}
