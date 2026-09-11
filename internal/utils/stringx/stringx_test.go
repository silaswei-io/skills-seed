package stringx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeStructureSummary(t *testing.T) {
	got := NormalizeStructureSummary("demo\r\n\u00a0\u00a0cmd\n&nbsp;&nbsp;main.go   \n")

	require.Equal(t, "demo\n  cmd\n  main.go", got)
}

func TestFirstNonEmptyPreservesOriginalValue(t *testing.T) {
	require.Equal(t, "  value  ", FirstNonEmpty("", " \t", "  value  ", "later"))
	require.Empty(t, FirstNonEmpty("", " \n"))
}

func TestFirstNonBlankReturnsTrimmedValue(t *testing.T) {
	require.Equal(t, "value", FirstNonBlank("", " \t", "  value  ", "later"))
	require.Empty(t, FirstNonBlank("", " \n"))
}

func TestEmptyIfNil(t *testing.T) {
	require.NotNil(t, EmptyIfNil(nil))
	values := []string{"value"}
	require.Same(t, &values[0], &EmptyIfNil(values)[0])
}

func TestUniqueNonBlankTrimsAndPreservesFirstOccurrence(t *testing.T) {
	require.Equal(t, []string{"first", "second"}, UniqueNonBlank([]string{
		" first ", "", "first", " second", "second ", "\t",
	}))
}

func TestUniqueNonEmptyPreservesWhitespaceAndOrder(t *testing.T) {
	require.Equal(t, []string{" first ", "first", " "}, UniqueNonEmpty([]string{
		" first ", "", "first", " first ", " ",
	}))
}
