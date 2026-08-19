package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/workflow"
	"github.com/stretchr/testify/require"
)

type optimizerStub struct {
	request *agent.OptimizeWorkflowRequest
	content string
	summary string
	terms   []string
}

func (s *optimizerStub) OptimizeWorkflow(_ context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeContentResult, error) {
	s.request = req
	return &agent.OptimizeContentResult{Content: s.content, Summary: s.summary, RouteTerms: s.terms}, nil
}

func TestUpsertWorkflowMergesByDefaultAndPreservesScripts(t *testing.T) {
	root := t.TempDir()
	repo := workflow.NewRepository(root)
	optimizer := &optimizerStub{content: "# Initial"}
	svc := NewService(repo, optimizer, agent.ProjectContext{Name: "demo", RootPath: "/demo"})
	first, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "deploy", Content: "# Deploy"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repo.ScriptsDir("deploy"), "verify.sh"), []byte("exit 0\n"), 0o755))

	optimizer.content = "# Merged"
	saved, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "deploy", Content: "Add smoke checks"})
	require.NoError(t, err)
	require.Equal(t, first.CreatedAt, saved.CreatedAt)
	require.Equal(t, "# Merged", saved.Content)
	require.Equal(t, "# Initial", optimizer.request.ExistingContent)
	require.Equal(t, "Add smoke checks", optimizer.request.Content)
	require.False(t, optimizer.request.Overwrite)
	require.Len(t, saved.Scripts, 1)
	require.Equal(t, "verify.sh", saved.Scripts[0].Path)
	require.Equal(t, "demo", optimizer.request.Project.Name)
}

func TestUpsertWorkflowMergesRoutingMetadata(t *testing.T) {
	repo := workflow.NewRepository(t.TempDir())
	optimizer := &optimizerStub{content: "初次", summary: "执行验证。", terms: []string{"验证", "变更"}}
	svc := NewService(repo, optimizer, agent.ProjectContext{})
	_, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "verify", Content: "初始"})
	require.NoError(t, err)

	optimizer.content = "合并"
	optimizer.summary = ""
	optimizer.terms = []string{"变更", "结果"}
	saved, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "verify", Content: "新增"})
	require.NoError(t, err)
	require.Equal(t, "执行验证。", saved.Summary)
	require.Equal(t, []string{"验证", "变更", "结果"}, saved.RouteTerms)
}

func TestUpsertWorkflowOverwriteDoesNotSendExistingContent(t *testing.T) {
	repo := workflow.NewRepository(t.TempDir())
	optimizer := &optimizerStub{content: "# Initial"}
	svc := NewService(repo, optimizer, agent.ProjectContext{Name: "demo", RootPath: "/demo"})
	first, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "deploy", Content: "# Deploy"})
	require.NoError(t, err)

	optimizer.content = "# Replaced"
	saved, err := svc.UpsertWorkflow(context.Background(), UpsertRequest{Name: "deploy", Content: "# Updated", Overwrite: true})
	require.NoError(t, err)
	require.Equal(t, first.CreatedAt, saved.CreatedAt)
	require.Equal(t, "# Replaced", saved.Content)
	require.Empty(t, optimizer.request.ExistingContent)
	require.Equal(t, "# Updated", optimizer.request.Content)
	require.True(t, optimizer.request.Overwrite)
}
