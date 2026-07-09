package analyzer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodeGraphManagerInitializesMissingIndex(t *testing.T) {
	root := t.TempDir()
	runner := newFakeCodeGraphRunner()
	runner.outputs["status"] = "ready"
	manager := newCodeGraphManager("codegraph")
	manager.runner = runner
	manager.locks = &codeGraphProjectLocks{locks: map[string]*sync.Mutex{}}

	status, err := manager.EnsureReady(context.Background(), root)

	require.NoError(t, err)
	require.True(t, status.Initialized)
	require.Equal(t, "ready", strings.TrimSpace(status.Output))
	require.Equal(t, []string{"init -i", "status"}, runner.callKeys())
}

func TestCodeGraphManagerRepairsSyncFailure(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".codegraph"), 0755))
	runner := newFakeCodeGraphRunner()
	runner.failures["sync"] = []error{errors.New("broken index")}
	runner.outputs["status"] = "ready"
	manager := newCodeGraphManager("codegraph")
	manager.runner = runner
	manager.locks = &codeGraphProjectLocks{locks: map[string]*sync.Mutex{}}

	status, err := manager.EnsureReady(context.Background(), root)

	require.NoError(t, err)
	require.True(t, status.Repaired)
	require.Equal(t, []string{"sync", "init -i", "sync", "status"}, runner.callKeys())
}

func TestAutoStructuralProviderFallsBackWhenCodeGraphUnavailable(t *testing.T) {
	provider := &autoStructuralProvider{
		primary:  failingStructuralProvider{err: errCodeGraphUnavailable},
		fallback: staticStructuralProvider{data: &structuralContextData{Source: structuralProviderTreeSitter}},
	}

	data, err := provider.Collect(context.Background(), t.TempDir(), structuralContextRequest{SeedPaths: []string{"main.go"}})

	require.NoError(t, err)
	require.Equal(t, structuralProviderTreeSitter, data.Source)
}

type fakeCodeGraphRunner struct {
	lookErr  error
	outputs  map[string]string
	failures map[string][]error
	calls    [][]string
}

func newFakeCodeGraphRunner() *fakeCodeGraphRunner {
	return &fakeCodeGraphRunner{
		outputs:  map[string]string{},
		failures: map[string][]error{},
	}
}

func (r *fakeCodeGraphRunner) LookPath(command string) (string, error) {
	if r.lookErr != nil {
		return "", r.lookErr
	}
	return command, nil
}

func (r *fakeCodeGraphRunner) Run(ctx context.Context, command, workDir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, append([]string(nil), args...))
	failures := r.failures[key]
	if len(failures) > 0 {
		err := failures[0]
		r.failures[key] = failures[1:]
		return r.outputs[key], err
	}
	return r.outputs[key], nil
}

func (r *fakeCodeGraphRunner) callKeys() []string {
	keys := make([]string, 0, len(r.calls))
	for _, call := range r.calls {
		keys = append(keys, strings.Join(call, " "))
	}
	return keys
}

type failingStructuralProvider struct {
	err error
}

func (p failingStructuralProvider) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error) {
	return nil, p.err
}

type staticStructuralProvider struct {
	data *structuralContextData
}

func (p staticStructuralProvider) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error) {
	return p.data, nil
}
