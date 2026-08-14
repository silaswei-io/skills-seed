package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSkillTriggerDescriptionCoversProjectTaskSurface(t *testing.T) {
	profile := &domain.ProjectProfile{ProjectName: "inventory", Language: "go"}

	chinese := skillTriggerDescription("fallback", "zh-CN", profile)
	require.Contains(t, chinese, "inventory")
	require.Contains(t, chinese, "需求实现")
	require.Contains(t, chinese, "问题排查")
	require.Contains(t, chinese, "业务流程")
	require.Contains(t, chinese, "接口与数据契约")
	require.Contains(t, chinese, "项目规则")
	require.Contains(t, chinese, "工作流")
	require.Contains(t, chinese, "验证边界")
	require.NotContains(t, chinese, "inventory go")

	english := skillTriggerDescription("fallback", "en-US", profile)
	require.Contains(t, english, "inventory")
	require.Contains(t, english, "requirements")
	require.Contains(t, english, "debugging")
	require.Contains(t, english, "business flows")
	require.Contains(t, english, "APIs and data contracts")
	require.Contains(t, english, "project rules")
	require.Contains(t, english, "workflows")
	require.Contains(t, english, "verification boundaries")
	require.NotContains(t, english, "inventory go")
}
