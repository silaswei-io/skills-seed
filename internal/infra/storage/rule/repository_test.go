package rule

import (
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRepositoryPersistsRuleTextAndExplicitScope(t *testing.T) {
	seedPath := t.TempDir()
	repo := NewRepository(seedPath)
	rule := domain.Rule{
		ID:               "foundation-code",
		Name:             "底座代码保护",
		Content:          "# 底座代码保护\n\n未经明确授权不得修改。",
		AffectedProjects: []string{"backend", "frontend"},
		Paths:            []string{"shared/contracts/**"},
	}

	require.NoError(t, repo.Save(rule))

	stored, err := repo.Get(rule.ID)
	require.NoError(t, err)
	require.Equal(t, rule.Name, stored.Name)
	require.Equal(t, rule.Content, stored.Content)
	require.Equal(t, rule.AffectedProjects, stored.AffectedProjects)
	require.Equal(t, rule.Paths, stored.Paths)
	require.FileExists(t, filepath.Join(seedPath, "rules", rule.ID, "RULE.md"))
	require.FileExists(t, filepath.Join(seedPath, "rules", rule.ID, "metadata.yaml"))
}
