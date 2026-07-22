package sourcecode

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/stretchr/testify/require"
)

func TestCompatibleKindUsesCanonicalTagFamilies(t *testing.T) {
	tests := []struct {
		requested string
		actual    string
		want      bool
	}{
		{requested: "function", actual: "func", want: true},
		{requested: "method", actual: "func", want: true},
		{requested: "type", actual: "struct", want: true},
		{requested: "type", actual: "class", want: true},
		{requested: "type", actual: "interface", want: true},
		{requested: "type", actual: "func", want: false},
		{requested: "file", actual: "struct", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.requested+"-"+tt.actual, func(t *testing.T) {
			require.Equal(t, tt.want, compatibleKind(tt.requested, tt.actual))
		})
	}
}

func TestExtractSymbolsDoesNotRetainDefinitions(t *testing.T) {
	src := []byte("package service\n\nfunc Start() error { return nil }\n")
	entry := grammars.DetectLanguage("service.go")
	require.NotNil(t, entry)
	lang := entry.Language()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	require.NoError(t, err)
	t.Cleanup(tree.Release)

	symbols := ExtractSymbols(tree.RootNode(), lang, "service.go", src)
	require.NotEmpty(t, symbols)
	require.Empty(t, symbols[0].Signature)

	parsed, err := ParseSymbols("service.go", src)
	require.NoError(t, err)
	require.NotEmpty(t, parsed)
	require.Equal(t, "func Start() error", parsed[0].Signature)
}
