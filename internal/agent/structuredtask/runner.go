// Package structuredtask 统一编排结构化 Agent 任务的输入、提示词和调用过程。
package structuredtask

import (
	"context"
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/prompts"
)

// BuildPromptData 使用一次性输入会话准备提示词数据。
type BuildPromptData func(*agent.PromptInputSession) (map[string]interface{}, error)

// Call 执行已经渲染完成的结构化 Agent 调用。
type Call func(context.Context, string, string, string, agent.RuntimeTask) (string, error)

// Task 描述一个结构化 Agent 任务的稳定运行契约。
type Task struct {
	Operation      string
	Template       string
	InputPrefix    string
	OutputContract string
	Runtime        agent.RuntimeTask
	Build          BuildPromptData
}

// Runner 负责执行 provider 无关的结构化任务编排。
type Runner struct {
	renderer prompts.Renderer
	call     Call
}

// New 创建结构化任务执行器。
func New(renderer prompts.Renderer, call Call) *Runner {
	return &Runner{renderer: renderer, call: call}
}

// Run 准备运行时输入、渲染提示词并交给 provider 执行。
func (r *Runner) Run(ctx context.Context, task Task) (string, error) {
	if r == nil || r.renderer == nil {
		return "", fmt.Errorf("structured task renderer is not configured")
	}
	if r.call == nil {
		return "", fmt.Errorf("structured task caller is not configured")
	}
	if task.Build == nil {
		return "", fmt.Errorf("structured task %q prompt data builder is not configured", task.Operation)
	}

	inputs, err := agent.NewPromptInputSessionForContext(ctx, task.InputPrefix)
	if err != nil {
		return "", err
	}
	defer inputs.Cleanup()

	data, err := task.Build(inputs)
	if err != nil {
		return "", err
	}
	prompt, err := r.renderer.RenderForRuntimeTask(task.Template, data, prompts.RuntimeTask{
		ID:   task.Runtime.ID,
		Slug: task.Runtime.Slug,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("structured task %q rendered an empty prompt", task.Operation)
	}
	return r.call(ctx, task.Operation, prompt, task.OutputContract, task.Runtime)
}
