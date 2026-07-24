package analyzer

import (
	"os"
	"path/filepath"
	"testing"

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
