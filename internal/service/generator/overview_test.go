package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestLearnedCoverageSummaryReportsTotalAndPreview(t *testing.T) {
	profile := &domain.ProjectProfile{
		KeyModules: []domain.ModuleInfo{
			{Name: "module-1"},
			{Name: "module-2"},
			{Name: "module-3"},
			{Name: "module-4"},
			{Name: "module-5"},
			{Name: "module-6"},
			{Name: "module-7"},
			{Name: "module-8"},
			{Name: "module-9"},
		},
	}

	summary := learnedCoverageSummary(profile, "zh-CN")

	assert.Contains(t, summary, "已覆盖 9 个模块/业务域")
	assert.Contains(t, summary, "仅列出前 8 个")
	assert.Contains(t, summary, "完整列表见关键模块")
}
