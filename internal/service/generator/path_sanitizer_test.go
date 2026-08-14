package generator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/service/repositoryscopeconfig"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeGenerationInputsKeepsExistingDirectoryModulePaths(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plugins", "key_manage"), 0755))
	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{{
			Name: "key_manage",
			Path: "plugins/key_manage",
		}},
	}

	sanitized, _ := sanitizeGenerationInputsForTest(t, profile, nil, root)

	require.Len(t, sanitized.KeyModules, 1)
	assert.Equal(t, "plugins/key_manage", sanitized.KeyModules[0].Path)
}

func TestSanitizeGenerationInputsDropsUnverifiableExternalUtilityLocations(t *testing.T) {
	root := t.TempDir()
	profile := &domain.ProjectProfile{
		CommonUtils: []domain.UtilityFunction{{
			Name: "SM2Decrypt",
			File: "gitlab.myibc.net/Olym_Management/go_group/olym-contrib.git/ocryptor",
		}},
	}

	sanitized, _ := sanitizeGenerationInputsForTest(t, profile, nil, root)

	assert.Empty(t, sanitized.CommonUtils)
}

func TestSanitizeGenerationInputsDropsLegacyProfileUtility(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "helper"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "internal", "helper", "response.go"),
		[]byte("package helper\n\nfunc BuildResponse(value any) error { return nil }\n"),
		0o644,
	))
	profile := &domain.ProjectProfile{CommonUtils: []domain.UtilityFunction{{
		Name:      "BuildResponse",
		File:      "internal/helper/response.go:3",
		Signature: "func BuildResponse(value any) error",
	}}}

	sanitized, _ := sanitizeGenerationInputsForTest(t, profile, nil, root)

	require.Empty(t, sanitized.CommonUtils)
}

func TestSanitizeGenerationInputsDropsMissingProjectUtilityLocations(t *testing.T) {
	root := t.TempDir()
	profile := &domain.ProjectProfile{
		CommonUtils: []domain.UtilityFunction{{
			Name: "MissingLocalUtil",
			File: "internal/helper/missing.go",
		}},
	}

	sanitized, _ := sanitizeGenerationInputsForTest(t, profile, nil, root)

	assert.Empty(t, sanitized.CommonUtils)
}

func TestSanitizeGenerationInputsDropsMissingModulePaths(t *testing.T) {
	root := t.TempDir()
	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{{
			Name: "missing",
			Path: "internal/missing",
		}},
	}

	sanitized, _ := sanitizeGenerationInputsForTest(t, profile, nil, root)

	assert.Empty(t, sanitized.KeyModules)
}

func TestSanitizeGenerationInputsDropsModulePathOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(parent, "outside"), 0o755))
	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{{
			Name: "outside",
			Path: "../outside",
		}},
	}

	sanitized, _ := sanitizeGenerationInputsForTest(t, profile, nil, root)

	assert.Empty(t, sanitized.KeyModules)
}

func TestSanitizeGenerationInputsDropsGoodExampleNotFoundInEvidenceFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "service"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "service", "login.go"), []byte("package service\n\nfunc Login() error {\n\treturn nil\n}\n"), 0644))
	pattern := domain.NewPattern("login-flow", "Login Flow", domain.CategoryBusiness)
	pattern.GoodExample = "func RefactoredSummary() error {\n\treturn nil\n}"
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/login.go", Line: 3, Symbol: "Login", Kind: "function"}}

	_, patterns := sanitizeGenerationInputsForTest(t, &domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Len(t, patterns, 1)
	assert.Empty(t, patterns[0].GoodExample)
}

func TestSanitizeGenerationInputsDoesNotReadGoodExampleFromOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	require.NoError(t, os.MkdirAll(root, 0o755))
	snippet := "func Outside() error { return nil }"
	require.NoError(t, os.WriteFile(filepath.Join(parent, "outside.go"), []byte("package outside\n"+snippet+"\n"), 0o644))
	pattern := domain.NewPattern("outside", "Outside", domain.CategoryBusiness)
	pattern.GoodExample = snippet
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "../outside.go", Kind: "file"}}

	_, patterns := sanitizeGenerationInputsForTest(t, &domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Empty(t, patterns)
}

