package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAssembleProfileKeepsConfiguredIdentity(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ca-admin"), 0755))
	base := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "ca-admin", Path: "ca-admin", Type: "backend", Language: "go"}},
	}
	analysis := &domain.WorkspaceProfile{
		Name:     "invented",
		RootPath: "/invented",
		Projects: []domain.WorkspaceProject{{
			ID:             "ca-admin",
			Path:           "wrong",
			Type:           "frontend",
			Language:       "typescript",
			Responsibility: "证书管理",
			Frameworks:     []string{"Gin"},
		}},
	}

	profile, err := AssembleProfile(base, analysis)

	require.NoError(t, err)
	require.Equal(t, "demo", profile.Name)
	require.Equal(t, root, profile.RootPath)
	require.Equal(t, "ca-admin", profile.Projects[0].Path)
	require.Equal(t, "backend", profile.Projects[0].Type)
	require.Equal(t, "go", profile.Projects[0].Language)
	require.Equal(t, "证书管理", profile.Projects[0].Responsibility)
}

func TestAssembleProfileRejectsUnknownProjectBeforePersistence(t *testing.T) {
	base := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: t.TempDir(),
		Projects: []domain.WorkspaceProject{{ID: "ca-admin", Path: "ca-admin", Type: "backend", Language: "go"}},
	}
	analysis := &domain.WorkspaceProfile{Projects: []domain.WorkspaceProject{{ID: "ca_admin"}}}

	profile, err := AssembleProfile(base, analysis)

	require.Nil(t, profile)
	require.ErrorContains(t, err, `projects[0].id: unknown project "ca_admin"`)
	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
}

func TestAssembleProfileAcceptsDeclaredPathDependency(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"producer", ".docs"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, path), 0755))
	}
	base := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "producer", Path: "producer", Type: "service", Language: "go"}},
	}
	analysis := &domain.WorkspaceProfile{
		Projects: []domain.WorkspaceProject{{ID: "producer"}},
		Shared:   []domain.WorkspacePath{{Path: ".docs", Consumers: []string{"producer"}}},
		Dependencies: []domain.WorkspaceDependency{{
			FromProjectID: "producer",
			To:            domain.WorkspaceReference{Kind: domain.WorkspaceReferencePath, Value: ".docs"},
			Reason:        "读取共享文档",
		}},
	}

	profile, err := AssembleProfile(base, analysis)

	require.NoError(t, err)
	require.Equal(t, ".docs", profile.Dependencies[0].To.Value)
}

func TestAssembleSpecValidatesTypedReferencesAndDotRoutes(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"backend", ".github"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, path), 0755))
	}
	profile := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend", Type: "service", Language: "go"}},
	}
	analysis := &domain.WorkspaceSpec{
		Routing: []domain.WorkspaceRoute{{PathPattern: ".github/**", ProjectIDs: []string{"backend"}, Reason: "CI"}},
		Rules: []domain.WorkspaceRule{{
			Title:       "服务发布",
			Description: "修改服务后执行发布校验",
			AppliesTo:   []domain.WorkspaceReference{{Kind: domain.WorkspaceReferenceRole, Value: "service"}},
			Source:      workspaceRuleSourceProfile,
		}},
		ParallelAgentGuidance: []domain.WorkspaceParallelGuidance{{
			Scope:     domain.WorkspaceReference{Kind: domain.WorkspaceReferencePath, Value: ".github/**"},
			Allowed:   false,
			Condition: "CI 配置串行修改",
		}},
	}

	spec, err := AssembleSpec(&domain.WorkspaceSpec{}, analysis, profile, ValidationOptions{RootPath: root})

	require.NoError(t, err)
	require.Equal(t, domain.WorkspaceRuleAuthorityInferred, spec.Rules[0].Authority)
	require.Equal(t, ".github/**", spec.Routing[0].PathPattern)
}

func TestAssembleSpecPreservesBaseRoutesAndSystemRules(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"backend", ".github"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, path), 0755))
	}
	profile := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend", Type: "service", Language: "go"}},
	}
	base := SpecFromProfile(profile, "en-US")
	analysis := &domain.WorkspaceSpec{
		Routing: []domain.WorkspaceRoute{{PathPattern: ".github/**", ProjectIDs: []string{"backend"}, Reason: "CI"}},
		Rules: []domain.WorkspaceRule{{
			Title:       "Release",
			Description: "Validate release configuration.",
			Source:      workspaceRuleSourceProfile,
		}},
	}

	spec, err := AssembleSpec(base, analysis, profile, ValidationOptions{RootPath: root})

	require.NoError(t, err)
	require.Len(t, spec.Routing, 2)
	require.Equal(t, "backend/**", spec.Routing[0].PathPattern)
	require.Equal(t, ".github/**", spec.Routing[1].PathPattern)
	require.Len(t, spec.Rules, 2)
	require.Equal(t, domain.WorkspaceRuleAuthoritySystem, spec.Rules[0].Authority)
	require.Equal(t, domain.WorkspaceRuleAuthorityInferred, spec.Rules[1].Authority)
}

func TestAssembleProfileRejectsPathSymlinkOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend"), 0755))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "shared")))
	base := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend", Type: "service", Language: "go"}},
	}
	analysis := &domain.WorkspaceProfile{
		Projects: []domain.WorkspaceProject{{ID: "backend"}},
		Shared:   []domain.WorkspacePath{{Path: "shared", Consumers: []string{"backend"}}},
	}

	profile, err := AssembleProfile(base, analysis)

	require.Nil(t, profile)
	require.ErrorContains(t, err, "outside the workspace root")
}

func TestAssembleSpecRejectsUnknownReferencesAndNumberedSteps(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend"), 0755))
	profile := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend", Type: "service", Language: "go"}},
	}
	analysis := &domain.WorkspaceSpec{
		Routing:     []domain.WorkspaceRoute{{PathPattern: "backend/**", ProjectIDs: []string{"ca_admin"}}},
		ChangeOrder: []string{"1. 更新契约"},
	}

	spec, err := AssembleSpec(&domain.WorkspaceSpec{}, analysis, profile, ValidationOptions{RootPath: root})

	require.Nil(t, spec)
	require.ErrorContains(t, err, `routing[0].project_ids[0]: unknown project "ca_admin"`)
	require.ErrorContains(t, err, "change_order[0]: step text must not contain a list number prefix")
}

func TestAssembleSpecRequiresEvidenceForRepositoryRule(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("run tests"), 0644))
	profile := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend", Type: "service", Language: "go"}},
	}
	analysis := &domain.WorkspaceSpec{Rules: []domain.WorkspaceRule{{
		Title:       "验证",
		Description: "修改后运行测试",
		Source:      "AGENTS.md",
	}}}

	spec, err := AssembleSpec(&domain.WorkspaceSpec{}, analysis, profile, ValidationOptions{RootPath: root})

	require.Nil(t, spec)
	require.ErrorContains(t, err, "rules[0].evidence: repository rule requires evidence paths")
}

func TestReconcileProfileRejectsConfigurationDrift(t *testing.T) {
	root := t.TempDir()
	base := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend-v2", Type: "service", Language: "go"}},
	}
	stored := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend", Type: "service", Language: "go"}},
	}

	profile, err := ReconcileProfile(base, stored)

	require.Nil(t, profile)
	require.ErrorContains(t, err, `stored identity for "backend" does not match configuration`)
}
