package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileAnalysisScopeKeyForPathNormalizesPath(t *testing.T) {
	scope := FileAnalysisScope{ProjectID: "backend", ScopePath: "services/backend"}

	require.Equal(t, "backend\x00services/backend\x00internal/app.go", scope.KeyForPath("./internal\\app.go"))
	require.Equal(t, "backend\x00services/backend\x00", scope.KeyPrefix())
}

func TestFileAnalysisScopeContainsPath(t *testing.T) {
	scope := FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}

	require.True(t, scope.ContainsPath("internal/app.go", []string{"internal"}))
	require.True(t, scope.ContainsPath("internal/app.go", []string{"internal/app.go"}))
	require.False(t, scope.ContainsPath("cmd/main.go", []string{"internal"}))
}