func TestSanitizeGenerationInputsKeepsGoodExampleFoundInEvidenceFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "service"), 0755))
	snippet := "func Login() error {\n\treturn nil\n}"
	source := "package service\n\n" + snippet + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "service", "login.go"), []byte(source), 0644))
	pattern := domain.NewPattern("login-flow", "Login Flow", domain.CategoryBusiness)
	pattern.GoodExample = snippet
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/login.go", Line: 3, Symbol: "Login", Kind: "function"}}

	_, patterns := sanitizeGenerationInputsForTest(t, &domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Len(t, patterns, 1)
	assert.Equal(t, strings.TrimSpace(snippet), patterns[0].GoodExample)
}

func TestSanitizeGenerationInputsDropsUnverifiedSymbolEvidence(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "policy.txt"), []byte("project-specific policy\n"), 0o644))
	pattern := domain.NewPattern("policy", "Policy", domain.CategoryConfig)
	pattern.Description = "配置由当前策略文件控制"
	pattern.Confidence = 0.73
	pattern.Frequency = 4
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "policy.txt", Symbol: "UnsupportedSymbol", Kind: "function"}}

	_, patterns := sanitizeGenerationInputsForTest(t, &domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Empty(t, patterns)
}

func TestSanitizeGenerationInputsKeepsExplicitFileEvidence(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "policy.txt"), []byte("project-specific policy\n"), 0o644))
	pattern := domain.NewPattern("policy", "Policy", domain.CategoryConfig)
	pattern.Description = "The file defines the observed local behavior."
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "policy.txt", Kind: "file"}}

	_, patterns := sanitizeGenerationInputsForTest(t, &domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Len(t, patterns, 1)
	require.Equal(t, []domain.PatternEvidenceLocation{{Path: "policy.txt", Kind: "file"}}, patterns[0].EvidenceLocations)
}

func TestSanitizeGenerationInputsDropsScopeWithoutVerifiedEvidence(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "policy"), 0o755))
	pattern := domain.NewPattern("policy", "Policy", domain.CategoryBusiness)
	pattern.ScopePath = "internal/policy"

	_, patterns := sanitizeGenerationInputsForTest(t, &domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Empty(t, patterns)
}

func TestSanitizeGenerationInputsDropsCacheEvidenceAndProfileEntries(t *testing.T) {
	root := t.TempDir()
	cacheFile := filepath.Join(root, ".test", "quality", "cache", "go-mod", "dependency", "service.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(cacheFile), 0o755))
	require.NoError(t, os.WriteFile(cacheFile, []byte("package dependency\n\nfunc Execute() error { return nil }\n"), 0o644))

	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{{
			Name: "cached",
			Path: ".test/quality/cache/go-mod/dependency",
		}},
		CommonUtils: []domain.UtilityFunction{{
			Name:      "Execute",
			File:      ".test/quality/cache/go-mod/dependency/service.go:3",
			Signature: "func Execute() error",
		}},
	}
	pattern := domain.NewPattern("cached-operation", "Cached Operation", domain.CategoryBusiness)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   ".test/quality/cache/go-mod/dependency/service.go",
		Line:   3,
		Symbol: "Execute",
		Kind:   "function",
	}}

	sanitized, patterns := sanitizeGenerationInputsForTest(t, profile, []domain.Pattern{*pattern}, root)

	require.Empty(t, sanitized.KeyModules)
	require.Empty(t, sanitized.CommonUtils)
	require.Empty(t, patterns)
}

func TestSanitizeGenerationInputsDropsProcedureBackedKnowledge(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeSource(t, root, "internal/service/account.go", "package service\n\nfunc CreateAccount() error { return nil }\n")
	writeKnowledgeSource(t, root, "tests/account_test.go", "package tests\n\nfunc CreateFixture() error { return nil }\n")
	writeKnowledgeSource(t, root, "deploy/release.go", "package deploy\n\nfunc PublishRelease() error { return nil }\n")

	profile := &domain.ProjectProfile{
		BusinessMethods: []domain.BusinessMethod{
			{Name: "CreateAccount", Function: "func CreateAccount() error", CodeLocation: domain.CodeLocation{CurrentLocation: "internal/service/account.go:3"}},
			{Name: "CreateFixture", Function: "func CreateFixture() error", CodeLocation: domain.CodeLocation{CurrentLocation: "tests/account_test.go:3"}},
		},
		CommonUtils: []domain.UtilityFunction{
			{Name: "CreateAccount", File: "internal/service/account.go:3", Signature: "func CreateAccount() error"},
			{Name: "PublishRelease", File: "deploy/release.go:3", Signature: "func PublishRelease() error"},
		},
	}
	business := domain.NewPattern("account-create", "Account Create", domain.CategoryBusiness)
	business.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/account.go", Line: 3, Symbol: "CreateAccount", Kind: "function"}}
	testingPattern := domain.NewPattern("fixture-create", "Fixture Create", domain.CategoryUtils)
	testingPattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "tests/account_test.go", Line: 3, Symbol: "CreateFixture", Kind: "function"}}
	releasePattern := domain.NewPattern("release-publish", "Release Publish", domain.CategoryBusiness)
	releasePattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "deploy/release.go", Line: 3, Symbol: "PublishRelease", Kind: "function"}}

	sanitized, patterns := sanitizeGenerationInputsForTest(t, profile, []domain.Pattern{*business, *testingPattern, *releasePattern}, root)

	require.Empty(t, sanitized.BusinessMethods)
	require.Empty(t, sanitized.CommonUtils)
	require.Len(t, patterns, 1)
	assert.Equal(t, "account-create", patterns[0].ID)
}

