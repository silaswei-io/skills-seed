package commandutil

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/container"
)

// RequireAgentAvailable fails before commands invoke an unavailable AI agent
func RequireAgentAvailable(cont *container.Container) error {
	if cont == nil || cont.Agent == nil {
		return nil
	}
	if cont.Agent.IsAvailable() {
		return nil
	}
	return fmt.Errorf("agent %q is not available; check agent command configuration", cont.Agent.Name())
}
