package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternAllowsHardConstraintOnlyForMaintainedSources(t *testing.T) {
	for _, source := range []Source{SourceUserDefined, SourceDefault} {
		pattern := Pattern{Source: source}
		assert.True(t, pattern.AllowsHardConstraint(), source)
	}
	for _, source := range []Source{SourceLearned, SourceLearnedCurrent, SourceLearnedHistory, SourceInit} {
		pattern := Pattern{Source: source}
		assert.False(t, pattern.AllowsHardConstraint(), source)
	}
}

func TestPatternEvidenceFileCountCountsFilesInsteadOfSymbols(t *testing.T) {
	count := PatternEvidenceFileCount([]PatternEvidenceLocation{
		{Path: "internal/health.go", Line: 10, Symbol: "Check"},
		{Path: "internal/health.go", Line: 20, Symbol: "collect"},
		{Path: "internal/health.go", Line: 30, Symbol: "worker"},
	})

	require.Equal(t, 1, count)
}

func TestPatternEvidenceFileCountUsesDistinctFileCoverage(t *testing.T) {
	count := PatternEvidenceFileCount([]PatternEvidenceLocation{
		{Path: "internal/a.go", Symbol: "A"},
		{Path: "internal/b.go", Symbol: "B"},
		{Path: "internal/c.go", Symbol: "C"},
	})

	require.Equal(t, 3, count)
}
