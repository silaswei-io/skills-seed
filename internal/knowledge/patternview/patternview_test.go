package patternview

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRenderCompactsEquivalentPatternsWithoutDroppingEvidence(t *testing.T) {
	first := newPattern("auth-error-wrap", "Error Wrapping", domain.CategoryError)
	first.Confidence = 0.8
	first.SetDescription("Repository errors are wrapped with operation context before returning.")
	first.SetRule("When repository calls fail, keep operation context in returned errors.")
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/auth/repo.go", Line: 10, Symbol: "LoadAuth", Kind: "func"}}

	second := newPattern("order-error-wrap", "Error Wrapping", domain.CategoryError)
	second.Confidence = 0.9
	second.SetDescription("Repository errors are wrapped with operation context before returning.")
	second.SetRule("When repository calls fail, keep operation context in returned errors.")
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/order/repo.go", Line: 20, Symbol: "LoadOrder", Kind: "func"}}

	view := Render([]domain.Pattern{*first, *second})

	require.Len(t, view, 1)
	require.Equal(t, "order-error-wrap", view[0].ID)
	require.ElementsMatch(t, []string{"auth-error-wrap", "order-error-wrap"}, view[0].MergedFrom)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), view[0].EvidenceLocations)
}

func TestRenderKeepsWorkspaceScopesSeparate(t *testing.T) {
	first := newPattern("api-error-wrap", "Error Wrapping", domain.CategoryError)
	first.ProjectID = "api"
	first.ScopePath = "services/api"
	first.WorkspaceRole = "service"
	first.SetDescription("Repository errors are wrapped with operation context before returning.")
	first.SetRule("When repository calls fail, keep operation context in returned errors.")

	second := newPattern("admin-error-wrap", "Error Wrapping", domain.CategoryError)
	second.ProjectID = "admin"
	second.ScopePath = "services/admin"
	second.WorkspaceRole = "service"
	second.SetDescription("Repository errors are wrapped with operation context before returning.")
	second.SetRule("When repository calls fail, keep operation context in returned errors.")

	view := Render([]domain.Pattern{*first, *second})

	require.Len(t, view, 2)
	require.Equal(t, "api-error-wrap", view[0].ID)
	require.Equal(t, "admin-error-wrap", view[1].ID)
}

func TestRenderPreservesFirstSeenScopeOrder(t *testing.T) {
	backend := newPattern("backend-rule", "Backend Rule", domain.CategoryBusiness)
	backend.ProjectID = "backend"
	backend.ScopePath = "services/backend"

	frontend := newPattern("frontend-rule", "Frontend Rule", domain.CategoryBusiness)
	frontend.ProjectID = "frontend"
	frontend.ScopePath = "apps/frontend"

	view := Render([]domain.Pattern{*frontend, *backend})

	require.Len(t, view, 2)
	require.Equal(t, "frontend-rule", view[0].ID)
	require.Equal(t, "backend-rule", view[1].ID)
}

func TestCompactDoesNotOverwriteDistinctPatternsWithBlankID(t *testing.T) {
	first := newPattern("", "Token Refresh", domain.CategoryBusiness)
	first.SetRule("Refresh access tokens through the session capability.")

	second := newPattern("", "Audit Export", domain.CategoryBusiness)
	second.SetRule("Export audit records through the audit capability.")

	view := Compact([]domain.Pattern{*first, *second}, nil)

	require.Len(t, view, 2)
	require.Equal(t, "Token Refresh", view[0].Name)
	require.Equal(t, "Audit Export", view[1].Name)
}

func TestCompactMergesEquivalentPatternsWithBlankID(t *testing.T) {
	first := newPattern("", "Session Token Refresh", domain.CategoryBusiness)
	first.Confidence = 0.7
	first.SetDescription("Session token refresh preserves the existing session boundary.")
	first.SetRule("When changing session token refresh, preserve the existing session boundary.")
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/session/refresh.ts", Line: 10}}

	second := newPattern("", "Session Token Refresh", domain.CategoryBusiness)
	second.Confidence = 0.9
	second.SetDescription("Session token refresh preserves the existing session boundary.")
	second.SetRule("When changing session token refresh, preserve the existing session boundary.")
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/session/controller.ts", Line: 20}}

	view := Compact([]domain.Pattern{*first, *second}, nil)

	require.Len(t, view, 1)
	require.Equal(t, "Session Token Refresh", view[0].Name)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), view[0].EvidenceLocations)
}

