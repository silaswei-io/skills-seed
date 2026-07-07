package policy

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDisplayPatternTextSoftensLocalChineseHardConstraint(t *testing.T) {
	pattern := domain.NewPattern("local-flow", "局部流程", domain.CategoryBusiness)
	pattern.Confidence = 0.83
	pattern.Frequency = 1
	pattern.SetRule("修改登录流程时必须先解密密码")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/login.go", Line: 10}}

	text := DisplayPatternText(*pattern, "zh-CN")

	assert.Contains(t, text, "定位线索")
	assert.NotContains(t, text, "必须")
	assert.Contains(t, text, "需要先解密密码")
}

func TestDisplayPatternTextKeepsStrongChineseHardConstraint(t *testing.T) {
	pattern := domain.NewPattern("core-flow", "核心流程", domain.CategoryBusiness)
	pattern.Confidence = 0.93
	pattern.Frequency = 3
	pattern.SetRule("修改登录流程时必须先解密密码")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/login.go", Line: 10},
		{Path: "internal/admin.go", Line: 20},
	}

	text := DisplayPatternText(*pattern, "zh-CN")

	assert.Equal(t, "修改登录流程时必须先解密密码", text)
}

func TestDisplayPatternTextSoftensLocalEnglishHardConstraint(t *testing.T) {
	pattern := domain.NewPattern("local-flow", "Local Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.83
	pattern.Frequency = 1
	pattern.SetRule("Login changes must decrypt the password first")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/login.go", Line: 10}}

	text := DisplayPatternText(*pattern, "en-US")

	assert.Contains(t, text, "local hint")
	assert.NotContains(t, text, " must ")
	assert.Contains(t, text, "should decrypt")
}
