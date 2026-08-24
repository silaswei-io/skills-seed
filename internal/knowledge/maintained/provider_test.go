package maintained

import (
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestProviderLoad(t *testing.T) {
	provider := New(ruleRepositoryStub{rules: []domain.Rule{{ID: "boundary"}}}, workflowRepositoryStub{workflows: []domain.Workflow{{ID: "verify"}}})

	snapshot, err := provider.Load()

	require.NoError(t, err)
	require.Equal(t, []domain.Rule{{ID: "boundary"}}, snapshot.Rules)
	require.Equal(t, []domain.Workflow{{ID: "verify"}}, snapshot.Workflows)
}

func TestProviderLoadReturnsRepositoryError(t *testing.T) {
	provider := New(ruleRepositoryStub{err: errors.New("rules unavailable")}, nil)

	_, err := provider.Load()

	require.ErrorContains(t, err, "list user rules")
}

type ruleRepositoryStub struct {
	rules []domain.Rule
	err   error
}

func (s ruleRepositoryStub) List() ([]domain.Rule, error) { return s.rules, s.err }
func (ruleRepositoryStub) Get(string) (*domain.Rule, error) {
	return nil, nil
}
func (ruleRepositoryStub) Save(domain.Rule) error { return nil }

type workflowRepositoryStub struct {
	workflows []domain.Workflow
	err       error
}

func (s workflowRepositoryStub) List() ([]domain.Workflow, error) { return s.workflows, s.err }
func (workflowRepositoryStub) Get(string) (*domain.Workflow, error) {
	return nil, nil
}
func (workflowRepositoryStub) Save(domain.Workflow) error { return nil }
func (workflowRepositoryStub) ScriptsDir(string) string   { return "" }
func (workflowRepositoryStub) Scripts(string) ([]domain.WorkflowScript, error) {
	return nil, nil
}
