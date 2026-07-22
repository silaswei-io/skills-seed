package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSkillTriggerDescriptionDoesNotInferProjectSemantics(t *testing.T) {
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

	require.Contains(t, description, "demo go")
	require.Contains(t, description, "调试")
	require.Contains(t, description, "测试")
	require.NotContains(t, description, "AuthorizeAndApply")
	require.NotContains(t, description, "权限")
}

func TestValidationGapsReportMissingTestStaticCheckAndScopeMatrix(t *testing.T) {
	profile := &domain.ProjectProfile{ValidationCommands: []domain.ValidationCommand{{
		Command: "go build ./...",
		Type:    "build",
	}}}

	gaps := validationGaps(profile, nil, "zh-CN")

	require.Len(t, gaps, 3)
	require.Contains(t, gaps[0], "测试命令")
	require.Contains(t, gaps[1], "静态检查命令")
	require.Contains(t, gaps[2], "验证矩阵")
}

func TestPatternsForTemplatePreservesLearnedStatementWithoutGrantingAuthority(t *testing.T) {
	pattern := domain.NewPattern("logic-structure", "Logic Structure", domain.CategoryStructure)
	pattern.Source = domain.SourceLearnedCurrent
	pattern.SetDescription("所有 Logic 必须严格使用固定字段结构")
	pattern.SetRule("All Logic values must use the fixed field structure")
	originalRule := pattern.Rule

	view := patternsForTemplate([]domain.Pattern{*pattern})

	require.Len(t, view, 1)
	require.Equal(t, pattern.Description, view[0].Description)
	require.Empty(t, view[0].Rule)
	require.False(t, view[0].AllowsHardConstraint())
	require.Equal(t, originalRule, pattern.Rule)
}
