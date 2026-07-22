package skills

import "github.com/silaswei-io/skills-seed/internal/metadata"

// TemplateEntry 描述一个逻辑 Skills 模板及其最终生成的标准化文件。
type TemplateEntry struct {
	ID           string   // 逻辑模板 ID，用于目录查询和测试断言
	RelativeName string   // provider 目录下不含语言后缀的相对模板名
	Ext          string   // 模板文件扩展名
	OutputPath   string   // 渲染后写入 Skills 目录的相对路径
	Providers    []string // 支持该模板的 provider，按查找优先级配合 fallback 使用
}

// skillTemplateCatalog 维护所有可生成 Skills 文件的固定目录。
var skillTemplateCatalog = []TemplateEntry{
	{
		ID:           "project-skill",
		RelativeName: "project/skill",
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
		ID:           "project-reference-validation",
		RelativeName: "project/references/validation",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/validation.md",
		Providers:    []string{"common"},
	},
	{
		ID:           "project-reference-testing",
		RelativeName: "project/references/testing",
		Ext:          metadata.SkillsTemplateExt,
		OutputPath:   "references/testing.md",
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
		RelativeName: "workspace/skill",
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

// SkillTemplateCatalog 返回所有逻辑 Skills 模板条目。
func SkillTemplateCatalog() []TemplateEntry {
	entries := make([]TemplateEntry, len(skillTemplateCatalog))
	copy(entries, skillTemplateCatalog)
	return entries
}

// TemplateCatalogEntry 按 ID 返回逻辑 Skills 模板条目。
func TemplateCatalogEntry(id string) (TemplateEntry, bool) {
	for _, entry := range skillTemplateCatalog {
		if entry.ID == id {
			return entry, true
		}
	}
	return TemplateEntry{}, false
}
