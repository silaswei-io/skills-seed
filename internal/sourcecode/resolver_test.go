package sourcecode

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestNewResolverUsesTreeSitterOnlyWhenExplicit(t *testing.T) {
	_, treeSitter := NewResolver(config.StructuralConfig{Provider: config.StructuralProviderTreeSitter}).(treeSitterResolver)
	_, automatic := NewResolver(config.StructuralConfig{Provider: config.StructuralProviderAuto}).(*codeGraphResolver)

	require.True(t, treeSitter)
	require.True(t, automatic)
}

func TestCodeGraphResolverQueriesUniqueNamesAndFiltersExactPaths(t *testing.T) {
	client := &fakeCodeGraphClient{outputs: map[string]string{
		"Publish": `[
			{"node":{"name":"Publish","qualifiedName":"Service::Publish","kind":"method","filePath":"internal/service.go","startLine":12,"signature":"(ctx context.Context) error"}},
			{"node":{"name":"Publish","qualifiedName":"Other::Publish","kind":"method","filePath":"vendor/other.go","startLine":8,"signature":"()"}}
		]`,
	}}
	resolver := newCodeGraphResolver(client)

	catalog, err := resolver.Resolve(context.Background(), "/project", []Reference{
		{Path: "internal/service.go", Name: "Service.Publish", Kind: "method"},
		{Path: "internal/service.go", Name: "Publish", Kind: "method"},
	})

	require.NoError(t, err)
	require.Equal(t, 1, client.queryCount("Publish"))
	require.Equal(t, []Symbol{{
		Name:      "Publish",
		Kind:      "method",
		Line:      12,
		Signature: "Service.Publish(ctx context.Context) error",
	}}, catalog["internal/service.go"])
	require.NotContains(t, catalog, "vendor/other.go")
}

func TestCodeGraphResolverReturnsQueryFailure(t *testing.T) {
	client := &fakeCodeGraphClient{queryErr: errors.New("query failed")}
	resolver := newCodeGraphResolver(client)

	_, err := resolver.Resolve(context.Background(), "/project", []Reference{{Path: "main.go", Name: "Run"}})

	require.ErrorContains(t, err, "query CodeGraph symbol")
}

type fakeCodeGraphClient struct {
	mu       sync.Mutex
	outputs  map[string]string
	queryErr error
	queries  []string
}

func (c *fakeCodeGraphClient) EnsureReady(context.Context, string) (*codegraph.Status, error) {
	return &codegraph.Status{}, nil
}

func (c *fakeCodeGraphClient) Run(_ context.Context, _ string, args ...string) (string, error) {
	name := args[len(args)-1]
	c.mu.Lock()
	c.queries = append(c.queries, name)
	c.mu.Unlock()
	if c.queryErr != nil {
		return "", c.queryErr
	}
	return c.outputs[name], nil
}

func (c *fakeCodeGraphClient) queryCount(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, query := range c.queries {
		if strings.EqualFold(query, name) {
			count++
		}
	}
	return count
}
