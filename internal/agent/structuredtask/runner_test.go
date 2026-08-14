package structuredtask

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/stretchr/testify/require"
)

type rendererStub struct {
	prompt string
	err    error
	task   prompts.RuntimeTask
}

func (s *rendererStub) Render(string, interface{}) (string, error) {
	return s.prompt, s.err
}

func (s *rendererStub) RenderForRuntimeTask(_ string, _ interface{}, task prompts.RuntimeTask) (string, error) {
	s.task = task
	return s.prompt, s.err
}

func TestRunnerRun(t *testing.T) {
	renderer := &rendererStub{prompt: "prompt"}
	runtimeTask := agent.RuntimeTask{ID: "runtime-id", Slug: "runtime-slug"}
	called := false
	runner := New(renderer, func(_ context.Context, operation, prompt, contract string, runtime agent.RuntimeTask) (string, error) {
		called = true
		require.Equal(t, "Analyze", operation)
		require.Equal(t, "prompt", prompt)
		require.Equal(t, "Output", contract)
		require.Equal(t, runtimeTask, runtime)
		return "result", nil
	})

	result, err := runner.Run(context.Background(), Task{
		Operation:      "Analyze",
		Template:       "analyze",
		InputPrefix:    "analyze-input",
		OutputContract: "Output",
		Runtime:        runtimeTask,
		Build: func(*agent.PromptInputSession) (map[string]interface{}, error) {
			return map[string]interface{}{"value": "input"}, nil
		},
	})

	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "result", result)
	require.Equal(t, prompts.RuntimeTask{ID: runtimeTask.ID, Slug: runtimeTask.Slug}, renderer.task)
}

func TestRunnerRunRejectsRenderFailureAndEmptyPrompt(t *testing.T) {
	tests := []struct {
		name     string
		renderer *rendererStub
	}{
		{name: "render error", renderer: &rendererStub{err: errors.New("render failed")}},
		{name: "empty prompt", renderer: &rendererStub{prompt: "  "}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			runner := New(test.renderer, func(context.Context, string, string, string, agent.RuntimeTask) (string, error) {
				called = true
				return "", nil
			})
			_, err := runner.Run(context.Background(), Task{
				Operation: "Analyze",
				Build: func(*agent.PromptInputSession) (map[string]interface{}, error) {
					return map[string]interface{}{}, nil
				},
			})
			require.Error(t, err)
			require.False(t, called)
		})
	}
}

func TestRunnerRunStopsWhenPromptDataFails(t *testing.T) {
	runner := New(&rendererStub{prompt: "prompt"}, func(context.Context, string, string, string, agent.RuntimeTask) (string, error) {
		t.Fatal("caller must not run")
		return "", nil
	})
	_, err := runner.Run(context.Background(), Task{
		Operation: "Analyze",
		Build: func(*agent.PromptInputSession) (map[string]interface{}, error) {
			return nil, errors.New("build failed")
		},
	})
	require.EqualError(t, err, "build failed")
}
