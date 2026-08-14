package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestValidateEngineeringRulesAcceptsCollectedAuthority(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("run tests"), 0644))
	rules := []domain.EngineeringRule{{
		Title:    "验证",
		Rule:     "修改后运行测试",
		Source:   "AGENTS.md",
		Evidence: []string{"AGENTS.md"},
	}}

	got, issues := validateEngineeringRules(root, []string{"AGENTS.md"}, false, rules)

	require.Empty(t, issues)
	require.Equal(t, rules, got)
}

func TestValidateEngineeringRulesDropsUncollectedSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("80% coverage"), 0644))

	got, issues := validateEngineeringRules(root, []string{"AGENTS.md"}, false, []domain.EngineeringRule{{
		Title:    "覆盖率",
		Rule:     "覆盖率必须达到 80%",
		Source:   "README.md",
		Evidence: []string{"README.md"},
	}})

	require.Empty(t, got)
	require.Len(t, issues, 1)
	require.ErrorContains(t, issues[0], "is not an authoritative engineering knowledge file")
}

func TestValidateEngineeringRulesDropsSymlinkOutsideProject(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "AGENTS.md")
	require.NoError(t, os.WriteFile(outside, []byte("external rule"), 0644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "AGENTS.md")))

	got, issues := validateEngineeringRules(root, []string{"AGENTS.md"}, false, []domain.EngineeringRule{{
		Title:    "验证",
		Rule:     "修改后运行测试",
		Source:   "AGENTS.md",
		Evidence: []string{"AGENTS.md"},
	}})

	require.Empty(t, got)
	require.Len(t, issues, 1)
	require.ErrorContains(t, issues[0], "outside the project root")
}

func TestValidateEngineeringRulesKeepsValidRulesWhenDroppingInvalidOnes(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("run tests"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("ordinary prose"), 0644))

	got, issues := validateEngineeringRules(root, []string{"AGENTS.md"}, false, []domain.EngineeringRule{
		{
			Title:    "验证",
			Rule:     "修改后运行测试",
			Source:   "AGENTS.md",
			Evidence: []string{"AGENTS.md"},
		},
		{
			Title:    "普通文档",
			Rule:     "不要把普通 README 当硬规则",
			Source:   "README.md",
			Evidence: []string{"README.md"},
		},
	})

	require.Len(t, got, 1)
	require.Equal(t, "AGENTS.md", got[0].Source)
	require.Len(t, issues, 1)
	require.ErrorContains(t, issues[0], "README.md")
}

func TestCollectAuthorityCoverageBuildsMarkdownSectionsAndFileRecords(t *testing.T) {
	root := t.TempDir()
	content := "# Project Rules\n\n## Services\nUse interfaces.\n\n```md\n## Not A Section\n```\n\n## HSM\nCheck return values.\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Taskfile.yml"), []byte("version: '3'\n"), 0o644))

	got, err := collectAuthorityCoverage(root, []string{"Taskfile.yml", "./AGENTS.md", "AGENTS.md"})

	require.NoError(t, err)
	require.Equal(t, []domain.AuthorityCoverage{
		{Source: "AGENTS.md", Sections: []string{"Project Rules", "Services", "HSM"}},
		{Source: "Taskfile.yml"},
	}, got)
}

func TestCollectAuthorityCoverageRejectsInvalidPath(t *testing.T) {
	root := t.TempDir()

	got, err := collectAuthorityCoverage(root, []string{"../AGENTS.md"})

	require.ErrorContains(t, err, "invalid authority path")
	require.Empty(t, got)
}

func TestCollectAuthorityCoverageRejectsMissingNonMarkdownFile(t *testing.T) {
	root := t.TempDir()

	got, err := collectAuthorityCoverage(root, []string{"Taskfile.yml"})

	require.ErrorContains(t, err, "stat authority path")
	require.Empty(t, got)
}

func TestBuildAuthoritySectionsCreatesStableCatalog(t *testing.T) {
	coverage := []domain.AuthorityCoverage{
		{Source: "GUIDE.md", Sections: []string{"Contracts", "Notes"}},
		{Source: "Taskfile.yml"},
	}

	got := buildAuthoritySections(coverage, true)

	require.Len(t, got, 4)
	require.Equal(t, agent.AuthoritySection{ID: newAuthoritySection("GUIDE.md", "Contracts").ID, Source: "GUIDE.md", Section: "Contracts"}, got[0])
	require.Equal(t, agent.AuthoritySection{ID: newAuthoritySection("GUIDE.md", "Notes").ID, Source: "GUIDE.md", Section: "Notes"}, got[1])
	require.Equal(t, agent.AuthoritySection{ID: newAuthoritySection("Taskfile.yml", "").ID, Source: "Taskfile.yml"}, got[2])
	require.Equal(t, agent.AuthoritySection{ID: newAuthoritySection("user_context", "").ID, Source: "user_context"}, got[3])
	require.Equal(t, newAuthoritySection("GUIDE.md", "Contracts").ID, buildAuthoritySections([]domain.AuthorityCoverage{{Source: "GUIDE.md", Sections: []string{"Contracts"}}}, false)[0].ID)
}

