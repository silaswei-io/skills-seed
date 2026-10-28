package sourcecode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestResolverValidatesLocalEvidenceWithoutExternalCLI(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte("package demo\nfunc Publish() {}\n"), 0600))
	// 旧 provider 配置不能重新启用外部 CLI 或第二份索引。
	for _, provider := range []config.StructuralProvider{config.StructuralProviderTreeSitter, config.StructuralProviderAuto, config.StructuralProviderCodeGraph} {
		catalog, err := NewResolver(config.StructuralConfig{Provider: provider}).Resolve(context.Background(), root, []Reference{{Path: "service.go", Name: "Publish", Kind: "function"}})
		require.NoError(t, err)
		_, ok := FindSymbol(catalog["service.go"], "Publish", "function", 2)
		require.True(t, ok)
	}
}
