package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeGenerationInputsKeepsExistingDirectoryModulePaths(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plugins", "key_manage"), 0755))
	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{{
			Name: "key_manage",
			Path: "plugins/key_manage",
		}},
	}

	sanitized, _ := sanitizeGenerationInputs(profile, nil, root)

	require.Len(t, sanitized.KeyModules, 1)
	assert.Equal(t, "plugins/key_manage", sanitized.KeyModules[0].Path)
}
