package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitBusinessPatternGroupsKeepsSparseGroupsInline(t *testing.T) {
	one := domain.NewPattern("admin-flow", "管理员流程", domain.CategoryBusiness)
	one.ScopePath = "internal/logic/system/admin"
	one.SetRule("修改管理员流程时必须保持角色校验")
	one.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/logic/system/admin/create.go", Line: 48}}
	twoA := domain.NewPattern("key-create", "密钥创建", domain.CategoryBusiness)
	twoA.ScopePath = "plugins/key_manage"
	twoA.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "plugins/key_manage/internal/logic/key_manage/create.go", Line: 12}}
	twoB := domain.NewPattern("key-delete", "密钥删除", domain.CategoryBusiness)
	twoB.ScopePath = "plugins/key_manage"
	twoB.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "plugins/key_manage/internal/logic/key_manage/delete.go", Line: 12}}
	groups := businessPatternGroups("zh-CN", []domain.Pattern{*one, *twoA, *twoB})

	detailGroups, inlineGroups := splitBusinessPatternGroups(groups)

	require.Len(t, detailGroups, 1)
	assert.Equal(t, "Key Manage", detailGroups[0].Title)
	require.Len(t, inlineGroups, 1)
	assert.Equal(t, "Admin", inlineGroups[0].Title)
}
