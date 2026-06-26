package runtimefiles

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNameUsesShortTimeAndReadableParts(t *testing.T) {
	name := Name("pattern-learn-current", "unit-auth")

	require.Regexp(t, regexp.MustCompile(`^\d{8}-\d{6}-pattern-learn-current-unit-auth$`), name)
}

func TestSafePartKeepsASCIIMeaningAndBoundsLength(t *testing.T) {
	part := SafePart("Auth Login/Permission Check", "")

	require.Equal(t, "auth-login-permission-check", part)
	require.LessOrEqual(t, len([]rune(SafePart(strings.Repeat("auth", 40), ""))), MaxSafePartLength)
}

func TestTempPatternIsValidMkdirTempPrefix(t *testing.T) {
	pattern := TempPattern("prompt-input", "skills-seed", "unit-auth")

	require.NotContains(t, filepath.Base(pattern), string(filepath.Separator))
	require.Regexp(t, regexp.MustCompile(`^\d{8}-\d{6}-prompt-input-skills-seed-unit-auth-$`), pattern)
}
