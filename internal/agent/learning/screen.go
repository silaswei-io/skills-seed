package learning

import (
	"context"
	"encoding/json"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
)

// ScreenSemanticGaps 对本地图无法定性的候选做一次紧凑语义准入。
func ScreenSemanticGaps(ctx context.Context, rt Runtime, req *agent.SemanticScreenRequest) (*agent.SemanticScreenResult, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-semantic-screen", ""))
	task.PromptOnly = true
	output, _, err := call(ctx, rt, "ScreenSemanticGaps", "learning-semantic-screen", "skills-seed-learning-semantic-screen", aicontract.ContractSemanticScreen, task, agent.Conversation{}, func(*agent.PromptInputSession) (map[string]interface{}, error) {
		return map[string]interface{}{"Input": string(data)}, nil
	})
	if err != nil {
		return nil, err
	}
	return parser.ParseSemanticScreenResult(output)
}
