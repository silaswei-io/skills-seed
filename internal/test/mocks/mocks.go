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
	NameVal                   string
	AvailableVal              bool
	AnalyzeCodeFn             func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error)
	LearnFromCommitFn         func(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error)
	BatchLearnFromCommitsFn   func(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error)
	GenerateFixesFn           func(ctx context.Context, req *agent.GenerateFixesRequest) (*agent.GenerateFixesResult, error)
	GenerateSkillsSummaryFn   func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error)
	MergePatternsFn           func(ctx context.Context, req *agent.MergePatternsRequest) (*agent.MergePatternsResult, error)
	AnalyzeProjectFn          func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error)
	AnalyzeCurrentCodebaseFn  func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error)
	AnalyzeWorkspaceProfileFn func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error)
	AnalyzeWorkspaceSpecFn    func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error)
}

// Name 返回模拟 Agent 名称
func (m *MockAgent) Name() string { return m.NameVal }

// IsAvailable 返回模拟 Agent 是否可用
func (m *MockAgent) IsAvailable() bool { return m.AvailableVal }

// AnalyzeCode 模拟代码分析
func (m *MockAgent) AnalyzeCode(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
	if m.AnalyzeCodeFn != nil {
		return m.AnalyzeCodeFn(ctx, req)
	}
	return &agent.AnalyzeResult{Issues: []domain.Issue{}, Confidence: 0.5}, nil
}

// LearnFromCommit 模拟单提交学习
func (m *MockAgent) LearnFromCommit(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
	if m.LearnFromCommitFn != nil {
		return m.LearnFromCommitFn(ctx, req)
	}
	return &agent.LearnResult{Patterns: []domain.Pattern{}, LearnedAt: time.Now()}, nil
}

// BatchLearnFromCommits 模拟批量提交学习
func (m *MockAgent) BatchLearnFromCommits(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
	if m.BatchLearnFromCommitsFn != nil {
		return m.BatchLearnFromCommitsFn(ctx, req)
	}
	return &agent.BatchLearnResult{Patterns: []domain.Pattern{}, LearnedAt: time.Now()}, nil
}

// GenerateFixes 模拟生成修复
func (m *MockAgent) GenerateFixes(ctx context.Context, req *agent.GenerateFixesRequest) (*agent.GenerateFixesResult, error) {
	if m.GenerateFixesFn != nil {
		return m.GenerateFixesFn(ctx, req)
	}
	return &agent.GenerateFixesResult{Fixes: map[string]string{}, Confidence: 0.8}, nil
}

// GenerateSkillsSummary 模拟生成 Skills 摘要
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

// MergePatterns 模拟模式合并
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

// AnalyzeProject 模拟项目分析
func (m *MockAgent) AnalyzeProject(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	if m.AnalyzeProjectFn != nil {
		return m.AnalyzeProjectFn(ctx, req)
	}
	return &agent.AnalyzeProjectResult{}, nil
}

// AnalyzeCurrentCodebase 模拟当前代码库分析
func (m *MockAgent) AnalyzeCurrentCodebase(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
	if m.AnalyzeCurrentCodebaseFn != nil {
		return m.AnalyzeCurrentCodebaseFn(ctx, req)
	}
	return &agent.AnalyzeCurrentCodebaseResult{}, nil
}

// AnalyzeWorkspaceProfile 模拟工作区画像分析
func (m *MockAgent) AnalyzeWorkspaceProfile(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
	if m.AnalyzeWorkspaceProfileFn != nil {
		return m.AnalyzeWorkspaceProfileFn(ctx, req)
	}
	return &domain.WorkspaceProfile{}, nil
}

