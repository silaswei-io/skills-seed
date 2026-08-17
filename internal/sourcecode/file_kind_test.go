package sourcecode

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDocumentMatchesCaseInsensitively(t *testing.T) {
	for _, path := range []string{
		"README.MD",
		"docs/Guide.MDX",
		"SECURITY.TXT",
		"Contributing",
		"CHANGELOG.RST",
	} {
		require.True(t, IsDocument(path), path)
	}
}

func TestIsAnalyzableKeepsSourcesUnderDocs(t *testing.T) {
	require.True(t, IsAnalyzable("docs/examples/main.go"))
	require.True(t, IsAnalyzable("docs/src/demo.TSX"))
	require.True(t, IsAnalyzable("examples/README.go"))
	require.False(t, IsAnalyzable("docs/Guide.MD"))
}

func TestIsAnalyzableKeepsDependencyFiles(t *testing.T) {
	require.True(t, IsAnalyzable("go.mod"))
	require.True(t, IsAnalyzable("requirements.txt"))
	require.True(t, IsAnalyzable("docs/package.json"))
	require.False(t, IsAnalyzable("LICENSE"))
}

func TestIsProcedureSource(t *testing.T) {
	for _, path := range []string{
		"internal/service/service_test.go",
		"src/auth/login.spec.ts",
		"tests/auth_test.py",
		"release/publish.sh",
		"deploy",
		"deploy/**",
		"deploy/release.go",
		"acceptance/cases.json",
		".github/workflows/verify.yml",
		"scripts/deploy.sh",
	} {
		require.True(t, IsProcedureSource(path), path)
	}
	for _, path := range []string{
		"internal/service/service.go",
		"internal/release/model.go",
		"internal/deployment_state/store.go",
		"internal/spec/model.go",
		"config/application.yaml",
		"deploy/chart/values.yaml",
		"deployment/service/config.toml",
		"release/image-metadata.json",
		"helm/service/values.yaml",
		"k8s/service/deployment.yaml",
		"Dockerfile",
		"docker-compose.yaml",
	} {
		require.False(t, IsProcedureSource(path), path)
	}
}

func TestIsUserRuleAuthorityRequiresExactSeedShape(t *testing.T) {
	require.True(t, IsUserRuleAuthority(".skills-seed/rules/foundation/RULE.md"))
	require.False(t, IsUserRuleAuthority(".skills-seed/rules/RULE.md"))
	require.False(t, IsUserRuleAuthority("nested/.skills-seed/rules/foundation/RULE.md"))
	require.False(t, IsUserRuleAuthority(".skills-seed/rules/foundation/metadata.yaml"))
}

func TestIsInstructionAuthorityExcludesAutomationSources(t *testing.T) {
	tests := map[string]bool{
		"AGENTS.md":                                  true,
		"modules/payments/CLAUDE.md":                 true,
		".skills-seed/rules/release/RULE.md":         true,
		"Taskfile.yml":                               false,
		".github/workflows/verify.yaml":              false,
		"scripts/build.sh":                           false,
		".skills-seed/workflows/release/WORKFLOW.md": false,
	}

	for path, expected := range tests {
		t.Run(path, func(t *testing.T) {
			require.Equal(t, expected, IsInstructionAuthority(path))
		})
	}
}

func TestIsEngineeringKnowledge(t *testing.T) {
	tests := map[string]bool{
		"AGENTS.md":                           true,
		"docs/AGENTS.md":                      true,
		"Taskfile.yml":                        true,
		"Makefile":                            true,
		".github/workflows/verify.yaml":       true,
		".skills-seed/rules/release/RULE.md":  true,
		".skills-seed/context/release.md":     false,
		".skills-seed/rules/metadata.yaml":    false,
		"internal/service/service.go":         false,
		".github/workflows/notes.md":          false,
		"docs/build-and-test-instructions.md": false,
	}

	for path, expected := range tests {
		t.Run(path, func(t *testing.T) {
			require.Equal(t, expected, IsEngineeringKnowledge(path))
		})
	}
}
