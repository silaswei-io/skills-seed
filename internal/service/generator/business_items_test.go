package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBusinessPatternItemsRouteEachReviewedRuleDirectly(t *testing.T) {
	first := domain.NewPattern("approval", "资源审批", domain.CategoryBusiness)
	second := domain.NewPattern("renewal", "资源续期", domain.CategoryBusiness)
	items := businessPatternItems([]domain.Pattern{*first, *second})

	require.Len(t, items, 2)
	require.Equal(t, "./business/items/"+referenceFileID(first.ID)+".md", items[0].ReferencePath)
	require.Equal(t, "./business/items/"+referenceFileID(second.ID)+".md", items[1].ReferencePath)
}

func TestGroupBusinessResponsibilitiesKeepsDistinctRulesInSmallThemeIndex(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("create", "证书模板创建校验", domain.CategoryBusiness),
		*domain.NewPattern("update", "证书模板更新保护", domain.CategoryBusiness),
		*domain.NewPattern("login", "管理员登录契约", domain.CategoryBusiness),
	}
	groups, singles := groupBusinessResponsibilities(businessPatternItems(patterns))
	require.Len(t, groups, 1)
	require.Equal(t, "证书模板", groups[0].Title)
	require.Len(t, groups[0].Items, 2)
	require.Equal(t, "../items/"+referenceFileID("create")+".md", groups[0].Items[0].ReferencePath)
	require.Len(t, singles, 1)
	require.Equal(t, "管理员登录契约", singles[0].Name)
	require.Empty(t, sharedTitlePrefix("Rule 00", "Rule 01"))
}
