package rule

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestRuleCommandStoresWorkspaceRuleWithExplicitTargets(t *testing.T) {
	cont := newWorkspaceContainer(t)
	defer cont.Close()
	cmd := Cmd(cont)
	cmd.SetArgs([]string{"--name", "shared-contract", "--content", "修改公共契约前必须确认两个消费者。", "--project", "backend,frontend"})

	require.NoError(t, cmd.Execute())

	rule, err := cont.RuleRepo.Get("shared-contract")
	require.NoError(t, err)
	require.Equal(t, []string{"backend", "frontend"}, rule.AffectedProjects)
	require.FileExists(t, filepath.Join(cont.SeedPath, "rules", "shared-contract", "RULE.md"))
}

func TestRuleCommandStoresWorkspaceRootRuleWithoutExplicitScope(t *testing.T) {
	cont := newWorkspaceContainer(t)
	defer cont.Close()
	cmd := Cmd(cont)
	cmd.SetArgs([]string{"--name", "foundation", "--content", "底座代码不能修改。"})

	require.NoError(t, cmd.Execute())

	saved, err := cont.RuleRepo.Get("foundation")
	require.NoError(t, err)
	require.Empty(t, saved.AffectedProjects)
	require.Empty(t, saved.Paths)
	require.Equal(t, "底座代码不能修改。", saved.Content)
}

func TestRuleCommandStoresSingleChildRuleInChildSeed(t *testing.T) {
	cont := newWorkspaceContainer(t)
	defer cont.Close()
	cmd := Cmd(cont)
	cmd.SetArgs([]string{"--name", "backend-boundary", "--content", "不得直接依赖存储实现。", "--child", "backend"})

	require.NoError(t, cmd.Execute())

	require.FileExists(t, filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend", ".skills-seed", "rules", "backend-boundary", "RULE.md"))
	require.NoFileExists(t, filepath.Join(cont.SeedPath, "rules", "backend-boundary", "RULE.md"))
}

func newWorkspaceContainer(t *testing.T) *container.Container {
	t.Helper()
	require.NoError(t, i18n.Init("zh-CN"))
	restore := container.RegisterAgentFactoryForTest("mock-rule", func(container.AgentFactoryOptions) agent.Agent {
		return &mocks.MockAgent{NameVal: "mock-rule", AvailableVal: true}
	})
	t.Cleanup(restore)
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Language: "go"},
		{ID: "frontend", Path: "frontend", Language: "typescript"},
	}
	for _, project := range projects {
		childRoot := filepath.Join(root, project.Path)
		require.NoError(t, os.MkdirAll(filepath.Join(childRoot, ".git"), 0o755))
		childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
		require.NoError(t, err)
		cfg := childConfig.Get()
		cfg.Project = config.ProjectConfig{Name: project.ID, Mode: domain.ModeProject, Language: project.Language, RootPath: childRoot, Locale: "zh-CN"}
		cfg.Agent.Engine = "mock-rule"
		cfg.Agent.Commands = map[string]string{"mock-rule": "mock-rule"}
		require.NoError(t, childConfig.Update(cfg))
	}
	seedPath := filepath.Join(root, ".skills-seed")
	repo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := repo.Get()
	cfg.Project = config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: root, Locale: "zh-CN"}
	cfg.Workspace.Projects = projects
	cfg.Agent.Engine = "mock-rule"
	cfg.Agent.Commands = map[string]string{"mock-rule": "mock-rule"}
	require.NoError(t, repo.Update(cfg))
	cont, err := container.NewContainer(context.Background(), seedPath)
	require.NoError(t, err)
	return cont
}
