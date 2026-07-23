package commandutil

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// RequireAgentAvailable 在命令调用不可用的 AI Agent 前提前失败。
func RequireAgentAvailable(cont *container.Container) error {
	if cont != nil && cont.Agent != nil && cont.Agent.IsAvailable() {
		return nil
	}
	return fmt.Errorf("%s", i18n.GetWithParams("AgentNotAvailable", map[string]interface{}{"Agent": configuredAgentName(cont)}))
}

func configuredAgentName(cont *container.Container) string {
	if cont == nil {
		return "unknown"
	}
	if cont.Agent != nil {
		return cont.Agent.Name()
	}
	if cont.ConfigRepo != nil {
		if name := cont.ConfigRepo.GetAgentConfig().Engine; name != "" {
			return name
		}
	}
	return "unknown"
}
