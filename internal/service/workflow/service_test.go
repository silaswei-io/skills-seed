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
}

func (s *optimizerStub) OptimizeWorkflow(_ context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeContentResult, error) {
	s.request = req
	return &agent.OptimizeContentResult{Content: s.content}, nil
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
