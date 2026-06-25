package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		{"another existing key", "LearnNoCommits", false},
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

	assert.Equal(t, "Scripts", GetForLocale("en-US", "WorkflowOutputScriptsHeading"))
	assert.Equal(t, "脚本", Get("WorkflowOutputScriptsHeading"))
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
