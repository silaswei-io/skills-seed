package learning

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/stretchr/testify/require"
)

type rendererStub struct{}

func (rendererStub) Render(string, interface{}) (string, error) { return "prompt", nil }

func (rendererStub) RenderForRuntimeTask(string, interface{}, prompts.RuntimeTask) (string, error) {
	return "prompt", nil
}

type fakeLearningRuntime struct {
	name   string
	invoke func(context.Context, agent.StructuredInvoke) (agent.StructuredResult, error)
}

func (f fakeLearningRuntime) Name() string { return f.name }

func (f fakeLearningRuntime) PromptRenderer() prompts.Renderer { return rendererStub{} }

func (f fakeLearningRuntime) InvokeStructured(ctx context.Context, in agent.StructuredInvoke) (agent.StructuredResult, error) {
	return f.invoke(ctx, in)
}

func TestPlanLearningAgendaUsesSharedContract(t *testing.T) {
	var gotContract string
	rt := fakeLearningRuntime{
		name: "fake",
		invoke: func(_ context.Context, in agent.StructuredInvoke) (agent.StructuredResult, error) {
			gotContract = in.OutputContract
			return agent.StructuredResult{Output: `{
				"focuses":[{"id":"f1","name":"core","analysis_depth":"standard","entry_paths":["main.go"]}],
				"skipped_paths":[],
				"reason":"ok"
			}`}, nil
		},
	}
	result, err := PlanLearningAgenda(context.Background(), rt, &agent.PlanLearningAgendaRequest{
		ProjectName: "demo",
		RootPath:    "/tmp",
		Language:    "go",
		FocusPaths:  []string{"main.go"},
	})
	require.NoError(t, err)
	require.Equal(t, aicontract.ContractPlanLearningAgenda, gotContract)
	require.Len(t, result.Focuses, 1)
	require.Equal(t, "f1", result.Focuses[0].ID)
}
