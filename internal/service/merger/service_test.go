package merger

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
)

func TestMergePatterns_NoPatterns(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}
	svc := NewMergerService(mockAgent, mockPattern)
	result, err := svc.MergePatterns(context.Background(), &MergePatternsRequest{})
	assert.NoError(t, err)
	assert.Equal(t, 0, result.Summary.TotalInput)
}

func TestMergePatterns_RepoError(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewMergerService(mockAgent, mockPattern)
	_, err := svc.MergePatterns(context.Background(), &MergePatternsRequest{})
	assert.Error(t, err)
}

func TestMergePatterns_DryRun(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "Error Wrap", domain.CategoryError),
		*domain.NewPattern("p2", "Error Handling", domain.CategoryError),
	}
	deleted := []string{}
	saved := []*domain.Pattern{}

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		MergePatternsFn: func(ctx context.Context, req *agent.MergePatternsRequest) (*agent.MergePatternsResult, error) {
			return &agent.MergePatternsResult{
				MergedPatterns: []agent.MergedPattern{
					{
						ID: "merged-1", Name: "Error Patterns", Category: "error",
						MergedFrom: []string{"p1", "p2"}, Confidence: 0.9,
					},
				},
				UnchangedPatterns: []agent.UnchangedPattern{},
				Summary: agent.MergeSummary{
					TotalInput: 2, TotalMerged: 1, MergeCount: 1,
				},
			}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return patterns, nil
		},
		DeleteFn: func(ctx context.Context, id string) error {
			deleted = append(deleted, id)
			return nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}

	svc := NewMergerService(mockAgent, mockPattern)
	result, err := svc.MergePatterns(context.Background(), &MergePatternsRequest{DryRun: true})
	assert.NoError(t, err)
	assert.Len(t, result.MergedPatterns, 1)
	// dry run 不应删除或保存数据。
	assert.Empty(t, deleted)
	assert.Empty(t, saved)
}

func TestMergePatterns_ActualMerge(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "Error Wrap", domain.CategoryError),
		*domain.NewPattern("p2", "Error Handling", domain.CategoryError),
	}
	deleted := []string{}
	saved := []*domain.Pattern{}

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		MergePatternsFn: func(ctx context.Context, req *agent.MergePatternsRequest) (*agent.MergePatternsResult, error) {
			return &agent.MergePatternsResult{
				MergedPatterns: []agent.MergedPattern{
					{
						ID: "merged-1", Name: "Error Patterns", Category: "error",
						Description: "Merged error patterns", Rule: "Always wrap errors",
						GoodExample: "return fmt.Errorf(\"create user: %w\", err)",
						BadExample:  "return err",
						MergedFrom:  []string{"p1", "p2"}, Confidence: 0.9,
						BusinessMethod: &domain.BusinessMethod{
							Name:          "UserService.Create(ctx, req) error",
							CodeLocation:  domain.CodeLocation{CurrentLocation: "internal/service/user.go:42"},
							Description:   "创建用户并包装仓储错误",
							Usage:         "用户创建流程",
							Type:          "domain",
							Function:      "func (s *UserService) Create(ctx context.Context, req CreateUserRequest) error",
							Prerequisites: "UserRepository 已初始化",
							Returns:       "成功返回 nil，失败返回包装错误",
						},
					},
				},
				UnchangedPatterns: []agent.UnchangedPattern{},
				Summary: agent.MergeSummary{
					TotalInput: 2, TotalMerged: 1, MergeCount: 1,
				},
			}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return patterns, nil
		},
		DeleteFn: func(ctx context.Context, id string) error {
			deleted = append(deleted, id)
			return nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}

	svc := NewMergerService(mockAgent, mockPattern)
	result, err := svc.MergePatterns(context.Background(), &MergePatternsRequest{})
	assert.NoError(t, err)
	assert.Len(t, result.MergedPatterns, 1)
	assert.Len(t, deleted, 2)
	assert.Len(t, saved, 1)
	assert.Equal(t, "merged-1", saved[0].ID)
	assert.True(t, saved[0].Merged)
	assert.Equal(t, "return fmt.Errorf(\"create user: %w\", err)", saved[0].GoodExample)
	assert.Equal(t, "return err", saved[0].BadExample)
	assert.NotNil(t, saved[0].BusinessMethod)
	assert.Equal(t, "internal/service/user.go:42", saved[0].BusinessMethod.DisplayLocation())
}

func TestMergePatterns_AIError(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "Test", domain.CategoryNaming),
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		MergePatternsFn: func(ctx context.Context, req *agent.MergePatternsRequest) (*agent.MergePatternsResult, error) {
			return nil, errors.New("AI error")
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return patterns, nil
		},
	}

	svc := NewMergerService(mockAgent, mockPattern)
	_, err := svc.MergePatterns(context.Background(), &MergePatternsRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI")
}

func TestMergePatterns_WithCategory(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	called := false
	mockPattern := &mocks.MockPatternRepository{
		GetByCategoryFn: func(ctx context.Context, cat domain.Category) ([]domain.Pattern, error) {
			called = true
			assert.Equal(t, domain.CategoryError, cat)
			return []domain.Pattern{}, nil
		},
	}

	svc := NewMergerService(mockAgent, mockPattern)
	_, err := svc.MergePatterns(context.Background(), &MergePatternsRequest{Category: "error"})
	assert.NoError(t, err)
	assert.True(t, called)
}
