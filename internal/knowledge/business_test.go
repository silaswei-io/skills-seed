package knowledge

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBusinessPatternGroupsUseScopePathWhenSourcePathIsUnavailable(t *testing.T) {
	pattern := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	pattern.ScopePath = "plugins/capability_lifecycle"

	groups := BusinessPatternGroups("en-US", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.Equal(t, "capability-lifecycle", groups[0].ID)
	require.Equal(t, "Capability Lifecycle", groups[0].Title)
}

func TestBusinessPatternGroupsFallBackToPatternTextWhenPathIsUnavailable(t *testing.T) {
	pattern := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	pattern.SetDescription("HSM 与 CA 管理需要复用既有生命周期流程")

	groups := BusinessPatternGroups("zh-CN", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.NotEmpty(t, groups[0].ID)
}

func TestBusinessPatternGroupsFallBackToSourcePath(t *testing.T) {
	pattern := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/capability/entry.ext", Symbol: "ExistingEntry"}}

	groups := BusinessPatternGroups("en-US", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.Equal(t, "capability", groups[0].ID)
}

func TestBusinessPatternGroupsPreferStableSourcePathOverScopePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "internal/service/handle_data_report.go", want: "handle-data-report"},
		{path: "internal/service/handle_data_report_tp_1.go", want: "handle-data-report-tp"},
		{path: "internal/svc/hsm_reloader.go", want: "hsm"},
		{path: "internal/async/task_manager.go", want: "async"},
		{path: "internal/logic/node/config.go", want: "node"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			pattern := domain.NewPattern("pattern", "Pattern", domain.CategoryBusiness)
			pattern.ScopePath = "unstable/ai/title"
			pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: tt.path}}

			groups := BusinessPatternGroups("en-US", []domain.Pattern{*pattern})

			require.Len(t, groups, 1)
			require.Equal(t, tt.want, groups[0].ID)
		})
	}
}

func TestTitleFromWordsPreservesUnicode(t *testing.T) {
	require.Equal(t, "Éclair", TitleFromWords([]string{"éclair"}))
}
