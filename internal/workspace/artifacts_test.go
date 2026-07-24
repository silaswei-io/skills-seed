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
	analysis := &domain.WorkspaceProfile{Projects: []domain.WorkspaceProject{{ID: "missing-service"}}}

	profile, err := AssembleProfile(base, analysis)

	require.Nil(t, profile)
	require.ErrorContains(t, err, `projects[0].id: unknown project "missing-service"`)
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

func TestAssembleProfileResolvesProjectPathDependency(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"crtapi4go", "ra_admin"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, path), 0755))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".github"), 0755))
	base := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{
			{ID: "crtapi4go", Path: "crtapi4go", Type: "library", Language: "go"},
			{ID: "ra-admin", Path: "ra_admin", Type: "service", Language: "go"},
		},
	}
	analysis := &domain.WorkspaceProfile{
		Projects: []domain.WorkspaceProject{{ID: "crtapi4go"}, {ID: "ra_admin"}},
		Shared: []domain.WorkspacePath{{
			Path:             "crtapi4go",
			AffectedProjects: []string{"ra_admin"},
		}, {
			Path:             "root-config.yaml",
			AffectedProjects: []string{"ra_admin"},
		}},
		Infra: []domain.WorkspacePath{{
			Path:             "**/Taskfile.yml",
			Description:      "任务入口会影响所有 Go 后端",
			AffectedProjects: []string{"ra_admin", "crtapi4go"},
		}, {
			Path:             ".github/**",
			Description:      "根级 CI 配置影响后端校验",
			AffectedProjects: []string{"ra_admin"},
		}, {
			Path:             "https://ci.example.test/job/service",
			Description:      "外部系统地址不是仓库路径",
			AffectedProjects: []string{"ra_admin"},
		}},
		Dependencies: []domain.WorkspaceDependency{
			{
				FromProjectID: "ra-admin",
				To:            domain.WorkspaceReference{Kind: domain.WorkspaceReferencePath, Value: "crtapi4go"},
				Reason:        "RA 使用密码学库",
			},
			{
				FromProjectID: "crtapi4go",
				To:            domain.WorkspaceReference{Kind: domain.WorkspaceReferencePath, Value: "ra_admin"},
				Reason:        "测试路径别名归一化",
			},
			{
				FromProjectID: "ra_admin",
				To:            domain.WorkspaceReference{Kind: domain.WorkspaceReferencePath, Value: "external-platform"},
				Reason:        "非仓库路径依赖会被丢弃",
			},
		},
	}

	profile, err := AssembleProfile(base, analysis)

	require.NoError(t, err)
	require.Equal(t, domain.WorkspaceReferenceProject, profile.Dependencies[0].To.Kind)
	require.Equal(t, "crtapi4go", profile.Dependencies[0].To.Value)
	require.Equal(t, domain.WorkspaceReferenceProject, profile.Dependencies[1].To.Kind)
	require.Equal(t, "ra-admin", profile.Dependencies[1].To.Value)
	require.Len(t, profile.Dependencies, 2)
	require.Equal(t, "ra-admin", profile.Projects[1].ID)
	require.Empty(t, profile.Shared)
	require.Empty(t, profile.Infra)
	require.Len(t, profile.ImpactRoutes, 1)
	require.Equal(t, ".github/**", profile.ImpactRoutes[0].PathPattern)
	require.Equal(t, []string{"ra-admin"}, profile.ImpactRoutes[0].ProjectIDs)
}

func TestAssembleProfileNormalizesWorkspacePathsAtSource(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"app/contracts/api", "app/internal/local", "worker", "shared"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, path), 0755))
	}
	base := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{
			{ID: "app", Path: "app", Type: "service", Language: "go"},
			{ID: "worker", Path: "worker", Type: "service", Language: "go"},
		},
	}
	analysis := &domain.WorkspaceProfile{
		Projects: []domain.WorkspaceProject{{ID: "app"}, {ID: "worker"}},
		Shared: []domain.WorkspacePath{
			{Path: "shared", Consumers: []string{"app"}},
			{Path: "shared", Consumers: []string{"worker"}, Producers: []string{"app"}},
		},
		Contracts: []domain.WorkspacePath{
			{Path: "app/contracts/api", Description: "应用对外契约", Consumers: []string{"worker"}},
			{Path: "app/internal/local", Description: "应用内部契约", Consumers: []string{"app"}},
		},
		ImpactRoutes: []domain.WorkspaceRoute{{
			PathPattern: "app/contracts/api/**",
			ProjectIDs:  []string{"app", "worker"},
			Reason:      "契约变更影响调用方",
		}},
	}

	profile, err := AssembleProfile(base, analysis)

	require.NoError(t, err)
	require.Len(t, profile.Shared, 1)
	require.Equal(t, []string{"app"}, profile.Shared[0].Producers)
	require.Equal(t, []string{"app", "worker"}, profile.Shared[0].Consumers)
	require.Len(t, profile.Contracts, 1)
	require.Equal(t, "app/contracts/api", profile.Contracts[0].Path)
	require.Equal(t, []string{"app"}, profile.Contracts[0].Producers)
	require.Equal(t, []string{"worker"}, profile.Contracts[0].Consumers)
	require.Empty(t, profile.ImpactRoutes)
}

