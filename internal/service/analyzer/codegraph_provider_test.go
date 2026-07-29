package analyzer

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestStructuralProviderUsesCodeGraphForAuto(t *testing.T) {
	provider := newStructuralProvider(config.StructuralConfig{Provider: config.StructuralProviderAuto})

	_, ok := provider.(*codeGraphProvider)
	require.True(t, ok)
}

func TestStructuralProviderUsesTreeSitterOnlyWhenExplicit(t *testing.T) {
	provider := newStructuralProvider(config.StructuralConfig{Provider: config.StructuralProviderTreeSitter})

	_, ok := provider.(*treesitterCollector)
	require.True(t, ok)
}

func TestCodeGraphProviderReportsProgressStages(t *testing.T) {
	provider := newCodeGraphProvider(config.StructuralConfig{MaxSymbols: 5})
	provider.client = fakeCodeGraphClient{
		status: &codegraph.Status{Output: "ready", Initialized: true},
		output: "context",
	}
	var stages []structuralContextStage

	data, err := provider.Collect(context.Background(), t.TempDir(), structuralContextRequest{
		ProjectName: "test",
		SeedPaths:   []string{"internal/auth/service.go"},
		Progress: func(stage structuralContextStage) {
			stages = append(stages, stage)
		},
	})

	require.NoError(t, err)
	require.NotNil(t, data)
	require.Equal(t, []structuralContextStage{
		structuralContextStageCodeGraphIndex,
		structuralContextStageCodeGraphContext,
	}, stages)
}

type fakeCodeGraphClient struct {
	status *codegraph.Status
	output string
}

func (f fakeCodeGraphClient) EnsureReady(ctx context.Context, projectRoot string) (*codegraph.Status, error) {
	return f.status, nil
}

func (f fakeCodeGraphClient) Repair(ctx context.Context, projectRoot string) (*codegraph.Status, error) {
	return f.status, nil
}

func (f fakeCodeGraphClient) Run(ctx context.Context, projectRoot string, args ...string) (string, error) {
	return f.output, nil
}
