package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequireResultRejectsNil(t *testing.T) {
	var result *AnalyzeResult
	require.Error(t, RequireResult(result, "AnalyzeCode"))

	result = &AnalyzeResult{}
	require.NoError(t, RequireResult(result, "AnalyzeCode"))
}
