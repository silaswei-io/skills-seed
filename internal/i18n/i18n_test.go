package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		lang    string
		wantErr bool
	}{
		{"zh-CN", "zh-CN", false},
		{"en-US", "en-US", false},
		{"empty defaults to zh-CN", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.lang)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGet(t *testing.T) {
	err := Init("zh-CN")
	assert.NoError(t, err)

	tests := []struct {
		name      string
		key       string
		wantEmpty bool
	}{
		{"existing key", "LearnTitle", false},
		{"another existing key", "ProgressLearnCurrentPrepareProject", false},
		{"non-existing key returns key itself", "NonExistingKey12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(tt.key)
			if tt.wantEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestGetEnglish(t *testing.T) {
	err := Init("en-US")
	assert.NoError(t, err)

	result := Get("LearnTitle")
	assert.NotEmpty(t, result)
}

func TestGetForLocaleDoesNotMutateGlobalLocale(t *testing.T) {
	err := Init("zh-CN")
	assert.NoError(t, err)

	assert.Equal(t, "Summary", GetForLocale("en-US", "CommandDocsHeaderSummary"))
	assert.Equal(t, "摘要", Get("CommandDocsHeaderSummary"))
}

func TestActiveLocalesHaveMatchingKeys(t *testing.T) {
	load := func(path string) map[string]map[string]string {
		data, err := localeFS.ReadFile(path)
		require.NoError(t, err)
		messages := make(map[string]map[string]string)
		require.NoError(t, unmarshalToml(data, &messages))
		return messages
	}

	english := load("locales/active.en-US.toml")
	chinese := load("locales/active.zh-CN.toml")
	require.Len(t, english, len(chinese))
	for key := range english {
		require.Contains(t, chinese, key, "missing zh-CN translation for %s", key)
	}
}

func TestGetWithoutInit(t *testing.T) {
	// 未初始化时应自动初始化并返回结果
	// 重置 localizer 为 nil 模拟未初始化
	// 但因为包级别变量，无法轻易重置，所以只测试 Get 能正常返回
	result := Get("LearnTitle")
	assert.NotEmpty(t, result)
}

func TestGetWithParams(t *testing.T) {
	err := Init("zh-CN")
	assert.NoError(t, err)

	result := GetWithParams("LearnTitle", map[string]interface{}{
		"count": 5,
	})
	assert.NotEmpty(t, result)
}

func TestGetWithParamsNonExisting(t *testing.T) {
	err := Init("zh-CN")
	assert.NoError(t, err)

	result := GetWithParams("NonExistingKey99999", map[string]interface{}{
		"name": "test",
	})
	assert.Equal(t, "NonExistingKey99999", result)
}

func TestLearningSummariesDistinguishCandidatesFromNormalizedWrites(t *testing.T) {
	params := map[string]interface{}{
		"Projects":        1,
		"ChangedProjects": 1,
		"Changed":         39,
		"Deleted":         0,
		"Skipped":         0,
		"Patterns":        5,
		"Saved":           7,
		"Retired":         0,
		"Duration":        "5s",
	}
	keys := []string{
		"SyncLearnCompleted",
		"SyncWorkspaceLearnCompleted",
		"ChangeLogLearnProjectSummary",
		"ChangeLogLearnWorkspaceSummary",
		"LearnJournalSummaryCounts",
		"SyncJournalLearnSummary",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			chinese := GetForLocaleWithParams(LocaleChinese, key, params)
			require.Contains(t, chinese, "发现候选模式 5 个")
			require.Contains(t, chinese, "规范化写入 7 个")

			english := GetForLocaleWithParams(LocaleEnglish, key, params)
			require.Contains(t, english, "5 candidate patterns found")
			require.Contains(t, english, "7 normalized patterns written")
		})
	}

	analysisParams := map[string]interface{}{"PatternsCount": 5}
	require.Contains(t, GetForLocaleWithParams(LocaleChinese, "LearnCurrentResult", analysisParams), "审查后候选模式数: 5")
	require.Contains(t, GetForLocaleWithParams(LocaleEnglish, "LearnCurrentResult", analysisParams), "candidate patterns after review: 5")
}
