package commandutil

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestRequireAgentAvailableRejectsUnavailableAgent(t *testing.T) {
	err := RequireAgentAvailable(&container.Container{
		Agent: &mocks.MockAgent{NameVal: "missing-agent", AvailableVal: false},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "missing-agent")
}

func TestRequireAgentAvailableAllowsAvailableAgent(t *testing.T) {
	err := RequireAgentAvailable(&container.Container{
		Agent: &mocks.MockAgent{NameVal: "agent", AvailableVal: true},
	})

	require.NoError(t, err)
}