// AnalyzeWorkspaceSpec 模拟工作区规范分析
func (m *MockAgent) AnalyzeWorkspaceSpec(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
	if m.AnalyzeWorkspaceSpecFn != nil {
		return m.AnalyzeWorkspaceSpecFn(ctx, req)
	}
	return &domain.WorkspaceSpec{}, nil
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

// GetCommits 模拟获取提交历史
func (m *MockGitRepository) GetCommits(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
	if m.CommitsFn != nil {
		return m.CommitsFn(ctx, limit, since)
	}
	return []domain.CommitInfo{}, nil
}

// GetChangedFiles 模拟获取变更文件
func (m *MockGitRepository) GetChangedFiles(ctx context.Context, hash string) ([]string, error) {
	if m.ChangedFilesFn != nil {
		return m.ChangedFilesFn(ctx, hash)
	}
	return []string{}, nil
}

// GetStagedFiles 模拟获取暂存文件
func (m *MockGitRepository) GetStagedFiles(ctx context.Context) ([]domain.FileInfo, error) {
	if m.StagedFilesFn != nil {
		return m.StagedFilesFn(ctx)
	}
	return []domain.FileInfo{}, nil
}

// GetAllFiles 模拟获取所有文件
func (m *MockGitRepository) GetAllFiles(ctx context.Context) ([]domain.FileInfo, error) {
	if m.AllFilesFn != nil {
		return m.AllFilesFn(ctx)
	}
	return []domain.FileInfo{}, nil
}

// GetCurrentBranch 模拟获取当前分支
func (m *MockGitRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.CurrentBranchFn != nil {
		return m.CurrentBranchFn(ctx)
	}
	return "main", nil
}

// GetProjectRoot 模拟获取项目根目录
func (m *MockGitRepository) GetProjectRoot(ctx context.Context) (string, error) {
	if m.ProjectRootFn != nil {
		return m.ProjectRootFn(ctx)
	}
	return "/tmp/project", nil
}

// Stash 模拟保存 stash
func (m *MockGitRepository) Stash(ctx context.Context, message string) error {
	if m.StashFn != nil {
		return m.StashFn(ctx, message)
	}
	return nil
}

// CreateBranch 模拟创建分支
func (m *MockGitRepository) CreateBranch(ctx context.Context, name string) error {
	if m.CreateBranchFn != nil {
		return m.CreateBranchFn(ctx, name)
	}
	return nil
}

// Checkout 模拟切换分支
func (m *MockGitRepository) Checkout(ctx context.Context, name string) error {
	if m.CheckoutFn != nil {
		return m.CheckoutFn(ctx, name)
	}
	return nil
}

// MockPatternRepository 模拟模式仓储
type MockPatternRepository struct {
	GetFn                func(ctx context.Context, id string) (*domain.Pattern, error)
	GetAllFn             func(ctx context.Context) ([]domain.Pattern, error)
	GetByCategoryFn      func(ctx context.Context, category domain.Category) ([]domain.Pattern, error)
	GetHighConfidenceFn  func(ctx context.Context, threshold float64) ([]domain.Pattern, error)
	SaveFn               func(ctx context.Context, p *domain.Pattern) error
	FindSimilarFn        func(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error)
	DeleteFn             func(ctx context.Context, id string) error
	CountFn              func(ctx context.Context) (int, error)
	RecordPatternHitsFn  func(ctx context.Context, hits []domain.PatternHit) error
	GetPatternHitStatsFn func(ctx context.Context) ([]domain.PatternHitStats, error)
}

// Get 模拟按 ID 获取模式
func (m *MockPatternRepository) Get(ctx context.Context, id string) (*domain.Pattern, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, id)
	}
	return nil, nil
}

// GetAll 模拟获取全部模式
func (m *MockPatternRepository) GetAll(ctx context.Context) ([]domain.Pattern, error) {
	if m.GetAllFn != nil {
		return m.GetAllFn(ctx)
	}
	return []domain.Pattern{}, nil
}

// GetByCategory 模拟按分类获取模式
func (m *MockPatternRepository) GetByCategory(ctx context.Context, category domain.Category) ([]domain.Pattern, error) {
	if m.GetByCategoryFn != nil {
		return m.GetByCategoryFn(ctx, category)
	}
	return []domain.Pattern{}, nil
}

// GetHighConfidence 模拟获取高置信度模式
func (m *MockPatternRepository) GetHighConfidence(ctx context.Context, threshold float64) ([]domain.Pattern, error) {
	if m.GetHighConfidenceFn != nil {
		return m.GetHighConfidenceFn(ctx, threshold)
	}
	return []domain.Pattern{}, nil
}

// Save 模拟保存模式
func (m *MockPatternRepository) Save(ctx context.Context, p *domain.Pattern) error {
	if m.SaveFn != nil {
		return m.SaveFn(ctx, p)
	}
	return nil
}

// FindSimilar 模拟查找相似模式
func (m *MockPatternRepository) FindSimilar(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error) {
	if m.FindSimilarFn != nil {
		return m.FindSimilarFn(ctx, pattern)
	}
	return nil, nil
}

