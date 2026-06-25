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
		title = "Release Workflow"
	}
	content := "# " + title + "\n\n## 适用场景\n" + req.Context
	if !req.Overwrite && req.ExistingContent != "" {
		content = req.ExistingContent + "\n\n## 补充\n" + req.Context
	}
	return &agent.OptimizeWorkflowResult{Title: title, Content: content}, nil
}

type captureOptimizer struct {
	requests []*agent.OptimizeWorkflowRequest
}

func (m *captureOptimizer) OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
	copied := *req
	m.requests = append(m.requests, &copied)
	title := req.Name
	if title == "" {
		title = "Release Workflow"
	}
	return &agent.OptimizeWorkflowResult{Title: title, Content: "# " + title + "\n\n## 适用场景\n" + req.Context}, nil
}

type fixedTitleOptimizer struct {
	title string
}

func (m fixedTitleOptimizer) OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
	return &agent.OptimizeWorkflowResult{
		Title:   m.title,
		Content: "# " + m.title + "\n\n## 适用场景\n" + req.Context,
	}, nil
}

func TestUpsertWorkflowMergesContextByDefault(t *testing.T) {
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
	require.Len(t, second.Contexts, 2)

	content, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "WORKFLOW.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "发布前检查环境变量")
	require.Contains(t, string(content), "发布后执行 smoke test")
	require.NotContains(t, string(content), "## 用户输入记录")

	meta, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "metadata.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(meta), "发布前检查环境变量")
	require.Contains(t, string(meta), "发布后执行 smoke test")
}

func TestUpsertWorkflowOverwritesContextWhenRequested(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	svc := NewService(repo, mockOptimizer{}, "go")

	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)

	workflow, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:      "deploy",
		Context:   "发布后执行 smoke test",
		Overwrite: true,
	})
	require.NoError(t, err)
	require.Len(t, workflow.Contexts, 1)

	content, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "WORKFLOW.md"))
	require.NoError(t, err)
	require.NotContains(t, string(content), "发布前检查环境变量")
	require.Contains(t, string(content), "发布后执行 smoke test")

	meta, err := os.ReadFile(filepath.Join(seedPath, "workflows", "deploy", "metadata.yaml"))
	require.NoError(t, err)
	require.NotContains(t, string(meta), "发布前检查环境变量")
	require.Contains(t, string(meta), "发布后执行 smoke test")
}

func TestUpsertWorkflowSendsExistingContentWhenMerging(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	optimizer := &captureOptimizer{}
	svc := NewService(repo, optimizer, "go")

	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)

	_, err = svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布后执行 smoke test",
	})
	require.NoError(t, err)

	require.Len(t, optimizer.requests, 2)
	require.Contains(t, optimizer.requests[1].ExistingContent, "发布前检查环境变量")
	require.False(t, optimizer.requests[1].Overwrite)
}

func TestUpsertWorkflowDoesNotSendExistingContentWhenOverwriting(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	optimizer := &captureOptimizer{}
	svc := NewService(repo, optimizer, "go")

	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)

	_, err = svc.UpsertWorkflow(context.Background(), UpsertRequest{
		Name:      "deploy",
		Context:   "发布后执行 smoke test",
		Overwrite: true,
	})
	require.NoError(t, err)

	require.Len(t, optimizer.requests, 2)
	require.Empty(t, optimizer.requests[1].ExistingContent)
	require.True(t, optimizer.requests[1].Overwrite)
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
	require.Equal(t, "Release Workflow", workflow.Name)
	require.Equal(t, "release-workflow", workflow.ID)
	require.FileExists(t, filepath.Join(seedPath, "workflows", workflow.ID, "WORKFLOW.md"))
}

func TestUpsertWorkflowWithoutNameDoesNotUpdateExistingTitleID(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	svc := NewService(repo, fixedTitleOptimizer{title: "deploy"}, "go")

	existing, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "deploy", Context: "发布前检查环境变量"})
	require.NoError(t, err)
	require.Equal(t, "deploy", existing.ID)

	created, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Context: "补充待确认事项"})
	require.NoError(t, err)
	require.Equal(t, "deploy", created.Name)
	require.Equal(t, "deploy-2", created.ID)

	unchanged, err := repo.Get("deploy")
	require.NoError(t, err)
	require.Len(t, unchanged.Contexts, 1)
	require.Equal(t, "发布前检查环境变量", unchanged.Contexts[0].Content)
}

func TestUpsertWorkflowWithoutNameAlwaysCreatesNewWorkflow(t *testing.T) {
	seedPath := t.TempDir()
	repo := workflowstore.NewRepository(seedPath)
	svc := NewService(repo, fixedTitleOptimizer{title: "Jzero Development Workflow"}, "go")

	first, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Context: "改元文件后执行 jzero gen"})
	require.NoError(t, err)

	second, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Context: "改元文件后执行 jzero gen"})
	require.NoError(t, err)

	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, "jzero-development-workflow", first.ID)
	require.Equal(t, first.ID+"-2", second.ID)

	workflows, err := repo.List()
	require.NoError(t, err)
	require.Len(t, workflows, 2)
}

func TestUpsertWorkflowRequiresContext(t *testing.T) {
	svc := NewService(workflowstore.NewRepository(t.TempDir()), mockOptimizer{}, "go")

	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "deploy"})
	require.Error(t, err)
}
