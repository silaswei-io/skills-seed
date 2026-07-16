package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
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

type projectOverviewTemplateData struct {
	domain.ProjectProfile
	OverviewReferences  []skills.ReferenceItem
	OverviewSummary     string
	ArchitectureSummary string
	ValidationMatrix    []ValidationMatrixItem
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
	References       ReferenceAvailability
	SourceOfTruth    []SourceOfTruthItem
	ValidationMatrix []ValidationMatrixItem
}

type SourceOfTruthItem struct {
	Area      string
	Edit      string
	DoNotEdit string
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

type ValidationMatrixItem struct {
	Area        string
	Command     string
	When        string
	Source      string
	Evidence    []string
	Confidence  float64
	Coverage    float64
	MatchKind   string
	Recommended bool
	Warning     string
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

// Stats 统计信息
type Stats struct {
	Total          int
	AvgConfidence  float64
	HighConfidence []domain.Pattern
	Frequent       []domain.Pattern
	ByCategory     map[string][]domain.Pattern
}