func TestSanitizeGenerationInputsBuildsModuleEntriesFromVerifiedSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "service"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "internal", "service", "custom.go"),
		[]byte(`package service

type Custom struct{}

func (c *Custom) Start() error { return nil }

func (c *Custom) Publish() error { return nil }
`),
		0o644,
	))

	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{{
			Name:         "service",
			Path:         "internal/service",
			Dependencies: []string{"profile-dependency"},
			Dependents:   []string{"profile-dependent"},
			KeyMethods:   []string{"InventedMethod"},
		}},
		BusinessMethods: []domain.BusinessMethod{{
			Name:         "Custom.Start",
			Function:     "func (c *Custom) Start() error",
			CodeLocation: domain.CodeLocation{CurrentLocation: "internal/service/custom.go:5"},
		}},
	}
	pattern := domain.NewPattern("publish", "Publish", domain.CategoryBusiness)
	pattern.BusinessMethod = &domain.BusinessMethod{
		Name:         "Custom.Publish",
		Function:     "func (c *Custom) Publish() error",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/service/custom.go:7"},
	}

	sanitized, patterns := sanitizeGenerationInputsForTest(t, profile, []domain.Pattern{*pattern}, root)

	require.Len(t, sanitized.BusinessMethods, 1)
	assert.Equal(t, "Publish", sanitized.BusinessMethods[0].Name)
	require.Len(t, sanitized.KeyModules, 1)
	assert.Empty(t, sanitized.KeyModules[0].Dependencies)
	assert.Empty(t, sanitized.KeyModules[0].Dependents)
	assert.Equal(t, []string{"Publish"}, sanitized.KeyModules[0].KeyMethods)
	require.Len(t, patterns, 1)
	require.NotNil(t, patterns[0].BusinessMethod)
	assert.Equal(t, "Publish", patterns[0].BusinessMethod.Name)
}

func TestSanitizeGenerationInputsKeepsOnlyKnownNonSelfModuleRelations(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "service"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "repository"), 0o755))
	profile := &domain.ProjectProfile{KeyModules: []domain.ModuleInfo{
		{Name: "service layer", Path: "internal/service", Dependencies: []string{"service layer", "internal/repository", "internal/missing"}},
		{Name: "repository", Path: "internal/repository", Dependents: []string{"internal/service", "repository"}},
	}}

	sanitized, _ := sanitizeGenerationInputsForTest(t, profile, nil, root)

	require.Len(t, sanitized.KeyModules, 2)
	require.Equal(t, []string{"internal/repository"}, sanitized.KeyModules[0].Dependencies)
	require.Empty(t, sanitized.KeyModules[0].Dependents)
	require.Empty(t, sanitized.KeyModules[1].Dependencies)
	require.Equal(t, []string{"internal/service"}, sanitized.KeyModules[1].Dependents)
}

func sanitizeGenerationInputsForTest(t *testing.T, profile *domain.ProjectProfile, patterns []domain.Pattern, root string) (*domain.ProjectProfile, []domain.Pattern) {
	t.Helper()
	resolver := sourcecode.NewResolver(config.StructuralConfig{Provider: config.StructuralProviderTreeSitter})
	sanitized, validated, err := knowledge.VerifyProjectKnowledge(context.Background(), profile, patterns, root, resolver, repositoryscopeconfig.DefaultKnowledgeScope())
	require.NoError(t, err)
	return sanitized, validated
}

func writeKnowledgeSource(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
}
