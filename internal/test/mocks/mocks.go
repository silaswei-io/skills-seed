package mocks

import (
	"context"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

// MockAgent 模拟 AI Agent
type MockAgent struct {
	NameVal                  string
	AvailableVal             bool
	AnalyzeCodeFn            func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error)
	LearnFromCommitFn        func(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error)
	BatchLearnFromCommitsFn  func(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error)
	GenerateFixesFn          func(ctx context.Context, req *agent.GenerateFixesRequest) (*agent.GenerateFixesResult, error)
	GenerateSkillsSummaryFn  func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error)
	MergePatternsFn          func(ctx context.Context, req *agent.MergePatternsRequest) (*agent.MergePatternsResult, error)
	AnalyzeProjectFn         func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error)
	AnalyzeCurrentCodebaseFn func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error)
}

func (m *MockAgent) Name() string      { return m.NameVal }
func (m *MockAgent) IsAvailable() bool { return m.AvailableVal }

func (m *MockAgent) AnalyzeCode(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
	if m.AnalyzeCodeFn != nil {
		return m.AnalyzeCodeFn(ctx, req)
	}
	return &agent.AnalyzeResult{Issues: []domain.Issue{}, Confidence: 0.5}, nil
}

func (m *MockAgent) LearnFromCommit(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
	if m.LearnFromCommitFn != nil {
		return m.LearnFromCommitFn(ctx, req)
	}
	return &agent.LearnResult{Patterns: []domain.Pattern{}, LearnedAt: time.Now()}, nil
}

func (m *MockAgent) BatchLearnFromCommits(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
	if m.BatchLearnFromCommitsFn != nil {
		return m.BatchLearnFromCommitsFn(ctx, req)
	}
	return &agent.BatchLearnResult{Patterns: []domain.Pattern{}, LearnedAt: time.Now()}, nil
}

func (m *MockAgent) GenerateFixes(ctx context.Context, req *agent.GenerateFixesRequest) (*agent.GenerateFixesResult, error) {
	if m.GenerateFixesFn != nil {
		return m.GenerateFixesFn(ctx, req)
	}
	return &agent.GenerateFixesResult{Fixes: map[string]string{}, Confidence: 0.8}, nil
}

func (m *MockAgent) GenerateSkillsSummary(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
	if m.GenerateSkillsSummaryFn != nil {
		return m.GenerateSkillsSummaryFn(ctx, req)
	}
	return &agent.GenerateSkillsResult{
		CategorySummaries: map[string]agent.CategorySummary{},
		KeyPatterns:       []agent.PatternSummary{},
		BusinessRules:     []string{},
		BestPractices:     []string{},
		CommonPatterns:    []string{},
	}, nil
}

func (m *MockAgent) MergePatterns(ctx context.Context, req *agent.MergePatternsRequest) (*agent.MergePatternsResult, error) {
	if m.MergePatternsFn != nil {
		return m.MergePatternsFn(ctx, req)
	}
	return &agent.MergePatternsResult{
		MergedPatterns:    []agent.MergedPattern{},
		UnchangedPatterns: []agent.UnchangedPattern{},
		Summary:           agent.MergeSummary{},
	}, nil
}

func (m *MockAgent) AnalyzeProject(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	if m.AnalyzeProjectFn != nil {
		return m.AnalyzeProjectFn(ctx, req)
	}
	return &agent.AnalyzeProjectResult{}, nil
}

func (m *MockAgent) AnalyzeCurrentCodebase(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
	if m.AnalyzeCurrentCodebaseFn != nil {
		return m.AnalyzeCurrentCodebaseFn(ctx, req)
	}
	return &agent.AnalyzeCurrentCodebaseResult{}, nil
}

// MockGitRepository 模拟 Git 仓储
type MockGitRepository struct {
	CommitsFn       func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error)
	ChangedFilesFn  func(ctx context.Context, hash string) ([]string, error)
	StagedFilesFn   func(ctx context.Context) ([]domain.FileInfo, error)
	AllFilesFn      func(ctx context.Context) ([]domain.FileInfo, error)
	CurrentBranchFn func(ctx context.Context) (string, error)
	ProjectRootFn   func(ctx context.Context) (string, error)
	StashFn         func(ctx context.Context, message string) error
	CreateBranchFn  func(ctx context.Context, name string) error
	CheckoutFn      func(ctx context.Context, name string) error
}

func (m *MockGitRepository) GetCommits(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
	if m.CommitsFn != nil {
		return m.CommitsFn(ctx, limit, since)
	}
	return []domain.CommitInfo{}, nil
}

func (m *MockGitRepository) GetChangedFiles(ctx context.Context, hash string) ([]string, error) {
	if m.ChangedFilesFn != nil {
		return m.ChangedFilesFn(ctx, hash)
	}
	return []string{}, nil
}

func (m *MockGitRepository) GetStagedFiles(ctx context.Context) ([]domain.FileInfo, error) {
	if m.StagedFilesFn != nil {
		return m.StagedFilesFn(ctx)
	}
	return []domain.FileInfo{}, nil
}

func (m *MockGitRepository) GetAllFiles(ctx context.Context) ([]domain.FileInfo, error) {
	if m.AllFilesFn != nil {
		return m.AllFilesFn(ctx)
	}
	return []domain.FileInfo{}, nil
}

func (m *MockGitRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.CurrentBranchFn != nil {
		return m.CurrentBranchFn(ctx)
	}
	return "main", nil
}

func (m *MockGitRepository) GetProjectRoot(ctx context.Context) (string, error) {
	if m.ProjectRootFn != nil {
		return m.ProjectRootFn(ctx)
	}
	return "/tmp/project", nil
}

