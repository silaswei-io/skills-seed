package runtimefiles

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNameUsesShortTimeAndReadableParts(t *testing.T) {
	name := Name("pattern-learn-current", "unit-auth")

	require.Regexp(t, regexp.MustCompile(`^\d{8}-\d{6}(?:-\d{3,})?-pattern-learn-current-unit-auth$`), name)
}

func TestNewIDUsesSecondPrecisionAndSequence(t *testing.T) {
	ids := make(map[string]struct{}, 32)
	for i := 0; i < 32; i++ {
		id := NewID()
		require.Regexp(t, regexp.MustCompile(`^\d{8}-\d{6}(?:-\d{3,})?$`), id)
		ids[id] = struct{}{}
	}

	require.Len(t, ids, 32)
}

func TestNameWithProvidedIDDoesNotConsumeGeneratedID(t *testing.T) {
	idState.Lock()
	idState.second = time.Now().Format(timestampLayout)
	idState.seq = 41
	idState.Unlock()

	_ = NameWithID("20260710-120000", "kind")

	idState.Lock()
	seq := idState.seq
	idState.Unlock()
	require.Equal(t, 41, seq)
}

func TestSafePartKeepsASCIIMeaningAndBoundsLength(t *testing.T) {
	part := SafePart("Auth Login/Permission Check", "")

	require.Equal(t, "auth-login-permission-check", part)
	require.LessOrEqual(t, len([]rune(SafePart(strings.Repeat("auth", 40), ""))), MaxSafePartLength)
}

func TestTempPatternIsValidMkdirTempPrefix(t *testing.T) {
	pattern := TempPattern("prompt-input", "skills-seed", "unit-auth")

	require.NotContains(t, filepath.Base(pattern), string(filepath.Separator))
	require.Regexp(t, regexp.MustCompile(`^\d{8}-\d{6}(?:-\d{3,})?-prompt-input-skills-seed-unit-auth-$`), pattern)
}
