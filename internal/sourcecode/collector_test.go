package sourcecode

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/stretchr/testify/require"
)

func TestCodeGraphSymbolCollectorPreparesIndexOnce(t *testing.T) {
	client := &recordingCodeGraphCollectorClient{output: `{"nodes":[{"kind":"function","name":"NewService","filePath":"service.go","startLine":4,"signature":"()"}]}`}
	collector := &codeGraphSymbolCollector{client: client, readiness: map[string]error{}, attempted: map[string]bool{}}

	first, err := collector.Collect(context.Background(), "/project", []string{"service.go"})
	require.NoError(t, err)
	second, err := collector.Collect(context.Background(), "/project", []string{"service.go"})
	require.NoError(t, err)
	require.Equal(t, 1, client.readyCalls)
	require.Equal(t, "NewService", first["service.go"][0].Name)
	require.Equal(t, first, second)
}

type recordingCodeGraphCollectorClient struct {
	output     string
	readyCalls int
}

func (c *recordingCodeGraphCollectorClient) EnsureReady(context.Context, string) (*codegraph.Status, error) {
	c.readyCalls++
	return &codegraph.Status{}, nil
}

func (c *recordingCodeGraphCollectorClient) Run(context.Context, string, ...string) (string, error) {
	return c.output, nil
}
