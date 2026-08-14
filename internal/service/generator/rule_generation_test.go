package generator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	rulestore "github.com/silaswei-io/skills-seed/internal/infra/storage/rule"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestGenerateSkillsProjectsRuleFromSeedIntoReference(t *testing.T) {
	seedPath := t.TempDir()
	ruleRepo := rulestore.NewRepository(seedPath)
	content := "# 底座代码保护\n\n未经明确授权不得修改 `internal/platform/**`。"
	require.NoError(t, ruleRepo.Save(domain.Rule{ID: "foundation", Name: "底座代码保护", Content: content, Paths: []string{"internal/platform/**"}}))
	profileRepo := &mocks.MockProjectProfileRepository{GetFn: func(context.Context) (*domain.ProjectProfile, error) {
		return nil, profilestore.ErrProfileNotFound
	}}
	svc := NewGeneratorService(&mocks.MockPatternRepository{}, profileRepo, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Language: "go"},
	}, nil, ruleRepo)
	output := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), output))

	skill := readGeneratedFile(t, output, "SKILL.md")
	require.Contains(t, skill, "## 用户规则")
	require.Contains(t, skill, "./references/rules/foundation.md")
	require.Contains(t, skill, "internal/platform/**")
	require.Equal(t, content+"\n", readGeneratedFile(t, output, "references", "rules", "foundation.md"))
	require.FileExists(t, filepath.Join(seedPath, "rules", "foundation", "RULE.md"))
}

func TestGenerateSkillsRejectsDuplicateLocalAndProjectedRuleID(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	seedPath := t.TempDir()
	ruleRepo := rulestore.NewRepository(seedPath)
	require.NoError(t, ruleRepo.Save(domain.Rule{ID: "foundation", Name: "本地规则", Content: "遵守本地边界。"}))
	svc := NewGeneratorService(&mocks.MockPatternRepository{}, &mocks.MockProjectProfileRepository{}, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Language: "go"},
	}, nil, ruleRepo)

	err := svc.GenerateSkillsWithOptions(context.Background(), t.TempDir(), GenerateOptions{
		ProjectedRules: []domain.Rule{{ID: "foundation", Name: "工作区规则", Content: "遵守工作区边界。"}},
	})

	require.ErrorContains(t, err, "当前项目规则与工作区投影规则使用了相同 ID：foundation")
}
