package knowledge

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTitleFromWordsPreservesUnicode(t *testing.T) {
	require.Equal(t, "Éclair", TitleFromWords([]string{"éclair"}))
}
