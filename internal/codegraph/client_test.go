package codegraph

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

func TestClientInitializesMissingIndex(t *testing.T) {
	root := t.TempDir()
	runner := newFakeRunner()
	runner.outputs["status"] = "ready"
	client := NewClient("codegraph")
	client.runner = runner
	client.locks = &projectLocks{locks: map[string]*sync.Mutex{}}

	status, err := client.EnsureReady(context.Background(), root)

	require.NoError(t, err)
	require.True(t, status.Initialized)
	require.Equal(t, "ready", strings.TrimSpace(status.Output))
	require.Equal(t, []string{"init -i", "status"}, runner.callKeys())
}

func TestClientRepairsSyncFailure(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".codegraph"), 0755))
	runner := newFakeRunner()
	runner.failures["sync"] = []error{errors.New("broken index")}
	runner.outputs["status"] = "ready"
	client := NewClient("codegraph")
	client.runner = runner
	client.locks = &projectLocks{locks: map[string]*sync.Mutex{}}

	status, err := client.EnsureReady(context.Background(), root)

	require.NoError(t, err)
	require.True(t, status.Repaired)
	require.Equal(t, []string{"sync", "init -i", "sync", "status"}, runner.callKeys())
}

type fakeRunner struct {
	outputs  map[string]string
	failures map[string][]error
	calls    [][]string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{outputs: map[string]string{}, failures: map[string][]error{}}
}

func (r *fakeRunner) LookPath(command string) (string, error) {
	return command, nil
}

func (r *fakeRunner) Run(_ context.Context, _, _ string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, append([]string(nil), args...))
	if failures := r.failures[key]; len(failures) > 0 {
		r.failures[key] = failures[1:]
		return r.outputs[key], failures[0]
	}
	return r.outputs[key], nil
}

func (r *fakeRunner) callKeys() []string {
	keys := make([]string, 0, len(r.calls))
	for _, call := range r.calls {
		keys = append(keys, strings.Join(call, " "))
	}
	return keys
}
