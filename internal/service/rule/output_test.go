package rule

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestWriteProjectsOriginalRuleTextWithoutRewriting(t *testing.T) {
	output := t.TempDir()
	content := "# 底座代码保护\n\n未经明确授权不得修改 `internal/platform/**`。"

	require.NoError(t, Write([]domain.Rule{{ID: "foundation", Name: "底座代码保护", Content: content}}, output))

	data, err := os.ReadFile(filepath.Join(output, "rules", "foundation.md"))
	require.NoError(t, err)
	require.Equal(t, content+"\n", string(data))
}

func TestApplicableToProjectRequiresExplicitTarget(t *testing.T) {
	rules := []domain.Rule{
		{ID: "shared", AffectedProjects: []string{"backend", "frontend"}},
		{ID: "backend-path", Paths: []string{"services/backend/**"}},
		{ID: "root-only", Paths: []string{"shared/contracts/**"}},
	}

	got := ApplicableToProject(rules, "backend", "services/backend")

	require.Len(t, got, 2)
	require.Equal(t, "shared", got[0].ID)
	require.Equal(t, "backend-path", got[1].ID)
}
