package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMergePatternsResultPreservesUsablePatternFields(t *testing.T) {
	output := `{
  "merged_patterns": [
    {
      "id": "merged-error-handling",
      "name": "错误处理模式",
      "category": "error",
      "description": "统一包装错误并保留调用上下文",
      "good_example": "return fmt.Errorf(\"create user: %w\", err)",
      "bad_example": "return err",
      "rule": "当跨层返回错误时，应该使用 %w 包装上下文",
      "confidence": 0.91,
      "merged_from": ["p1", "p2"],
      "merge_reason": "规则和场景一致",
      "similarity_score": 0.87,
      "business_method": {
        "name": "UserService.Create(ctx, req) error",
        "location": "internal/service/user.go:42",
        "description": "创建用户并包装仓储错误",
        "usage": "用户创建流程",
        "type": "domain",
        "function": "func (s *UserService) Create(ctx context.Context, req CreateUserRequest) error",
        "prerequisites": "UserRepository 已初始化",
        "returns": "成功返回 nil，失败返回包装错误"
      }
    }
  ],
  "unchanged_patterns": [],
  "summary": {
    "total_input": 2,
    "total_merged": 1,
    "total_unchanged": 0,
    "merge_count": 1
  }
}`

	result, err := ParseMergePatternsResult(output)

	require.NoError(t, err)
	require.Len(t, result.MergedPatterns, 1)
	merged := result.MergedPatterns[0]
	require.Equal(t, `return fmt.Errorf("create user: %w", err)`, merged.GoodExample)
	require.Equal(t, "return err", merged.BadExample)
	require.Equal(t, 0.87, merged.SimilarityScore)
	require.NotNil(t, merged.BusinessMethod)
	require.Equal(t, "internal/service/user.go:42", merged.BusinessMethod.Location)
	require.Equal(t, "UserRepository 已初始化", merged.BusinessMethod.Prerequisites)
}

func TestParseGenerateSkillsResultPreservesInsightsAndSuggestions(t *testing.T) {
	output := `{
  "category_summaries": {},
  "key_patterns": [],
  "business_rules": ["规则"],
  "best_practices": ["实践"],
  "common_patterns": ["模式"],
  "key_insights": ["错误处理是跨层一致性核心"],
  "improvement_suggestions": ["为外部调用补充超时测试"]
}`

	result, err := ParseGenerateSkillsResult(output)

	require.NoError(t, err)
	require.Equal(t, []string{"错误处理是跨层一致性核心"}, result.KeyInsights)
	require.Equal(t, []string{"为外部调用补充超时测试"}, result.ImprovementSuggestions)
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
