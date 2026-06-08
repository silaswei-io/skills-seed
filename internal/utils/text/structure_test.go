package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeStructureSummary(t *testing.T) {
	got := NormalizeStructureSummary("demo\r\n\u00a0\u00a0cmd\n&nbsp;&nbsp;main.go   \n")

	require.Equal(t, "demo\n  cmd\n  main.go", got)
	require.NotContains(t, got, "\u00a0")
	require.NotContains(t, got, "&nbsp;")
	require.NotContains(t, got, "main.go   ")
}
