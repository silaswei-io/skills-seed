package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
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
	OverviewReferences  []skills.ReferenceItem
	OverviewSummary     string
	ArchitectureSummary string
}

type profileReferenceTemplateData struct {
	domain.ProjectProfile
	HasBusinessPatterns bool
	HasUtilityPatterns  bool
	CodeFenceLanguage   string
}

type projectSpecTemplateData struct {
	domain.ProjectSpec
	References    ReferenceAvailability
	SourceOfTruth []SourceOfTruthItem
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
