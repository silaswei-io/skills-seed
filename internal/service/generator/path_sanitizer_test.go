package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
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

	sanitized, _ := sanitizeGenerationInputs(profile, nil, root)

	require.Len(t, sanitized.KeyModules, 1)
	assert.Equal(t, "plugins/key_manage", sanitized.KeyModules[0].Path)
}

func TestSanitizeGenerationInputsPreservesExternalUtilityLocations(t *testing.T) {
	root := t.TempDir()
	profile := &domain.ProjectProfile{
		CommonUtils: []domain.UtilityFunction{{
			Name: "SM2Decrypt",
			File: "gitlab.myibc.net/Olym_Management/go_group/olym-contrib.git/ocryptor",
		}},
	}

	sanitized, _ := sanitizeGenerationInputs(profile, nil, root)

	require.Len(t, sanitized.CommonUtils, 1)
	assert.Equal(t, "gitlab.myibc.net/Olym_Management/go_group/olym-contrib.git/ocryptor", sanitized.CommonUtils[0].File)
}

func TestSanitizeGenerationInputsDropsMissingProjectUtilityLocations(t *testing.T) {
	root := t.TempDir()
	profile := &domain.ProjectProfile{
		CommonUtils: []domain.UtilityFunction{{
			Name: "MissingLocalUtil",
			File: "internal/helper/missing.go",
		}},
	}

	sanitized, _ := sanitizeGenerationInputs(profile, nil, root)

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

	sanitized, _ := sanitizeGenerationInputs(profile, nil, root)

	assert.Empty(t, sanitized.KeyModules)
}

func TestSanitizeGenerationInputsDropsGoodExampleNotFoundInEvidenceFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "service"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "service", "login.go"), []byte("func Login() error {\n\treturn nil\n}\n"), 0644))
	pattern := domain.NewPattern("login-flow", "Login Flow", domain.CategoryBusiness)
	pattern.GoodExample = "func RefactoredSummary() error {\n\treturn nil\n}"
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/login.go", Line: 1}}

	_, patterns := sanitizeGenerationInputs(&domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Len(t, patterns, 1)
	assert.Empty(t, patterns[0].GoodExample)
}

func TestSanitizeGenerationInputsKeepsGoodExampleFoundInEvidenceFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "service"), 0755))
	source := "func Login() error {\n\treturn nil\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "service", "login.go"), []byte(source), 0644))
	pattern := domain.NewPattern("login-flow", "Login Flow", domain.CategoryBusiness)
	pattern.GoodExample = strings.TrimSpace(source)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/login.go", Line: 1}}

	_, patterns := sanitizeGenerationInputs(&domain.ProjectProfile{}, []domain.Pattern{*pattern}, root)

	require.Len(t, patterns, 1)
	assert.Equal(t, strings.TrimSpace(source), patterns[0].GoodExample)
}