// Delete 模拟删除模式
func (m *MockPatternRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

// Count 模拟统计模式数量
func (m *MockPatternRepository) Count(ctx context.Context) (int, error) {
	if m.CountFn != nil {
		return m.CountFn(ctx)
	}
	return 0, nil
}

// RecordPatternHits 模拟保存模式命中记录。
func (m *MockPatternRepository) RecordPatternHits(ctx context.Context, hits []domain.PatternHit) error {
	if m.RecordPatternHitsFn != nil {
		return m.RecordPatternHitsFn(ctx, hits)
	}
	return nil
}

// GetPatternHitStats 模拟获取模式命中统计。
func (m *MockPatternRepository) GetPatternHitStats(ctx context.Context) ([]domain.PatternHitStats, error) {
	if m.GetPatternHitStatsFn != nil {
		return m.GetPatternHitStatsFn(ctx)
	}
	return []domain.PatternHitStats{}, nil
}

// MockPatternStatsRepository 模拟模式统计仓储。
type MockPatternStatsRepository struct {
	GetPatternHitStatsFn func(ctx context.Context) ([]domain.PatternHitStats, error)
}

func (m *MockPatternStatsRepository) GetPatternHitStats(ctx context.Context) ([]domain.PatternHitStats, error) {
	if m.GetPatternHitStatsFn != nil {
		return m.GetPatternHitStatsFn(ctx)
	}
	return []domain.PatternHitStats{}, nil
}

// MockReviewRepository 模拟评审评论仓储。
type MockReviewRepository struct {
	ImportReviewCommentsFn func(ctx context.Context, comments []domain.ReviewComment) error
	GetReviewStatsFn       func(ctx context.Context, lineWindow int) (domain.ReviewStats, error)
}

func (m *MockReviewRepository) ImportReviewComments(ctx context.Context, comments []domain.ReviewComment) error {
	if m.ImportReviewCommentsFn != nil {
		return m.ImportReviewCommentsFn(ctx, comments)
	}
	return nil
}

func (m *MockReviewRepository) GetReviewStats(ctx context.Context, lineWindow int) (domain.ReviewStats, error) {
	if m.GetReviewStatsFn != nil {
		return m.GetReviewStatsFn(ctx, lineWindow)
	}
	return domain.ReviewStats{}, nil
}

// MockProjectProfileRepository 模拟项目画像仓储
type MockProjectProfileRepository struct {
	GetFn                func(ctx context.Context) (*domain.ProjectProfile, error)
	SaveFn               func(ctx context.Context, profile *domain.ProjectProfile) error
	GetForProjectFn      func(ctx context.Context, projectID string) (*domain.ProjectProfile, error)
	SaveForProjectFn     func(ctx context.Context, projectID string, profile *domain.ProjectProfile) error
	GetSpecFn            func(ctx context.Context) (*domain.ProjectSpec, error)
	SaveSpecFn           func(ctx context.Context, spec *domain.ProjectSpec) error
	GetSpecForProjectFn  func(ctx context.Context, projectID string) (*domain.ProjectSpec, error)
	SaveSpecForProjectFn func(ctx context.Context, projectID string, spec *domain.ProjectSpec) error
}

// Get 模拟获取项目画像
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

// Save 模拟保存项目画像
func (m *MockProjectProfileRepository) Save(ctx context.Context, profile *domain.ProjectProfile) error {
	if m.SaveFn != nil {
		return m.SaveFn(ctx, profile)
	}
	return nil
}

// GetForProject 模拟获取 workspace 子项目画像
func (m *MockProjectProfileRepository) GetForProject(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
	if m.GetForProjectFn != nil {
		return m.GetForProjectFn(ctx, projectID)
	}
	return m.Get(ctx)
}

// SaveForProject 模拟保存 workspace 子项目画像
func (m *MockProjectProfileRepository) SaveForProject(ctx context.Context, projectID string, profile *domain.ProjectProfile) error {
	if m.SaveForProjectFn != nil {
		return m.SaveForProjectFn(ctx, projectID, profile)
	}
	return m.Save(ctx, profile)
}

// GetSpec 模拟获取项目规范
func (m *MockProjectProfileRepository) GetSpec(ctx context.Context) (*domain.ProjectSpec, error) {
	if m.GetSpecFn != nil {
		return m.GetSpecFn(ctx)
	}
	return nil, nil
}

// SaveSpec 模拟保存项目规范
func (m *MockProjectProfileRepository) SaveSpec(ctx context.Context, spec *domain.ProjectSpec) error {
	if m.SaveSpecFn != nil {
		return m.SaveSpecFn(ctx, spec)
	}
	return nil
}

// GetSpecForProject 模拟获取 workspace 子项目规范
func (m *MockProjectProfileRepository) GetSpecForProject(ctx context.Context, projectID string) (*domain.ProjectSpec, error) {
	if m.GetSpecForProjectFn != nil {
		return m.GetSpecForProjectFn(ctx, projectID)
	}
	return nil, nil
}

// SaveSpecForProject 模拟保存 workspace 子项目规范
func (m *MockProjectProfileRepository) SaveSpecForProject(ctx context.Context, projectID string, spec *domain.ProjectSpec) error {
	if m.SaveSpecForProjectFn != nil {
		return m.SaveSpecForProjectFn(ctx, projectID, spec)
	}
	return nil
}

// MockCommitTracker 模拟提交追踪
type MockCommitTracker struct {
	MarkAnalyzedFn func(ctx context.Context, hash string) error
	IsAnalyzedFn   func(ctx context.Context, hash string) (bool, error)
	GetAnalyzedFn  func(ctx context.Context) ([]string, error)
}

// MarkCommitAnalyzed 模拟标记提交已分析
func (m *MockCommitTracker) MarkCommitAnalyzed(ctx context.Context, hash string) error {
	if m.MarkAnalyzedFn != nil {
		return m.MarkAnalyzedFn(ctx, hash)
	}
	return nil
}

// IsCommitAnalyzed 模拟判断提交是否已分析
func (m *MockCommitTracker) IsCommitAnalyzed(ctx context.Context, hash string) (bool, error) {
	if m.IsAnalyzedFn != nil {
		return m.IsAnalyzedFn(ctx, hash)
	}
	return false, nil
}

// GetAnalyzedCommits 模拟获取已分析提交
func (m *MockCommitTracker) GetAnalyzedCommits(ctx context.Context) ([]string, error) {
	if m.GetAnalyzedFn != nil {
		return m.GetAnalyzedFn(ctx)
	}
	return []string{}, nil
}

// MockFileAnalysisTracker 模拟文件分析追踪器
type MockFileAnalysisTracker struct {
	GetAnalyzedFileFn     func(ctx context.Context, scope domain.FileAnalysisScope, path string) (*domain.FileAnalysisRecord, error)
	ListAnalyzedFilesFn   func(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error)
	SaveAnalyzedFilesFn   func(ctx context.Context, records []domain.FileAnalysisRecord) error
	DeleteAnalyzedFilesFn func(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error
}

func (m *MockFileAnalysisTracker) GetAnalyzedFile(ctx context.Context, scope domain.FileAnalysisScope, path string) (*domain.FileAnalysisRecord, error) {
	if m.GetAnalyzedFileFn != nil {
		return m.GetAnalyzedFileFn(ctx, scope, path)
	}
	return nil, nil
}

func (m *MockFileAnalysisTracker) ListAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
	if m.ListAnalyzedFilesFn != nil {
		return m.ListAnalyzedFilesFn(ctx, scope)
	}
	return []domain.FileAnalysisRecord{}, nil
}

func (m *MockFileAnalysisTracker) SaveAnalyzedFiles(ctx context.Context, records []domain.FileAnalysisRecord) error {
	if m.SaveAnalyzedFilesFn != nil {
		return m.SaveAnalyzedFilesFn(ctx, records)
	}
	return nil
}

func (m *MockFileAnalysisTracker) DeleteAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
	if m.DeleteAnalyzedFilesFn != nil {
		return m.DeleteAnalyzedFilesFn(ctx, scope, paths)
	}
	return nil
}

