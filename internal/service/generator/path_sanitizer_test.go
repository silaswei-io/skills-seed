package generator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
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

func TestSanitizeGenerationInputsKeepsSourceVerifiedUtility(t *testing.T) {
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

	require.Len(t, sanitized.CommonUtils, 1)
	assert.Equal(t, "internal/helper/response.go:3", sanitized.CommonUtils[0].File)
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

func TestSanitizeGenerationInputsKeepsExistingScopeWithoutEvidence(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "policy"), 0o755))
	pattern := domain.NewPattern("policy", "Policy", domain.CategoryBusiness)
	pattern.ScopePath = "internal/policy"

	_, patterns := sanitizeGenerationInputsForTest(t, &domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Len(t, patterns, 1)
	require.Equal(t, "internal/policy", patterns[0].ScopePath)
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

	require.Len(t, sanitized.BusinessMethods, 2)
	assert.ElementsMatch(t, []string{"Start", "Publish"}, []string{
		sanitized.BusinessMethods[0].Name,
		sanitized.BusinessMethods[1].Name,
	})
	require.Len(t, sanitized.KeyModules, 1)
	assert.Equal(t, []string{"profile-dependency"}, sanitized.KeyModules[0].Dependencies)
	assert.Equal(t, []string{"profile-dependent"}, sanitized.KeyModules[0].Dependents)
	assert.ElementsMatch(t, []string{"Start", "Publish"}, sanitized.KeyModules[0].KeyMethods)
	require.Len(t, patterns, 1)
	require.NotNil(t, patterns[0].BusinessMethod)
	assert.Equal(t, "Publish", patterns[0].BusinessMethod.Name)
}

func sanitizeGenerationInputsForTest(t *testing.T, profile *domain.ProjectProfile, patterns []domain.Pattern, root string) (*domain.ProjectProfile, []domain.Pattern) {
	t.Helper()
	resolver := sourcecode.NewResolver(config.StructuralConfig{Provider: config.StructuralProviderTreeSitter})
	sanitized, validated, err := sanitizeGenerationInputs(context.Background(), profile, patterns, root, resolver)
	require.NoError(t, err)
	return sanitized, validated
}
