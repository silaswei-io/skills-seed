package patternnorm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGateEvidenceForPersistence(t *testing.T) {
	root := t.TempDir()
	rel := filepath.Join("internal", "svc", "user.go")
	abs := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte("package svc\n"), 0o644))

	good := domain.Pattern{
		ID: "good",
		EvidenceLocations: []domain.PatternEvidenceLocation{
			{Path: filepath.ToSlash(rel), Line: 1},
		},
	}
	missing := domain.Pattern{
		ID: "missing",
		EvidenceLocations: []domain.PatternEvidenceLocation{
			{Path: "internal/missing.go", Line: 1},
		},
	}
	escape := domain.Pattern{
		ID: "escape",
		EvidenceLocations: []domain.PatternEvidenceLocation{
			{Path: "../outside.go", Line: 1},
		},
	}

	kept, dropped := gateEvidenceForPersistence(root, []domain.Pattern{good, missing, escape})
	require.Len(t, kept, 1)
	require.Equal(t, "good", kept[0].ID)
	require.Len(t, dropped, 2)
	require.Equal(t, DropUnsupportedEvidence, dropped[0].ReasonCode)
}

func TestGateEvidencePathWithoutRootStillRejectsEscape(t *testing.T) {
	_, ok := gateEvidencePath("", "internal/ok.go")
	require.True(t, ok)
	_, ok = gateEvidencePath("", "../escape.go")
	require.False(t, ok)
}
