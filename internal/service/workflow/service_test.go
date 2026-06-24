package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	workflowstore "github.com/silaswei-io/skills-seed/internal/infra/storage/workflow"
	"github.com/stretchr/testify/require"
)

type mockOptimizer struct{}

func (m mockOptimizer) OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
	title := req.Name
	if title == "" {
		title = "自动发布流程"
	}
	content := "# " + title + "\n\n## 适用场景\n" + req.Context
	if req.Append && req.ExistingContent != "" {
		content = req.ExistingContent + "\n\n## 补充\n" + req.Context
	}
	return &agent.OptimizeWorkflowResult{Title: title, Content: content}, nil
}

func TestUpsertWorkflowCreatesAndRewritesContextByDefault(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	svc := NewService(repo, mockOptimizer{}, "go")

	first, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)
	require.Equal(t, "deploy", first.ID)
	require.Len(t, first.Contexts, 1)
	require.DirExists(t, filepath.Join(seedPath, "workflows", "deploy", "scripts"))

	second, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布后执行 smoke test",
	})
	require.NoError(t, err)
	require.Equal(t, "deploy", second.ID)
	require.Len(t, second.Contexts, 1)

	content, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "WORKFLOW.md"))
	require.NoError(t, err)
	require.NotContains(t, string(content), "发布前检查环境变量")
	require.Contains(t, string(content), "发布后执行 smoke test")
	require.NotContains(t, string(content), "## 用户输入记录")

	meta, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "metadata.yaml"))
	require.NoError(t, err)
	require.NotContains(t, string(meta), "发布前检查环境变量")
	require.Contains(t, string(meta), "发布后执行 smoke test")
}

func TestUpsertWorkflowAppendsContextWhenRequested(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	svc := NewService(repo, mockOptimizer{}, "go")

	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)

	workflow, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布后执行 smoke test",
		Append:  true,
	})
	require.NoError(t, err)
	require.Len(t, workflow.Contexts, 2)

	content, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "WORKFLOW.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "发布前检查环境变量")
	require.Contains(t, string(content), "发布后执行 smoke test")

	meta, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "metadata.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(meta), "发布前检查环境变量")
	require.Contains(t, string(meta), "发布后执行 smoke test")
}

func TestWorkflowRoundTripDoesNotMixScriptsIntoContexts(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	svc := NewService(repo, mockOptimizer{}, "go")

	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "workflows", "deploy", "scripts", "smoke.sh"), []byte("#!/bin/sh\necho ok\n"), 0755))

	workflow, err := repo.Get("deploy")
	require.NoError(t, err)
	require.Len(t, workflow.Contexts, 1)
	require.Equal(t, "发布前检查环境变量", workflow.Contexts[0].Content)
	require.Len(t, workflow.Scripts, 1)
	require.Equal(t, "smoke.sh", workflow.Scripts[0].Path)
}

func TestUpsertWorkflowGeneratesNameWhenMissing(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	svc := NewService(repo, mockOptimizer{}, "go")

	workflow, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Context: "发布前检查环境变量"})
	require.NoError(t, err)
	require.Equal(t, "自动发布流程", workflow.Name)
	require.Regexp(t, `^workflow-[a-f0-9]{12}$`, workflow.ID)
	require.FileExists(t, filepath.Join(seedPath, "workflows", workflow.ID, "WORKFLOW.md"))
}

func TestUpsertWorkflowRequiresContext(t *testing.T) {
	svc := NewService(workflowstore.NewRepository(t.TempDir()), mockOptimizer{}, "go")

	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "deploy"})
	require.Error(t, err)
}
