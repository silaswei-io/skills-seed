package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestValidateCurrentPatternsUsesVerifiedSourceEvidence(t *testing.T) {
	root := t.TempDir()
	source := "package service\n\nfunc LoadUser() error {\n\treturn nil\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte(source), 0o644))

	pattern := domain.NewPattern("load-user", "Load User", domain.CategoryBusiness)
	pattern.Rule = "When loading a user, preserve the repository error boundary."
	pattern.GoodExample = "func LoadUser() error {\n\treturn nil\n}"
	pattern.Confidence = 0.99
	pattern.Frequency = 99
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "service.go", Line: 99, Symbol: "LoadUser", Kind: "function"},
	}

	patterns := validateCurrentPatterns(root, []domain.Pattern{*pattern})

	require.Len(t, patterns, 1)
	require.Equal(t, domain.SourceLearnedCurrent, patterns[0].Source)
	require.Equal(t, "service.go", patterns[0].ScopePath)
	require.Equal(t, 3, patterns[0].EvidenceLocations[0].Line)
	require.Equal(t, "func", patterns[0].EvidenceLocations[0].Kind)
	require.Equal(t, 1, patterns[0].Frequency)
	require.Equal(t, 0.80, patterns[0].Confidence)
	require.Equal(t, pattern.GoodExample, patterns[0].GoodExample)
}

func TestValidateCurrentPatternsAcceptsTypeFamilyEvidence(t *testing.T) {
	root := t.TempDir()
	source := "package service\n\ntype User struct {\n\tID string\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte(source), 0o644))

	pattern := domain.NewPattern("user-identity", "User Identity", domain.CategoryStructure)
	pattern.Rule = "When extending users, preserve the string identity field."
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "service.go", Symbol: "User", Kind: "type"},
	}

	patterns := validateCurrentPatterns(root, []domain.Pattern{*pattern})

	require.Len(t, patterns, 1)
	require.Equal(t, "struct", patterns[0].EvidenceLocations[0].Kind)
	require.Equal(t, 3, patterns[0].EvidenceLocations[0].Line)
}

func TestCompatibleSymbolKindUsesCanonicalFamilies(t *testing.T) {
	tests := []struct {
		requested string
		actual    string
		want      bool
	}{
		{requested: "function", actual: "func", want: true},
		{requested: "method", actual: "func", want: true},
		{requested: "type", actual: "struct", want: true},
		{requested: "type", actual: "class", want: true},
		{requested: "type", actual: "interface", want: true},
		{requested: "type", actual: "func", want: false},
		{requested: "file", actual: "struct", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.requested+"-"+tt.actual, func(t *testing.T) {
			require.Equal(t, tt.want, compatibleSymbolKind(tt.requested, tt.actual))
		})
	}
}

func TestValidateCurrentPatternsRejectsUnverifiedCandidates(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte("package service\n"), 0o644))

	missingRule := domain.NewPattern("missing-rule", "Missing Rule", domain.CategoryBusiness)
	missingRule.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Kind: "file"}}

	missingFile := domain.NewPattern("missing-file", "Missing File", domain.CategoryBusiness)
	missingFile.Rule = "Preserve the missing behavior."
	missingFile.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "missing.go", Kind: "file"}}

	missingSymbol := domain.NewPattern("missing-symbol", "Missing Symbol", domain.CategoryBusiness)
	missingSymbol.Rule = "Preserve the missing symbol behavior."
	missingSymbol.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Symbol: "Unknown", Kind: "function"}}

	patterns := validateCurrentPatterns(root, []domain.Pattern{*missingRule, *missingFile, *missingSymbol})

	require.Empty(t, patterns)
}

func TestValidateCurrentPatternsDropsNonContiguousGoodExample(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte("package service\nfunc LoadUser() {}\n"), 0o644))

	pattern := domain.NewPattern("load-user", "Load User", domain.CategoryBusiness)
	pattern.Rule = "When loading a user, use LoadUser."
	pattern.GoodExample = "func LoadUser() {\n\t// ...\n}"
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Symbol: "LoadUser", Kind: "function"}}

	patterns := validateCurrentPatterns(root, []domain.Pattern{*pattern})

	require.Len(t, patterns, 1)
	require.Empty(t, patterns[0].GoodExample)
}

func TestValidateCurrentPatternsHasNoQuantityLimit(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte("package service\n"), 0o644))

	candidates := make([]domain.Pattern, 20)
	for i := range candidates {
		pattern := domain.NewPattern(fmt.Sprintf("pattern-%02d", i), "Project Rule", domain.CategoryBusiness)
		pattern.Rule = "When changing this behavior, preserve the evidenced project constraint."
		pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Kind: "file"}}
		candidates[i] = *pattern
	}

	require.Len(t, validateCurrentPatterns(root, candidates), len(candidates))
}

func TestValidateCurrentPatternsRejectsSymlinkEvidenceOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	require.NoError(t, os.WriteFile(outside, []byte("package outside\nfunc Secret() {}\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "linked.go")))
	pattern := domain.NewPattern("secret", "Secret", domain.CategoryBusiness)
	pattern.Rule = "Use Secret."
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "linked.go", Symbol: "Secret", Kind: "function"}}

	require.Empty(t, validateCurrentPatterns(root, []domain.Pattern{*pattern}))
}
