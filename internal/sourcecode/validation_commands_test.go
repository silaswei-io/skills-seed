package sourcecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDiscoverValidationCommandsUsesRepositoryEvidence(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "service_test.go"), []byte("package demo\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".jzero.yaml"), []byte("gen:\n  hooks:\n    after:\n      - jzero format\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("RUN --mount=type=cache,target=/go/pkg CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -ldflags=\"$LDFLAGS\" -o /dist/app main.go \\\n    && jzero gen swagger\n"), 0644))

	tests, err := DiscoverGoTests(root)
	require.NoError(t, err)
	commands := DiscoverValidationCommands(root, tests)

	require.Contains(t, validationCommandNames(commands), "go test ./...")
	require.Contains(t, validationCommandNames(commands), "jzero gen")
	require.Contains(t, validationCommandNames(commands), "jzero format")
	require.Contains(t, validationCommandNames(commands), "CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -ldflags=\"$LDFLAGS\" -o /dist/app main.go")
}

func validationCommandNames(commands []domain.ValidationCommand) []string {
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Command)
	}
	return names
}
