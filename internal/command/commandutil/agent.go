package commandutil

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// RequireAgentAvailable fails before commands invoke an unavailable AI agent
func RequireAgentAvailable(cont *container.Container) error {
	if cont == nil || cont.Agent == nil {
		return nil
	}
	if cont.Agent.IsAvailable() {
		return nil
	}
	return fmt.Errorf("%s", i18n.GetWithParams("AgentNotAvailable", map[string]interface{}{"Agent": cont.Agent.Name()}))
}
