package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/claim"
	"github.com/silaswei-io/skills-seed/internal/knowledge/routing"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

const GenerateProjectStepTotal = 5

type GenerateProgressHooks struct {
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

// GenerateOptions 控制生成行为的可选参数
type GenerateOptions struct {
	SkipReferences bool
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
	PatternCount        int
	Categories          int
	LastUpdated         string
	KeyInsights         []string
	References          ReferenceAvailability
	OverviewReferences  []skills.ReferenceItem
	ReferenceGroups     []skills.ReferenceGroup
	WorkflowReferences  []WorkflowReference
	StateSummaries      []string
}

type categoryPatternTemplateData struct {
	Category          string
	Summary           string
	PatternObjects    []patternRenderModel
	ClaimGroups       []claim.Group
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
	Groups            []patternGroup
	RelatedReferences []PatternReferenceLink
	CoverageWarnings  []CoverageWarning
}

type businessDetailTemplateData struct {
	Category          string
	GroupTitle        string
	GroupSummary      routing.BusinessGroupSummary
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
	References ReferenceAvailability
}

type validationReferenceTemplateData struct {
	Commands []validationCommand
	Matrix   []ValidationMatrixItem
	Gaps     []string
}

type testingReferenceTemplateData struct {
	Inventory sourcecode.GoTestInventory
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

type CoverageWarning struct {
	Title   string
	Path    string
	Message string
}

type patternRenderModel struct {
	domain.Pattern
	HardConstraint bool
}

func (p patternRenderModel) AllowsHardConstraint() bool {
	return p.HardConstraint
}

type patternGroup struct {
	routing.BusinessGroup
	Patterns []patternRenderModel
}

type ValidationMatrixItem struct {
	Area     string
	Command  string
	When     string
	Source   string
	Evidence []string
}

type ReferenceAvailability struct {
	Enabled          bool
	ProjectSpec      bool
	ProjectOverview  bool
	BusinessMethods  bool
	KeyModules       bool
	CommonUtils      bool
	BusinessPatterns bool
	Validation       bool
	Testing          bool
}

type WorkflowReference struct {
	ID          string
	Name        string
	Path        string
	Description string
}
