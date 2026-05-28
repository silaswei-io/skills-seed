package commandutil

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// RequireAgentAvailable 在命令调用不可用的 AI Agent 前提前失败。
func RequireAgentAvailable(cont *container.Container) error {
	if cont == nil || cont.Agent == nil {
		return nil
	}
	if cont.Agent.IsAvailable() {
		return nil
	}
	return fmt.Errorf("%s", i18n.GetWithParams("AgentNotAvailable", map[string]interface{}{"Agent": cont.Agent.Name()}))
}
