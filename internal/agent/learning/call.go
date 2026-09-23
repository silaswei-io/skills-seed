// Package learning 提供 Claude/Codex 共享的当前学习结构化任务实现。
package learning

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/structuredtask"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
)

func call(
	ctx context.Context,
	rt Runtime,
	operation, templateName, inputPrefix, outputContract string,
	task agent.RuntimeTask,
	conversation agent.Conversation,
	build func(*agent.PromptInputSession) (map[string]interface{}, error),
) (string, agent.Conversation, error) {
	return callWithOptions(ctx, rt, operation, templateName, inputPrefix, outputContract, aicontract.StructuredOutputOptions{}, task, conversation, build)
}

func callWithOptions(
	ctx context.Context,
	rt Runtime,
	operation, templateName, inputPrefix, outputContract string,
	opts aicontract.StructuredOutputOptions,
	task agent.RuntimeTask,
	conversation agent.Conversation,
	build func(*agent.PromptInputSession) (map[string]interface{}, error),
) (string, agent.Conversation, error) {
	runner := structuredtask.NewWithResult(rt.PromptRenderer(), func(ctx context.Context, operation, prompt, contract string, runtime agent.RuntimeTask) (structuredtask.Result, error) {
		result, err := rt.InvokeStructured(ctx, agent.StructuredInvoke{
			Operation:      operation,
			Prompt:         prompt,
			OutputContract: contract,
			Options:        opts,
			Conversation:   conversation,
			Runtime:        runtime,
		})
		return structuredtask.Result{Output: result.Output, Conversation: result.Conversation}, err
	})
	result, err := runner.RunResult(ctx, structuredtask.Task{
		Operation:      operation,
		Template:       templateName,
		InputPrefix:    inputPrefix,
		OutputContract: outputContract,
		Runtime:        task,
		Build:          build,
	})
	if err == nil {
		runtimecontext.AgentCalls(ctx).Add(operation)
	}
	return result.Output, result.Conversation, err
}
