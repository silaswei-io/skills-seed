package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCuratePatternsResultPreservesDecisionFields(t *testing.T) {
	output := `{
  "patterns": [
    {
      "id": "existing-error-handling",
      "name": "错误处理模式",
      "category": "error",
      "description": "统一包装错误并保留调用上下文",
      "rule": "当跨层返回错误时，应该使用 %w 包装上下文",
      "confidence": 0.91,
      "source_ids": ["existing-error-handling", "candidate-error-handling"]
    }
  ],
  "dropped": [{"id":"candidate-generic","reason":"规则过于泛化"}]
}`

	result, err := ParseCuratePatternsResult(output)

	require.NoError(t, err)
	require.Len(t, result.Patterns, 1)
	pattern := result.Patterns[0]
	require.Equal(t, "existing-error-handling", pattern.ID)
	require.Equal(t, []string{"existing-error-handling", "candidate-error-handling"}, pattern.SourceIDs)
	require.Len(t, result.Dropped, 1)
}

func TestParseGenerateFixesResultPreservesSummaryAndWarnings(t *testing.T) {
	output := `{
  "fixes": {
    "main.go": "package main\n"
  },
  "confidence": 0.82,
  "summary": "修复 main.go 的错误包装",
  "warnings": ["未运行集成测试"]
}`

	result, err := ParseGenerateFixesResult(output)

	require.NoError(t, err)
	require.Equal(t, "修复 main.go 的错误包装", result.Summary)
	require.Equal(t, []string{"未运行集成测试"}, result.Warnings)
}
