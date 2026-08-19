package rule

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/rule"
	"github.com/stretchr/testify/require"
)

type optimizerStub struct {
	request *agent.OptimizeRuleRequest
	content string
	summary string
	terms   []string
}

func (s *optimizerStub) OptimizeRule(_ context.Context, req *agent.OptimizeRuleRequest) (*agent.OptimizeContentResult, error) {
	s.request = req
	return &agent.OptimizeContentResult{Content: s.content, Summary: s.summary, RouteTerms: s.terms}, nil
}

func TestUpsertRuleMergesContentAndScopeByDefault(t *testing.T) {
	repo := rule.NewRepository(t.TempDir())
	optimizer := &optimizerStub{content: "初次整理后的规则"}
	service := NewService(repo, optimizer, agent.ProjectContext{Name: "demo", RootPath: "/demo"})
	ctx := context.Background()

	first, err := service.UpsertRule(ctx, UpsertRequest{
		Name:             "foundation",
		Content:          "初始规则",
		AffectedProjects: []string{"service-a"},
		Paths:            []string{"internal/platform/**"},
	})
	require.NoError(t, err)

	optimizer.content = "合并整理后的规则"
	updated, err := service.UpsertRule(ctx, UpsertRequest{
		Name:             "foundation",
		Content:          "新增规则",
		AffectedProjects: []string{"service-a", "service-b"},
		Paths:            []string{"pkg/runtime/**"},
	})
	require.NoError(t, err)
	require.Equal(t, first.CreatedAt, updated.CreatedAt)
	require.Equal(t, "合并整理后的规则", updated.Content)
	require.Equal(t, []string{"service-a", "service-b"}, updated.AffectedProjects)
	require.Equal(t, []string{"internal/platform/**", "pkg/runtime/**"}, updated.Paths)
	require.Equal(t, "初次整理后的规则", optimizer.request.ExistingContent)
	require.Equal(t, "新增规则", optimizer.request.Content)
	require.False(t, optimizer.request.Overwrite)
}

func TestUpsertRuleMergesRoutingMetadata(t *testing.T) {
	repo := rule.NewRepository(t.TempDir())
	optimizer := &optimizerStub{content: "初次规则", summary: "保护边界。", terms: []string{"边界", "修改"}}
	service := NewService(repo, optimizer, agent.ProjectContext{})
	_, err := service.UpsertRule(context.Background(), UpsertRequest{Name: "foundation", Content: "初始"})
	require.NoError(t, err)

	optimizer.content = "合并规则"
	optimizer.summary = ""
	optimizer.terms = []string{"修改", "范围"}
	saved, err := service.UpsertRule(context.Background(), UpsertRequest{Name: "foundation", Content: "新增"})
	require.NoError(t, err)
	require.Equal(t, "保护边界。", saved.Summary)
	require.Equal(t, []string{"边界", "修改", "范围"}, saved.RouteTerms)
}

func TestUpsertRuleOverwriteReplacesContentAndScope(t *testing.T) {
	repo := rule.NewRepository(t.TempDir())
	optimizer := &optimizerStub{content: "初次整理后的规则"}
	service := NewService(repo, optimizer, agent.ProjectContext{Name: "demo", RootPath: "/demo"})
	ctx := context.Background()

	first, err := service.UpsertRule(ctx, UpsertRequest{Name: "foundation", Content: "初始规则", Paths: []string{"internal/platform/**"}})
	require.NoError(t, err)

	optimizer.content = "覆盖整理后的规则"
	updated, err := service.UpsertRule(ctx, UpsertRequest{Name: "foundation", Content: "替换规则", Paths: []string{"pkg/runtime/**"}, Overwrite: true})
	require.NoError(t, err)
	require.Equal(t, first.CreatedAt, updated.CreatedAt)
	require.Equal(t, "覆盖整理后的规则", updated.Content)
	require.Equal(t, []string{"pkg/runtime/**"}, updated.Paths)
	require.Empty(t, optimizer.request.ExistingContent)
	require.Equal(t, "替换规则", optimizer.request.Content)
	require.Equal(t, []string{"pkg/runtime/**"}, optimizer.request.Paths)
	require.True(t, optimizer.request.Overwrite)
}
