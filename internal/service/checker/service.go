package checker

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/utils/filefilter"
)

// CheckerService 代码检查服务
type CheckerService struct {
	agent       agent.Agent
	gitRepo     domain.GitRepository
	patternRepo domain.PatternRepository
	configRepo  config.Reader
}

// NewCheckerService 创建代码检查服务
func NewCheckerService(
	ag agent.Agent,
	gitRepo domain.GitRepository,
	patternRepo domain.PatternRepository,
	configRepo config.Reader,
) *CheckerService {
	return &CheckerService{
		agent:       ag,
		gitRepo:     gitRepo,
		patternRepo: patternRepo,
		configRepo:  configRepo,
	}
}

// Check 检查暂存文件
func (s *CheckerService) Check(ctx context.Context) ([]domain.Issue, error) {
	files, err := s.gitRepo.GetStagedFiles(ctx)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrGitOperation, "获取暂存文件失败", err)
	}
	files = s.filterExcluded(files)
	return s.CheckFiles(ctx, files)
}

// CheckAll 检查所有文件
func (s *CheckerService) CheckAll(ctx context.Context) ([]domain.Issue, error) {
	files, err := s.gitRepo.GetAllFiles(ctx)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrGitOperation, "获取所有文件失败", err)
	}
	files = s.filterExcluded(files)
	return s.CheckFiles(ctx, files)
}

// CheckFiles 检查指定文件
func (s *CheckerService) CheckFiles(ctx context.Context, files []domain.FileInfo) ([]domain.Issue, error) {
	if len(files) == 0 {
		return nil, nil
	}
	files = withoutFileContent(files)

	context := s.getProjectContext()

	patterns, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrDatabase, "获取已知模式失败", err)
	}

	recentCommits, err := s.gitRepo.GetCommits(ctx, 10, "")
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrGitOperation, "获取最近提交失败", err)
	}

	req := &agent.AnalyzeRequest{
		Files:         files,
		Context:       context,
		Patterns:      patterns,
		RecentCommits: recentCommits,
	}

	result, err := s.agent.AnalyzeCode(ctx, req)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, "AI 分析失败", err).WithContext("files_count", len(files))
	}

	return result.Issues, nil
}

func (s *CheckerService) filterExcluded(files []domain.FileInfo) []domain.FileInfo {
	if s.configRepo == nil {
		return files
	}
	return filefilter.FilterFiles(files, s.configRepo.GetExclude())
}

func (s *CheckerService) getProjectContext() agent.ProjectContext {
	if s.configRepo == nil {
		return agent.ProjectContext{
			Name:     "project",
			Language: "go",
		}
	}
	projectConfig := s.configRepo.GetProjectConfig()
	projectContext := agent.ProjectContext{
		Name:     projectConfig.Name,
		Language: projectConfig.Language,
	}
	if projectContext.Name == "" {
		projectContext.Name = "project"
	}
	if projectContext.Language == "" {
		projectContext.Language = "go"
	}
	return projectContext
}

// GetPatterns 获取全部模式
func (s *CheckerService) GetPatterns(ctx context.Context) ([]domain.Pattern, error) {
	return s.patternRepo.GetAll(ctx)
}

// GetHighConfidencePatterns 获取高置信度模式
func (s *CheckerService) GetHighConfidencePatterns(ctx context.Context, threshold float64) ([]domain.Pattern, error) {
	return s.patternRepo.GetHighConfidence(ctx, threshold)
}

// AnalyzeFiles 分析指定绝对路径文件
func (s *CheckerService) AnalyzeFiles(ctx context.Context, absPaths []string) error {
	files := make([]domain.FileInfo, 0, len(absPaths))
	for _, path := range absPaths {
		files = append(files, domain.FileInfo{
			Path:     path,
			Language: domain.NewFileInfo(path, "").Language,
			Status:   domain.StatusModified,
		})
	}
	_, err := s.CheckFiles(ctx, files)
	return err
}

func withoutFileContent(files []domain.FileInfo) []domain.FileInfo {
	clean := make([]domain.FileInfo, 0, len(files))
	for _, file := range files {
		file.Content = ""
		clean = append(clean, file)
	}
	return clean
}
