package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSkillTriggerDescriptionUsesGenericBusinessHints(t *testing.T) {
	description := skillTriggerDescription("demo", "go", "zh-CN", &domain.ProjectProfile{
		ProjectName: "demo",
		Language:    "go",
		BusinessMethods: []domain.BusinessMethod{
			{
				Name:        "AuthorizeAndApply",
				Description: "validates authorization policy and applies a workflow state transition",
				Usage:       "business orchestration",
				Type:        "domain",
				Function:    "func (s *Service) AuthorizeAndApply(ctx context.Context) error",
			},
		},
	})

	require.Contains(t, description, "业务入口与编排")
	require.Contains(t, description, "权限与策略")
	require.NotContains(t, description, "AuthorizeAndApply")
}