func TestExpandAuthoritySectionsAssignsProgramOwnedAuthority(t *testing.T) {
	catalog := []agent.AuthoritySection{
		newAuthoritySection("GUIDE.md", "Contracts"),
		newAuthoritySection("GUIDE.md", "Notes"),
	}

	got, err := expandAuthoritySections(catalog, []agent.AuthoritySectionResult{
		{
			SectionID: catalog[0].ID,
			Rules: []domain.EngineeringRule{{
				Title:     "Preserve contract",
				Rule:      "Keep the public contract compatible.",
				AppliesTo: []string{"public interfaces"},
			}},
		},
		{SectionID: catalog[1].ID, NoRuleReason: "This section contains explanatory background only."},
	})

	require.NoError(t, err)
	require.Equal(t, []domain.EngineeringRule{{
		Title:     "Preserve contract",
		Rule:      "Keep the public contract compatible.",
		Source:    "GUIDE.md",
		Section:   "Contracts",
		AppliesTo: []string{"public interfaces"},
		Evidence:  []string{"GUIDE.md"},
	}}, got)
}

func TestExpandAuthoritySectionsKeepsSameTitleRulesOwnedByDifferentSections(t *testing.T) {
	catalog := []agent.AuthoritySection{
		newAuthoritySection("GUIDE.md", "Commands"),
		newAuthoritySection("GUIDE.md", "Generated Files"),
	}

	got, err := expandAuthoritySections(catalog, []agent.AuthoritySectionResult{
		{
			SectionID: catalog[0].ID,
			Rules:     []domain.EngineeringRule{{Title: "Restrictions", Rule: "Request authorization before executing governed commands."}},
		},
		{
			SectionID: catalog[1].ID,
			Rules:     []domain.EngineeringRule{{Title: "Restrictions", Rule: "Do not edit generated artifacts directly."}},
		},
	})

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "Commands", got[0].Section)
	require.Equal(t, "Generated Files", got[1].Section)
	require.Equal(t, []string{"GUIDE.md"}, got[0].Evidence)
	require.Equal(t, []string{"GUIDE.md"}, got[1].Evidence)
}

func TestExpandAuthoritySectionsDoesNotInventRepositoryEvidenceForUserContext(t *testing.T) {
	catalog := []agent.AuthoritySection{newAuthoritySection("user_context", "")}

	got, err := expandAuthoritySections(catalog, []agent.AuthoritySectionResult{{
		SectionID: catalog[0].ID,
		Rules:     []domain.EngineeringRule{{Title: "Temporary constraint", Rule: "Keep the requested boundary."}},
	}})

	require.NoError(t, err)
	require.Equal(t, "user_context", got[0].Source)
	require.Empty(t, got[0].Evidence)
}

func TestExpandAuthoritySectionsRejectsUnknownSection(t *testing.T) {
	_, err := expandAuthoritySections(
		[]agent.AuthoritySection{newAuthoritySection("GUIDE.md", "Contracts")},
		[]agent.AuthoritySectionResult{{SectionID: "authority-unknown", NoRuleReason: "No constraint is present."}},
	)

	require.ErrorContains(t, err, "unknown section_id")
}

func TestExpandAuthoritySectionsRejectsDuplicateSection(t *testing.T) {
	section := newAuthoritySection("GUIDE.md", "Contracts")
	_, err := expandAuthoritySections([]agent.AuthoritySection{section}, []agent.AuthoritySectionResult{
		{SectionID: section.ID, NoRuleReason: "No constraint is present."},
		{SectionID: section.ID, NoRuleReason: "This section contains background only."},
	})

	require.ErrorContains(t, err, "duplicated")
}

func TestExpandAuthoritySectionsRejectsMissingSection(t *testing.T) {
	catalog := []agent.AuthoritySection{
		newAuthoritySection("GUIDE.md", "Contracts"),
		newAuthoritySection("GUIDE.md", "Notes"),
	}

	_, err := expandAuthoritySections(catalog, []agent.AuthoritySectionResult{{
		SectionID:    catalog[0].ID,
		NoRuleReason: "No constraint is present.",
	}})

	require.ErrorContains(t, err, "was not returned")
}

func TestExpandAuthoritySectionsIgnoresNoRuleReasonWhenRulesExist(t *testing.T) {
	section := newAuthoritySection("GUIDE.md", "Contracts")
	got, err := expandAuthoritySections([]agent.AuthoritySection{section}, []agent.AuthoritySectionResult{{
		SectionID:    section.ID,
		Rules:        []domain.EngineeringRule{{Title: "Preserve contract", Rule: "Keep compatibility."}},
		NoRuleReason: "No constraint is present.",
	}})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Preserve contract", got[0].Title)
}

func TestExpandAuthoritySectionsRejectsPlaceholderNoRuleReason(t *testing.T) {
	section := newAuthoritySection("GUIDE.md", "Notes")
	for _, reason := range []string{"", "N/A", "none", "无规则"} {
		t.Run(reason, func(t *testing.T) {
			_, err := expandAuthoritySections([]agent.AuthoritySection{section}, []agent.AuthoritySectionResult{{
				SectionID:    section.ID,
				NoRuleReason: reason,
			}})

			require.ErrorContains(t, err, "must contain rules or a no-rule reason")
		})
	}
}

func TestValidateEngineeringRulesPreservesCommandPolicyAndScope(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("do not run commands"), 0o644))

	got, issues := validateEngineeringRules(root, []string{"AGENTS.md"}, false, []domain.EngineeringRule{{
		Title:         "Command execution",
		Rule:          "Only describe validation commands unless authorized in this turn.",
		Source:        "AGENTS.md",
		Section:       "Validation",
		AppliesTo:     []string{"go test", "task build"},
		CommandPolicy: domain.CommandPolicyRequiresAuthorization,
		Evidence:      []string{"AGENTS.md"},
	}})

	require.Empty(t, issues)
	require.Len(t, got, 1)
	require.Equal(t, domain.CommandPolicyRequiresAuthorization, got[0].CommandPolicy)
	require.Equal(t, []string{"go test", "task build"}, got[0].AppliesTo)
}
