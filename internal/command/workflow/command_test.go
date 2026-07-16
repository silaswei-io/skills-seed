package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestWorkflowCommandInWorkspaceDefaultsToRootWorkflow(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	cmd := Cmd(cont)
	cmd.SetArgs([]string{"--name", "release", "--context", "工作区发布前确认 backend 和部署脚本兼容"})

	require.NoError(t, cmd.Execute())

	require.FileExists(t, filepath.Join(cont.SeedPath, "workflows", "release", "WORKFLOW.md"))
	require.NoFileExists(t, filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend", ".skills-seed", "workflows", "release", "WORKFLOW.md"))
}

func TestWorkflowCommandGeneratesNameWhenMissing(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	cmd := Cmd(cont)
	cmd.SetArgs([]string{"--context", "工作区发布前确认 backend 和部署脚本兼容"})

	require.NoError(t, cmd.Execute())

	entries, err := os.ReadDir(filepath.Join(cont.SeedPath, "workflows"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "workflow", entries[0].Name())
	require.FileExists(t, filepath.Join(cont.SeedPath, "workflows", entries[0].Name(), "WORKFLOW.md"))
}

func TestWorkflowCommandInWorkspaceCanTargetChildProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	cmd := Cmd(cont)
	cmd.SetArgs([]string{"--name", "deploy", "--context", "backend 发布前运行数据库迁移检查", "--child", "backend"})

	require.NoError(t, cmd.Execute())

	childSeedPath := filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend", ".skills-seed")
	require.FileExists(t, filepath.Join(childSeedPath, "workflows", "deploy", "WORKFLOW.md"))
	require.NoFileExists(t, filepath.Join(cont.SeedPath, "workflows", "deploy", "WORKFLOW.md"))
}

func TestWorkflowCommandPrintsOptimizeProgress(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	output := captureWorkflowStdout(t, func() {
		cmd := Cmd(cont)
		cmd.SetArgs([]string{"--name", "release", "--context", "工作区发布前确认 backend 和部署脚本兼容"})
		require.NoError(t, cmd.Execute())
	})

	require.Contains(t, output, "优化用户工作流")
}

func TestWorkflowCommandUsesOverwriteFlagOnly(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	cmd := Cmd(cont)
	require.NotNil(t, cmd.Flags().Lookup("overwrite"))
	require.Nil(t, cmd.Flags().Lookup("append"))
}

func TestWorkflowShowListsLightweightSummariesAsJSON(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	now := time.Date(2026, 7, 13, 12, 30, 0, 0, time.UTC)
	require.NoError(t, cont.WorkflowRepo.Save(domain.Workflow{
		ID:   "release",
		Name: "Release",
		Contexts: []domain.WorkflowContext{
			{Content: "发布前检查构建产物", CreatedAt: now},
		},
		Content:   "# Release\n\n## 适用场景\n发布流程覆盖构建产物检查和上线后验证。\n\n## 步骤\n- 检查构建产物",
		CreatedAt: now,
		UpdatedAt: now,
	}))

	cmd := Cmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "--format", "json"})
	require.NoError(t, cmd.Execute())

	var got []workflowSummaryView
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got, 1)
	require.Equal(t, "release", got[0].ID)
	require.Equal(t, "workspace", got[0].Target)
	require.Equal(t, "发布流程覆盖构建产物检查和上线后验证。", got[0].Summary)
	require.Equal(t, 1, got[0].ContextCount)
	require.NotContains(t, out.String(), "\"content\"")
}

func TestWorkflowShowReturnsFullDetailsAsJSON(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	now := time.Date(2026, 7, 13, 12, 30, 0, 0, time.UTC)
	require.NoError(t, cont.WorkflowRepo.Save(domain.Workflow{
		ID:   "release",
		Name: "Release",
		Contexts: []domain.WorkflowContext{
			{Content: "发布前检查构建产物", CreatedAt: now},
		},
		Content:   "# Release\n\n## 适用场景\n发布流程覆盖构建产物检查和上线后验证。",
		CreatedAt: now,
		UpdatedAt: now,
	}))
	require.NoError(t, os.WriteFile(filepath.Join(cont.WorkflowRepo.ScriptsDir("release"), "smoke.sh"), []byte("#!/bin/sh\necho ok\n"), 0755))

	cmd := Cmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "release", "--format", "json"})
	require.NoError(t, cmd.Execute())

	var got workflowDetailView
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Equal(t, "release", got.ID)
	require.Equal(t, "workspace", got.Target)
	require.Contains(t, got.Content, "发布流程覆盖构建产物检查")
	require.Len(t, got.Contexts, 1)
	require.Len(t, got.Scripts, 1)
	require.Equal(t, "smoke.sh", got.Scripts[0].Path)
	require.NotEmpty(t, got.Scripts[0].SHA256)
}

func TestWorkflowShowCanInspectWorkspaceChild(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := newWorkflowWorkspaceContainer(t)
	defer cont.Close()

	childSeedPath := filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend", ".skills-seed")
	childCont, err := container.NewContainer(context.Background(), childSeedPath)
	require.NoError(t, err)
	require.NoError(t, childCont.WorkflowRepo.Save(domain.Workflow{
		ID:      "deploy",
		Name:    "Backend Deploy",
		Content: "# Backend Deploy\n\n## 适用场景\n后端部署前执行数据库迁移检查。",
	}))
	require.NoError(t, childCont.Close())

	cmd := Cmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "--child", "backend", "--format", "json"})
	require.NoError(t, cmd.Execute())

	var got []workflowSummaryView
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got, 1)
	require.Equal(t, "deploy", got[0].ID)
	require.Equal(t, "backend", got[0].Target)
}

func TestWorkflowShowRequiresInitializedProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cmd := Cmd(nil)
	cmd.SetArgs([]string{"show", "--format", "json"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), i18n.Get("ErrNotInitialized"))
}

func newWorkflowWorkspaceContainer(t *testing.T) *container.Container {
	t.Helper()

	restoreFactory := container.RegisterAgentFactoryForTest("mock", func(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) agent.Agent {
		return &mocks.MockAgent{NameVal: "mock", AvailableVal: true}
	})
	t.Cleanup(restoreFactory)

	workspaceRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspaceRoot, ".git"), 0755))
	seedPath := filepath.Join(workspaceRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = "demo"
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.Language = "go"
	cfg.Project.RootPath = workspaceRoot
	cfg.Project.Locale = "zh-CN"
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}}
	require.NoError(t, configRepo.Update(cfg))

	initWorkflowChildProject(t, workspaceRoot, cfg.Workspace.Projects[0])

	cont, err := container.NewContainer(context.Background(), seedPath)
	require.NoError(t, err)
	return cont
}

func initWorkflowChildProject(t *testing.T, workspaceRoot string, project config.WorkspaceProjectConfig) {
	t.Helper()

	childRoot := filepath.Join(workspaceRoot, filepath.FromSlash(project.Path))
	require.NoError(t, os.MkdirAll(filepath.Join(childRoot, ".git"), 0755))
	childSeedPath := filepath.Join(childRoot, ".skills-seed")
	configRepo, err := config.NewRepository(childSeedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = project.ID
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.Language = project.Language
	cfg.Project.RootPath = childRoot
	cfg.Project.Locale = "zh-CN"
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	require.NoError(t, configRepo.Update(cfg))
}

func captureWorkflowStdout(t *testing.T, fn func()) string {
	t.Helper()

	tempFile, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)

	originalStdout := os.Stdout
	os.Stdout = tempFile
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	require.NoError(t, tempFile.Close())
	data, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	return string(data)
}