func TestRenderKeepsHighRiskBoundarySeparate(t *testing.T) {
	update := newPattern("resource-update", "Resource Lifecycle Update", domain.CategoryBusiness)
	update.Confidence = 0.9
	update.SetDescription("Update resource lifecycle state through the verified capability entry.")
	update.SetRule("When changing resource state, inspect the lifecycle entry.")
	update.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/resource/lifecycle.ts", Symbol: "updateResource"}}

	destroy := newPattern("resource-destroy", "Resource Lifecycle Destroy", domain.CategoryBusiness)
	destroy.Confidence = 0.9
	destroy.SetDescription("Destroy command deletes resource state and has external environment side effects.")
	destroy.SetRule("When changing destroy behavior, inspect the command safeguards before modifying it.")
	destroy.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "tools/commands/destroy.ts", Symbol: "destroyResource"}}

	view := Render([]domain.Pattern{*update, *destroy})

	require.Len(t, view, 2)
	require.Equal(t, "resource-update", view[0].ID)
	require.Equal(t, "resource-destroy", view[1].ID)
}

func TestRenderCompactsPatternsWithSharedCoreEvidence(t *testing.T) {
	first := newPattern("hotback-state-lock", "热备编辑状态锁机制", domain.CategoryConcurrency)
	first.SetDescription("热备系统使用全局状态机协调编辑操作与定时任务，防止VIP漂移或编辑过程中定时任务干扰。")
	first.SetRule("角色切换流程开始时设置EditingStateEditing，完成后通过defer恢复为Idle状态。")
	first.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/hacontrol/hotback_classical/controller.go", Line: 26, Symbol: "SetEditingState", Kind: "func"},
		{Path: "internal/service/hacontrol/hotback_classical/controller.go", Line: 40, Symbol: "IsEditing", Kind: "func"},
		{Path: "internal/service/hacontrol/hotback_classical/transition.go", Line: 118, Symbol: "masterToBackup", Kind: "func"},
	}

	second := newPattern("hotback-edit-lock", "热备编辑锁状态管理", domain.CategoryConcurrency)
	second.SetDescription("热备编辑锁用于协调用户配置编辑与后台定时任务的并发冲突。")
	second.SetRule("编辑开始时设置EditingStateEditing并在defer中恢复Idle，IsEditing判断编辑状态。")
	second.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/hacontrol/hotback_classical/controller.go", Line: 26, Symbol: "SetEditingState", Kind: "func"},
		{Path: "internal/service/hacontrol/hotback_classical/controller.go", Line: 40, Symbol: "IsEditing", Kind: "func"},
		{Path: "internal/service/hacontrol/hotback_classical/edit.go", Line: 121, Symbol: "EditHotbackConfig", Kind: "func"},
	}

	view := Render([]domain.Pattern{*first, *second})

	require.Len(t, view, 1)
	require.ElementsMatch(t, []string{"hotback-state-lock", "hotback-edit-lock"}, view[0].MergedFrom)
	require.Len(t, view[0].EvidenceLocations, 4)
}

func TestSameScopeUsesOnlySemanticWorkspaceScope(t *testing.T) {
	left := domain.Pattern{}
	right := domain.Pattern{}
	require.False(t, SameScope(left, right))

	left.ScopePath = "services/ca-admin"
	right.ScopePath = "services/ca-admin"
	require.True(t, SameScope(left, right))

	right.ScopePath = "services/other"
	require.False(t, SameScope(left, right))
}

func TestTokensMatchReorderedChinesePhrases(t *testing.T) {
	left := Tokens("用户状态校验")
	right := Tokens("校验用户状态")

	require.Greater(t, jaccard(left, right), 0.3)
}

func newPattern(id, name string, category domain.Category) *domain.Pattern {
	pattern := domain.NewPattern(id, name, category)
	pattern.Confidence = 0.9
	pattern.SetDescription(name + " description")
	pattern.SetRule(name + " rule")
	return pattern
}
