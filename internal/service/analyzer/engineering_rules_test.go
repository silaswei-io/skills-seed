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

	got, err := validateEngineeringRules(root, []string{"AGENTS.md"}, false, rules)

	require.NoError(t, err)
	require.Equal(t, rules, got)
}

func TestValidateEngineeringRulesRejectsUncollectedSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("80% coverage"), 0644))

	got, err := validateEngineeringRules(root, []string{"AGENTS.md"}, false, []domain.EngineeringRule{{
		Title:    "覆盖率",
		Rule:     "覆盖率必须达到 80%",
		Source:   "README.md",
		Evidence: []string{"README.md"},
	}})

	require.Nil(t, got)
	require.ErrorContains(t, err, "is not an authoritative engineering knowledge file")
}

func TestValidateEngineeringRulesRejectsSymlinkOutsideProject(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "AGENTS.md")
	require.NoError(t, os.WriteFile(outside, []byte("external rule"), 0644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "AGENTS.md")))

	got, err := validateEngineeringRules(root, []string{"AGENTS.md"}, false, []domain.EngineeringRule{{
		Title:    "验证",
		Rule:     "修改后运行测试",
		Source:   "AGENTS.md",
		Evidence: []string{"AGENTS.md"},
	}})

	require.Nil(t, got)
	require.ErrorContains(t, err, "outside the project root")
}
