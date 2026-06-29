package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyProjectProfileDeltaKeepsExistingGlobalNarrative(t *testing.T) {
	base := &ProjectProfile{
		ProjectName:  "demo",
		Language:     "go",
		Summary:      "demo 是覆盖认证、密钥和首页能力的后端服务",
		Architecture: "demo 使用分层架构组织 API、logic、service 和 model",
	}
	delta := ProjectProfileDelta{
		Summary:      "home-info 单元实现首页仪表盘",
		Architecture: "home-info 单元采用 go-zero 分层架构",
		KeyModules: []ModuleInfo{
			{Name: "home", Path: "internal/logic/home", Description: "dashboard"},
		},
	}

	merged := ApplyProjectProfileDelta(base, delta, "demo", "go")

	require.Equal(t, base.Summary, merged.Summary)
	require.Equal(t, base.Architecture, merged.Architecture)
	require.Len(t, merged.KeyModules, 1)
	require.Equal(t, "internal/logic/home", merged.KeyModules[0].Path)
}

func TestApplyProjectProfileDeltaBackfillsMissingGlobalNarrative(t *testing.T) {
	merged := ApplyProjectProfileDelta(&ProjectProfile{}, ProjectProfileDelta{
		Summary:      "first learned summary",
		Architecture: "first learned architecture",
	}, "demo", "go")

	require.Equal(t, "first learned summary", merged.Summary)
	require.Equal(t, "first learned architecture", merged.Architecture)
}
