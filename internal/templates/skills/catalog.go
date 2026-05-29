package skills

import "github.com/silaswei-io/skills-seed/internal/metadata"

// TemplateEntry describes one logical skills template and the normalized file it produces.
type TemplateEntry struct {
	ID           string
	RelativeName string
	Ext          string
	OutputPath   string
	Providers    []string
}

var skillTemplateCatalog = []TemplateEntry{
	{
		ID:           "project-skill",
		RelativeName: "project/project-skill",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "SKILL.md",
		Providers:    []string{"common", "claude", "codex"},
	},
	{
		ID:           "project-reference-overview",
		RelativeName: "project/references/project-overview",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/project-overview.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-reference-project-spec",
		RelativeName: "project/references/project-spec",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/project-spec.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-reference-business-methods",
		RelativeName: "project/references/business-methods",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/business-methods.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-reference-modules",
		RelativeName: "project/references/modules",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/modules.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-reference-common-utils",
		RelativeName: "project/references/common-utils",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/common-utils.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-pattern-business",
		RelativeName: "project/references/patterns/business",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/patterns/business.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-pattern-default",
		RelativeName: "project/references/patterns/default",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/patterns/default.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-pattern-utils",
		RelativeName: "project/references/patterns/utils",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/patterns/utils.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "workspace-skill",
		RelativeName: "workspace/workspace-skill",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "SKILL.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "workspace-reference-overview",
		RelativeName: "workspace/references/workspace-overview",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/workspace-overview.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "workspace-reference-cross-project-rules",
		RelativeName: "workspace/references/cross-project-rules",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/cross-project-rules.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "codex-agent-openai",
		RelativeName: "project/agents/codex-agent-openai",
		Ext:          ".yaml.tmpl",
		OutputPath:   "agents/openai.yaml",
		Providers:    []string{"codex"},
	},
}

// SkillTemplateCatalog returns all logical skills template entries.
func SkillTemplateCatalog() []TemplateEntry {
	entries := make([]TemplateEntry, len(skillTemplateCatalog))
	copy(entries, skillTemplateCatalog)
	return entries
}

// TemplateCatalogEntry returns a logical skills template entry by ID.
func TemplateCatalogEntry(id string) (TemplateEntry, bool) {
	for _, entry := range skillTemplateCatalog {
		if entry.ID == id {
			return entry, true
		}
	}
	return TemplateEntry{}, false
}