func (m *MockGitRepository) Stash(ctx context.Context, message string) error {
	if m.StashFn != nil {
		return m.StashFn(ctx, message)
	}
	return nil
}

func (m *MockGitRepository) CreateBranch(ctx context.Context, name string) error {
	if m.CreateBranchFn != nil {
		return m.CreateBranchFn(ctx, name)
	}
	return nil
}

func (m *MockGitRepository) Checkout(ctx context.Context, name string) error {
	if m.CheckoutFn != nil {
		return m.CheckoutFn(ctx, name)
	}
	return nil
}

// MockPatternRepository 模拟模式仓储
type MockPatternRepository struct {
	GetFn               func(ctx context.Context, id string) (*domain.Pattern, error)
	GetAllFn            func(ctx context.Context) ([]domain.Pattern, error)
	GetByCategoryFn     func(ctx context.Context, category domain.Category) ([]domain.Pattern, error)
	GetHighConfidenceFn func(ctx context.Context, threshold float64) ([]domain.Pattern, error)
	SaveFn              func(ctx context.Context, p *domain.Pattern) error
	FindSimilarFn       func(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error)
	DeleteFn            func(ctx context.Context, id string) error
	CountFn             func(ctx context.Context) (int, error)
}

func (m *MockPatternRepository) Get(ctx context.Context, id string) (*domain.Pattern, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, id)
	}
	return nil, nil
}

func (m *MockPatternRepository) GetAll(ctx context.Context) ([]domain.Pattern, error) {
	if m.GetAllFn != nil {
		return m.GetAllFn(ctx)
	}
	return []domain.Pattern{}, nil
}

func (m *MockPatternRepository) GetByCategory(ctx context.Context, category domain.Category) ([]domain.Pattern, error) {
	if m.GetByCategoryFn != nil {
		return m.GetByCategoryFn(ctx, category)
	}
	return []domain.Pattern{}, nil
}

func (m *MockPatternRepository) GetHighConfidence(ctx context.Context, threshold float64) ([]domain.Pattern, error) {
	if m.GetHighConfidenceFn != nil {
		return m.GetHighConfidenceFn(ctx, threshold)
	}
	return []domain.Pattern{}, nil
}

func (m *MockPatternRepository) Save(ctx context.Context, p *domain.Pattern) error {
	if m.SaveFn != nil {
		return m.SaveFn(ctx, p)
	}
	return nil
}

func (m *MockPatternRepository) FindSimilar(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error) {
	if m.FindSimilarFn != nil {
		return m.FindSimilarFn(ctx, pattern)
	}
	return nil, nil
}

func (m *MockPatternRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

func (m *MockPatternRepository) Count(ctx context.Context) (int, error) {
	if m.CountFn != nil {
		return m.CountFn(ctx)
	}
	return 0, nil
}

// MockProjectProfileRepository 模拟项目画像仓储
type MockProjectProfileRepository struct {
	GetFn  func(ctx context.Context) (*domain.ProjectProfile, error)
	SaveFn func(ctx context.Context, profile *domain.ProjectProfile) error
}

func (m *MockProjectProfileRepository) Get(ctx context.Context) (*domain.ProjectProfile, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx)
	}
	return &domain.ProjectProfile{
		ProjectName: "test",
		Language:    "go",
		Summary:     "Test project",
		GeneratedAt: "2026-05-19 00:00:00",
	}, nil
}

func (m *MockProjectProfileRepository) Save(ctx context.Context, profile *domain.ProjectProfile) error {
	if m.SaveFn != nil {
		return m.SaveFn(ctx, profile)
	}
	return nil
}

// MockCommitTracker 模拟提交追踪
type MockCommitTracker struct {
	MarkAnalyzedFn func(ctx context.Context, hash string) error
	IsAnalyzedFn   func(ctx context.Context, hash string) (bool, error)
	GetAnalyzedFn  func(ctx context.Context) ([]string, error)
}

func (m *MockCommitTracker) MarkCommitAnalyzed(ctx context.Context, hash string) error {
	if m.MarkAnalyzedFn != nil {
		return m.MarkAnalyzedFn(ctx, hash)
	}
	return nil
}

func (m *MockCommitTracker) IsCommitAnalyzed(ctx context.Context, hash string) (bool, error) {
	if m.IsAnalyzedFn != nil {
		return m.IsAnalyzedFn(ctx, hash)
	}
	return false, nil
}

func (m *MockCommitTracker) GetAnalyzedCommits(ctx context.Context) ([]string, error) {
	if m.GetAnalyzedFn != nil {
		return m.GetAnalyzedFn(ctx)
	}
	return []string{}, nil
}

// MockConfigReader 模拟配置读取
type MockConfigReader struct {
	ProjectCfg  config.ProjectConfig
	AgentCfg    config.AgentConfig
	LearningCfg config.LearningConfig
	AutoFixCfg  config.AutoFixConfig
	OutputCfg   config.OutputConfig
	LoggingCfg  config.LoggingConfig
	Exclude     []string
}

func (m *MockConfigReader) GetProjectConfig() config.ProjectConfig   { return m.ProjectCfg }
func (m *MockConfigReader) GetAgentConfig() config.AgentConfig       { return m.AgentCfg }
func (m *MockConfigReader) GetLearningConfig() config.LearningConfig { return m.LearningCfg }
func (m *MockConfigReader) GetAutoFixConfig() config.AutoFixConfig   { return m.AutoFixCfg }
func (m *MockConfigReader) GetOutputConfig() config.OutputConfig     { return m.OutputCfg }
func (m *MockConfigReader) GetLoggingConfig() config.LoggingConfig   { return m.LoggingCfg }
func (m *MockConfigReader) GetExclude() []string                     { return m.Exclude }
