package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCuratePatternsResultPreservesCanonicalPatternFields(t *testing.T) {
	output := `{
  "patterns": [
    {
      "id": "existing-error-handling",
      "name": "错误处理模式",
      "category": "error",
      "description": "统一包装错误并保留调用上下文",
      "good_example": "return fmt.Errorf(\"create user: %w\", err)",
      "bad_example": "return err",
      "rule": "当跨层返回错误时，应该使用 %w 包装上下文",
      "confidence": 0.91,
      "frequency": 4,
      "evidence_locations": [
        {
          "path": "internal/service/user.go",
          "line": 42,
          "symbol": "Create",
          "kind": "method",
          "description": "包装仓储错误",
          "confidence": 0.88
        }
      ],
      "merged_from": ["existing-error-handling", "candidate-error-handling"],
      "merge_reason": "规则和场景一致",
      "similarity_score": 0.87,
      "source": "learned_current",
      "business_method": {
        "name": "UserService.Create(ctx, req) error",
        "code_location": {"current_location":"internal/service/user.go:42"},
        "description": "创建用户并包装仓储错误",
        "usage": "用户创建流程",
        "type": "domain",
        "function": "func (s *UserService) Create(ctx context.Context, req CreateUserRequest) error",
        "prerequisites": "UserRepository 已初始化",
        "returns": "成功返回 nil，失败返回包装错误"
      },
      "project_id": "backend",
      "scope_path": "backend",
      "workspace_role": "backend"
    }
  ],
  "dropped": [{"id":"candidate-generic","reason":"规则过于泛化"}],
  "summary": {
    "total_candidates": 2,
    "total_existing": 1,
    "total_written": 1,
    "total_dropped": 1,
    "merge_count": 1
  }
}`

	result, err := ParseCuratePatternsResult(output)

	require.NoError(t, err)
	require.Len(t, result.Patterns, 1)
	pattern := result.Patterns[0]
	require.Equal(t, "existing-error-handling", pattern.ID)
	require.Equal(t, 4, pattern.Frequency)
	require.Equal(t, "learned_current", pattern.Source)
	require.Len(t, pattern.EvidenceLocations, 1)
	require.Equal(t, "internal/service/user.go:42", pattern.EvidenceLocations[0].DisplayLocation())
	require.Equal(t, []string{"existing-error-handling", "candidate-error-handling"}, pattern.MergedFrom)
	require.Equal(t, "backend", pattern.ProjectID)
	require.NotNil(t, pattern.BusinessMethod)
	require.Equal(t, "internal/service/user.go:42", pattern.BusinessMethod.DisplayLocation())
	require.Len(t, result.Dropped, 1)
	require.Equal(t, 2, result.Summary.TotalCandidates)
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