// MockConfigReader 模拟配置读取
type MockConfigReader struct {
	ProjectCfg   config.ProjectConfig
	WorkspaceCfg config.WorkspaceConfig
	AnalysisCfg  config.AnalysisConfig
	AgentCfg     config.AgentConfig
	LearningCfg  config.LearningConfig
	AutoFixCfg   config.AutoFixConfig
	OutputCfg    config.OutputConfig
	LoggingCfg   config.LoggingConfig
	Exclude      []string
}

// GetProjectConfig 模拟获取项目配置
func (m *MockConfigReader) GetProjectConfig() config.ProjectConfig { return m.ProjectCfg }

// GetWorkspaceConfig 模拟获取工作区配置
func (m *MockConfigReader) GetWorkspaceConfig() config.WorkspaceConfig { return m.WorkspaceCfg }

// GetAnalysisConfig 模拟获取分析增强配置
func (m *MockConfigReader) GetAnalysisConfig() config.AnalysisConfig { return m.AnalysisCfg }

// GetAgentConfig 模拟获取 Agent 配置
func (m *MockConfigReader) GetAgentConfig() config.AgentConfig { return m.AgentCfg }

// GetLearningConfig 模拟获取学习配置
func (m *MockConfigReader) GetLearningConfig() config.LearningConfig { return m.LearningCfg }

// GetAutoFixConfig 模拟获取自动修复配置
func (m *MockConfigReader) GetAutoFixConfig() config.AutoFixConfig { return m.AutoFixCfg }

// GetOutputConfig 模拟获取输出配置
func (m *MockConfigReader) GetOutputConfig() config.OutputConfig { return m.OutputCfg }

// GetLoggingConfig 模拟获取日志配置
func (m *MockConfigReader) GetLoggingConfig() config.LoggingConfig { return m.LoggingCfg }

// GetExclude 模拟获取排除配置
func (m *MockConfigReader) GetExclude() []string { return m.Exclude }
