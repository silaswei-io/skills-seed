package workflow

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestWorkflowDescriptionUsesOptimizedContentSummary(t *testing.T) {
	workflow := domain.Workflow{
		ID:   "deploy",
		Name: "deploy",
		Contexts: []domain.WorkflowContext{
			{Content: "发布前检查环境变量和构建产物，发布后执行 smoke test"},
		},
		Content: "# deploy\n\n## 适用场景\n发布流程覆盖上线前环境与产物核验，以及上线后的冒烟验证。\n\n## 步骤\n- 检查配置\n",
	}

	require.Equal(t, "发布流程覆盖上线前环境与产物核验，以及上线后的冒烟验证。", Summary(workflow, "zh-CN"))
}

func TestWorkflowDescriptionSkipsApplicabilityLabelLine(t *testing.T) {
	workflow := domain.Workflow{
		ID:      "requirement-analysis-workflow",
		Name:    "需求分析与方案生成工作流",
		Content: "# 需求分析与方案生成工作流\n\n## Applicability\n\n适用于以下场景：\n- 接收到新的功能需求清单\n- 收到重大变更或重构需求\n",
	}

	require.Equal(t, "接收到新的功能需求清单", Summary(workflow, "zh-CN"))
}

func TestWorkflowDescriptionIgnoresOriginalContextWithoutOptimizedContent(t *testing.T) {
	workflow := domain.Workflow{
		ID: "deploy",
		Contexts: []domain.WorkflowContext{
			{Content: "发布前检查环境变量"},
		},
	}

	require.Empty(t, Summary(workflow, "zh-CN"))
}

func TestRenderWorkflowOutputUsesLocalizedTemplate(t *testing.T) {
	workflow := domain.Workflow{
		ID:       "deploy",
		Name:     "Deploy",
		Contexts: []domain.WorkflowContext{{Content: "Run smoke tests"}},
		Scripts:  []domain.WorkflowScript{{Path: "smoke.sh"}},
	}

	english, err := renderWorkflowOutput(workflow, "en-US")
	require.NoError(t, err)
	require.Contains(t, english, "## Context")
	require.Contains(t, english, "## Scripts")
	require.NotContains(t, english, "## 上下文")

	chinese, err := renderWorkflowOutput(workflow, "zh-CN")
	require.NoError(t, err)
	require.Contains(t, chinese, "## 上下文")
	require.Contains(t, chinese, "## 脚本")
}
