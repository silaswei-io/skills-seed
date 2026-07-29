package stringx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeStructureSummary(t *testing.T) {
	got := NormalizeStructureSummary("demo\r\n\u00a0\u00a0cmd\n&nbsp;&nbsp;main.go   \n")

	require.Equal(t, "demo\n  cmd\n  main.go", got)
}
