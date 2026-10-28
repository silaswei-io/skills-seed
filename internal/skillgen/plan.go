package skillgen

type TemplateKind string

const (
	CatalogTemplate         TemplateKind = "catalog"
	ReferenceTemplate       TemplateKind = "reference_file"
	PatternTemplate         TemplateKind = "pattern"
	ProjectOverviewTemplate TemplateKind = "project_overview"
	RelativeTemplate        TemplateKind = "relative"
)

type File struct {
	Path     string
	Kind     TemplateKind
	Template string
	Data     any
}

type Plan struct {
	OutputPath        string
	Files             []File
	AgentMetadataData any
}

func NewPlan(outputPath string) *Plan {
	return &Plan{OutputPath: outputPath}
}

func (p *Plan) AddFile(path string, kind TemplateKind, template string, data any) {
	p.Files = append(p.Files, File{
		Path:     path,
		Kind:     kind,
		Template: template,
		Data:     data,
	})
}