func TestSpecFromProfileMergesDefaultAndImpactRoutes(t *testing.T) {
	profile := &domain.WorkspaceProfile{
		Name: "demo",
		Projects: []domain.WorkspaceProject{
			{ID: "app", Path: "app"},
			{ID: "worker", Path: "worker"},
		},
		ImpactRoutes: []domain.WorkspaceRoute{{
			PathPattern: "app/**",
			ProjectIDs:  []string{"worker"},
			Reason:      "应用变更影响 worker",
		}},
	}

	spec := SpecFromProfile(profile, "zh-CN")

	require.Len(t, spec.Routing, 2)
	require.Equal(t, "app/**", spec.Routing[0].PathPattern)
	require.Equal(t, []string{"app", "worker"}, spec.Routing[0].ProjectIDs)
	require.Equal(t, "worker/**", spec.Routing[1].PathPattern)
	require.Equal(t, []string{"worker"}, spec.Routing[1].ProjectIDs)
}

func TestAssembleSpecValidatesTypedReferencesAndDotRoutes(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"backend_service", ".github"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, path), 0755))
	}
	profile := &domain.WorkspaceProfile{
		Name:     "demo",
		RootPath: root,
		Projects: []domain.WorkspaceProject{{ID: "backend", Path: "backend_service", Type: "service", Language: "go"}},
	}
	analysis := &domain.WorkspaceSpec{
		Routing: []domain.WorkspaceRoute{
			{PathPattern: ".github/**", ProjectIDs: []string{"backend_service"}, Reason: "CI"},
			{PathPattern: "external-platform/**", ProjectIDs: []string{"backend_service"}, Reason: "外部系统不是仓库路径"},
			{PathPattern: "https://ci.example.test/job/demo", ProjectIDs: []string{"backend_service"}, Reason: "外部 URL 不是仓库路径"},
		},
		Rules: []domain.WorkspaceRule{{
			Title:       "服务发布",
			Description: "修改服务后执行发布校验",
			AppliesTo: []domain.WorkspaceReference{
				{Kind: domain.WorkspaceReferenceProject, Value: "backend_service"},
				{Kind: domain.WorkspaceReferencePath, Value: "external-platform"},
			},
			Source: workspaceRuleSourceProfile,
		}},
		ParallelAgentGuidance: []domain.WorkspaceParallelGuidance{
			{
				Scope:     domain.WorkspaceReference{Kind: domain.WorkspaceReferencePath, Value: ".github/**"},
				Allowed:   false,
				Condition: "CI 配置串行修改",
			},
			{
				Scope:     domain.WorkspaceReference{Kind: domain.WorkspaceReferencePath, Value: "external-platform"},
				Allowed:   false,
				Condition: "外部系统不是仓库路径",
			},
		},
	}

	spec, err := AssembleSpec(&domain.WorkspaceSpec{}, analysis, profile, ValidationOptions{RootPath: root})

	require.NoError(t, err)
	require.Equal(t, domain.WorkspaceRuleAuthorityInferred, spec.Rules[0].Authority)
	require.Equal(t, ".github/**", spec.Routing[0].PathPattern)
	require.Equal(t, []string{"backend"}, spec.Routing[0].ProjectIDs)
	require.Equal(t, domain.WorkspaceReferenceProject, spec.Rules[0].AppliesTo[0].Kind)
	require.Equal(t, "backend", spec.Rules[0].AppliesTo[0].Value)
	require.Len(t, spec.Routing, 1)
	require.Len(t, spec.Rules[0].AppliesTo, 1)
	require.Len(t, spec.ParallelAgentGuidance, 1)
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
