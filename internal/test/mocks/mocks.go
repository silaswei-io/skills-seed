package mocks

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

// MockAgent 模拟 AI Agent
type MockAgent struct {
	NameVal                    string
	AvailableVal               bool
	CuratePatternsFn           func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error)
	UserDefinePatternFn        func(ctx context.Context, req *agent.UserDefinePatternRequest) (*agent.UserDefinePatternResult, error)
	RefreshProjectProfileFn    func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error)
	SelectLearningCandidatesFn func(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error)
	PlanLearningAgendaFn       func(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error)
	AnalyzeCurrentCodebaseFn   func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error)
	AnalyzeCurrentBatchFn      func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error)
	AnalyzeCurrentDeltaFn      func(ctx context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error)
	StartLearningSessionFn     func(ctx context.Context, req agent.LearningSessionRequest) (agent.LearningSession, error)
	AnalyzeWorkspaceProfileFn  func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error)
	AnalyzeWorkspaceSpecFn     func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error)
	OptimizeWorkflowFn         func(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error)
}

// Name 返回模拟 Agent 名称
func (m *MockAgent) Name() string { return m.NameVal }

// IsAvailable 返回模拟 Agent 是否可用
func (m *MockAgent) IsAvailable() bool { return m.AvailableVal }

// StartLearningSession 模拟当前代码学习会话。
func (m *MockAgent) StartLearningSession(ctx context.Context, req agent.LearningSessionRequest) (agent.LearningSession, error) {
	if m.StartLearningSessionFn != nil {
		return m.StartLearningSessionFn(ctx, req)
	}
	return mockLearningSession{agent: m}, nil
}

// CuratePatterns 模拟模式策展
func (m *MockAgent) CuratePatterns(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
	if m.CuratePatternsFn != nil {
		return m.CuratePatternsFn(ctx, req)
	}
	patterns := make([]agent.CuratedPattern, 0, len(req.CandidatePatterns))
	for _, candidate := range req.CandidatePatterns {
		patterns = append(patterns, agent.CuratedPattern{
			ID:          candidate.ID,
			Name:        candidate.Name,
			Category:    string(candidate.Category),
			Description: candidate.Description,
			Rule:        candidate.Rule,
			Confidence:  candidate.Confidence,
			SourceIDs:   []string{candidate.ID},
		})
	}
	return &agent.CuratePatternsResult{
		Patterns: patterns,
		Dropped:  []agent.CuratedDrop{},
	}, nil
}

// UserDefinePattern 模拟用户自定义模式
func (m *MockAgent) UserDefinePattern(ctx context.Context, req *agent.UserDefinePatternRequest) (*agent.UserDefinePatternResult, error) {
	if m.UserDefinePatternFn != nil {
		return m.UserDefinePatternFn(ctx, req)
	}
	return &agent.UserDefinePatternResult{}, nil
}

// AnalyzeProject 模拟项目分析
func (m *MockAgent) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	if m.RefreshProjectProfileFn != nil {
		return m.RefreshProjectProfileFn(ctx, req)
	}
	return &agent.AnalyzeProjectResult{}, nil
}

// SelectLearningCandidates 模拟当前代码学习候选收敛。
func (m *MockAgent) SelectLearningCandidates(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
	if m.SelectLearningCandidatesFn != nil {
		return m.SelectLearningCandidatesFn(ctx, req)
	}
	return &agent.SelectLearningCandidatesResult{SelectedPaths: append([]string(nil), req.CandidatePaths...), Reason: "mock selects all candidates"}, nil
}

// PlanLearningAgenda 模拟业务学习议程规划。
func (m *MockAgent) PlanLearningAgenda(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	if m.PlanLearningAgendaFn != nil {
		return m.PlanLearningAgendaFn(ctx, req)
	}
	return &agent.PlanLearningAgendaResult{}, nil
}

// AnalyzeCurrentCodebase 模拟当前代码库分析
func (m *MockAgent) AnalyzeCurrentCodebase(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
	if m.AnalyzeCurrentCodebaseFn != nil {
		return m.AnalyzeCurrentCodebaseFn(ctx, req)
	}
	return &agent.AnalyzeCurrentCodebaseResult{}, nil
}

// AnalyzeCurrentCodebaseBatch 模拟当前代码库批量分析。
func (m *MockAgent) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	if m.AnalyzeCurrentBatchFn != nil {
		return m.AnalyzeCurrentBatchFn(ctx, req)
	}
	focuses := make([]agent.AnalyzeCurrentEvidenceResult, 0, len(req.Focuses))
	for _, unit := range req.Focuses {
		if m.AnalyzeCurrentCodebaseFn != nil {
			result, err := m.AnalyzeCurrentCodebaseFn(ctx, &agent.AnalyzeCurrentCodebaseRequest{
				ProjectName:       req.ProjectName,
				RootPath:          req.RootPath,
				Language:          req.Language,
				RuntimeLabel:      req.RuntimeLabel,
				ChangeProfile:     req.ChangeProfile,
				EvidenceFocus:     unit.EvidenceFocus,
				FocusPaths:        unit.FocusPaths,
				Structure:         req.Structure,
				StructurePath:     req.StructurePath,
				StructuralContext: req.StructuralContext,
				MainFiles:         req.MainFiles,
				SampleFiles:       unit.SampleFiles,
				DiffFiles:         unit.DiffFiles,
				UserContext:       req.UserContext,
				UserContextPath:   req.UserContextPath,
				LearningMode:      req.LearningMode,
			})
			if err != nil {
				return nil, err
			}
			focuses = append(focuses, agent.AnalyzeCurrentEvidenceResult{
				FocusID:                   unit.EvidenceFocus.ID,
				FocusName:                 unit.EvidenceFocus.Name,
				Patterns:                  result.Patterns,
				ProfileRefreshRecommended: result.ProfileRefreshRecommended,
			})
			continue
		}
		focuses = append(focuses, agent.AnalyzeCurrentEvidenceResult{
			FocusID:   unit.EvidenceFocus.ID,
			FocusName: unit.EvidenceFocus.Name,
		})
	}
	return &agent.AnalyzeCurrentCodebaseBatchResult{Focuses: focuses}, nil
}

// AnalyzeCurrentDeltaBatch 模拟 diff 锚定增量分析。
func (m *MockAgent) AnalyzeCurrentDeltaBatch(ctx context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	if m.AnalyzeCurrentDeltaFn != nil {
		return m.AnalyzeCurrentDeltaFn(ctx, req)
	}
	if m.AnalyzeCurrentBatchFn != nil {
		batchReq := &agent.AnalyzeCurrentCodebaseBatchRequest{
			ProjectName:           req.ProjectName,
			RootPath:              req.RootPath,
			Language:              req.Language,
			RuntimeLabel:          req.RuntimeLabel,
			Focuses:               make([]agent.AnalyzeCurrentEvidenceFocus, 0, len(req.Focuses)),
			Structure:             req.Structure,
			StructurePath:         req.StructurePath,
			StructuralContext:     req.StructuralContext,
			StructuralContextPath: req.StructuralContextPath,
			UserContext:           req.UserContext,
			UserContextPath:       req.UserContextPath,
			LearningMode:          req.LearningMode,
			ChangeProfile:         req.ChangeProfile,
		}
		for _, unit := range req.Focuses {
			batchReq.Focuses = append(batchReq.Focuses, agent.AnalyzeCurrentEvidenceFocus{
				EvidenceFocus: unit.EvidenceFocus,
				FocusPaths:    unit.FocusPaths,
				SampleFiles:   unit.ContextFiles,
				DiffFiles:     unit.DiffFiles,
			})
		}
		result, err := m.AnalyzeCurrentBatchFn(ctx, batchReq)
		if err != nil {
			return nil, err
		}
		return currentBatchResultToDeltaResult(req.Focuses, result), nil
	}
	if m.AnalyzeCurrentCodebaseFn != nil {
		var changes []domain.KnowledgeChange
		for _, unit := range req.Focuses {
			result, err := m.AnalyzeCurrentCodebaseFn(ctx, &agent.AnalyzeCurrentCodebaseRequest{
				ProjectName:     req.ProjectName,
				RootPath:        req.RootPath,
				Language:        req.Language,
				RuntimeLabel:    req.RuntimeLabel,
				EvidenceFocus:   unit.EvidenceFocus,
				FocusPaths:      unit.FocusPaths,
				Structure:       req.Structure,
				StructurePath:   req.StructurePath,
				DiffFiles:       unit.DiffFiles,
				SampleFiles:     unit.ContextFiles,
				UserContext:     req.UserContext,
				UserContextPath: req.UserContextPath,
				LearningMode:    req.LearningMode,
				ChangeProfile:   req.ChangeProfile,
			})
			if err != nil {
				return nil, err
			}
			if len(result.Patterns) == 0 {
				changes = append(changes, mockNoChange(unit))
			}
			for _, pattern := range result.Patterns {
				anchorPath := ""
				if len(unit.FocusPaths) > 0 {
					anchorPath = unit.FocusPaths[0]
				}
				pattern.DiffAnchors = []domain.PatternDiffAnchor{{Path: anchorPath, ChangeKind: "modified", Description: "mock delta anchor"}}
				changes = append(changes, domain.KnowledgeChange{
					FocusAction:   domain.KnowledgeFocusExisting,
					FocusID:       unit.EvidenceFocus.ID,
					FocusName:     unit.EvidenceFocus.Name,
					PatternAction: domain.KnowledgePatternAdd,
					PatternID:     pattern.ID,
					Proposal:      &pattern,
					Anchors:       pattern.DiffAnchors,
					Reason:        "mock delta analysis",
				})
			}
			if result.ProfileRefreshRecommended.Needed {
				return &agent.AnalyzeCurrentDeltaBatchResult{
					Changes:                   changes,
					ProfileRefreshRecommended: result.ProfileRefreshRecommended,
				}, nil
			}
		}
		return &agent.AnalyzeCurrentDeltaBatchResult{Changes: changes}, nil
	}
	return &agent.AnalyzeCurrentDeltaBatchResult{}, nil
}

func currentBatchResultToDeltaResult(inputFocuses []agent.AnalyzeCurrentDeltaFocus, result *agent.AnalyzeCurrentCodebaseBatchResult) *agent.AnalyzeCurrentDeltaBatchResult {
	if result == nil {
		return &agent.AnalyzeCurrentDeltaBatchResult{}
	}
	focusesByID := make(map[string]agent.AnalyzeCurrentDeltaFocus, len(inputFocuses))
	focusesByName := make(map[string]agent.AnalyzeCurrentDeltaFocus, len(inputFocuses))
	for _, unit := range inputFocuses {
		focusesByID[unit.EvidenceFocus.ID] = unit
		focusesByName[unit.EvidenceFocus.Name] = unit
	}
	var changes []domain.KnowledgeChange
	for _, unitResult := range result.Focuses {
		inputUnit, ok := focusesByID[unitResult.FocusID]
		if !ok && unitResult.FocusName != "" {
			inputUnit, ok = focusesByName[unitResult.FocusName]
		}
		if !ok {
			inputUnit = agent.AnalyzeCurrentDeltaFocus{
				EvidenceFocus: domain.EvidenceFocus{ID: unitResult.FocusID, Name: unitResult.FocusName},
			}
		}
		if len(unitResult.Patterns) == 0 {
			changes = append(changes, mockNoChange(inputUnit))
			continue
		}
		for _, pattern := range unitResult.Patterns {
			anchorPath := ""
			if len(inputUnit.FocusPaths) > 0 {
				anchorPath = inputUnit.FocusPaths[0]
			}
			pattern.DiffAnchors = []domain.PatternDiffAnchor{{Path: anchorPath, ChangeKind: "modified", Description: "mock delta anchor"}}
			changes = append(changes, domain.KnowledgeChange{
				FocusAction:   domain.KnowledgeFocusExisting,
				FocusID:       inputUnit.EvidenceFocus.ID,
				FocusName:     inputUnit.EvidenceFocus.Name,
				PatternAction: domain.KnowledgePatternAdd,
				PatternID:     pattern.ID,
				Proposal:      &pattern,
				Anchors:       pattern.DiffAnchors,
				Reason:        "mock delta analysis",
			})
		}
	}
	out := &agent.AnalyzeCurrentDeltaBatchResult{Changes: changes}
	for _, unitResult := range result.Focuses {
		if unitResult.ProfileRefreshRecommended.Needed {
			out.ProfileRefreshRecommended = unitResult.ProfileRefreshRecommended
			break
		}
	}
	return out
}

func mockNoChange(unit agent.AnalyzeCurrentDeltaFocus) domain.KnowledgeChange {
	anchorPath := ""
	if len(unit.FocusPaths) > 0 {
		anchorPath = unit.FocusPaths[0]
	}
	return domain.KnowledgeChange{
		FocusAction:   domain.KnowledgeFocusExisting,
		FocusID:       unit.EvidenceFocus.ID,
		FocusName:     unit.EvidenceFocus.Name,
		PatternAction: domain.KnowledgePatternNoChange,
		Anchors:       []domain.PatternDiffAnchor{{Path: anchorPath, ChangeKind: "modified", Description: "mock delta no change"}},
		Reason:        "mock delta analysis",
	}
}

type mockLearningSession struct {
	agent *MockAgent
}

func (s mockLearningSession) SessionID() string {
	return "mock-learning-session"
}

func (s mockLearningSession) SelectLearningCandidates(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
	return s.agent.SelectLearningCandidates(ctx, req)
}

func (s mockLearningSession) PlanLearningAgenda(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	return s.agent.PlanLearningAgenda(ctx, req)
}

func (s mockLearningSession) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	return s.agent.AnalyzeCurrentCodebaseBatch(ctx, req)
}

func (s mockLearningSession) AnalyzeCurrentDeltaBatch(ctx context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	return s.agent.AnalyzeCurrentDeltaBatch(ctx, req)
}

func (s mockLearningSession) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	return s.agent.RefreshProjectProfile(ctx, req)
}

func (s mockLearningSession) CuratePatterns(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
	return s.agent.CuratePatterns(ctx, req)
}

func (s mockLearningSession) Close(context.Context) error {
	return nil
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

// OptimizeWorkflow 模拟工作流优化。
func (m *MockAgent) OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
	if m.OptimizeWorkflowFn != nil {
		return m.OptimizeWorkflowFn(ctx, req)
	}
	title := req.Name
	if title == "" {
		title = "workflow"
	}
	return &agent.OptimizeWorkflowResult{
		Title:   title,
		Content: "# " + title + "\n\n## 适用场景\n" + req.Context + "\n",
	}, nil
}

// MockGitRepository 模拟 Git 仓储
type MockGitRepository struct {
	CommitsFn       func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error)
	ChangedFilesFn  func(ctx context.Context, hash string) ([]string, error)
	StagedFilesFn   func(ctx context.Context) ([]domain.FileInfo, error)
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
	GetFn                  func(ctx context.Context, id string) (*domain.Pattern, error)
	GetAllFn               func(ctx context.Context) ([]domain.Pattern, error)
	GetByCategoryFn        func(ctx context.Context, category domain.Category) ([]domain.Pattern, error)
	GetHighConfidenceFn    func(ctx context.Context, threshold float64) ([]domain.Pattern, error)
	SaveFn                 func(ctx context.Context, p *domain.Pattern) error
	ApplyPatternMutationFn func(ctx context.Context, mutation domain.PatternMutation) error
	FindSimilarFn          func(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error)
	DeleteFn               func(ctx context.Context, id string) error
	CountFn                func(ctx context.Context) (int, error)
	GetPatternStatsFn      func(ctx context.Context) ([]domain.PatternStats, error)
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

// ApplyPatternMutation 模拟原子模式变更。
func (m *MockPatternRepository) ApplyPatternMutation(ctx context.Context, mutation domain.PatternMutation) error {
	if m.ApplyPatternMutationFn != nil {
		return m.ApplyPatternMutationFn(ctx, mutation)
	}
	for _, id := range mutation.DeleteIDs {
		if err := m.Delete(ctx, id); err != nil {
			return err
		}
	}
	for _, pattern := range mutation.Save {
		if err := m.Save(ctx, pattern); err != nil {
			return err
		}
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

// GetPatternStats 模拟获取模式质量统计。
func (m *MockPatternRepository) GetPatternStats(ctx context.Context) ([]domain.PatternStats, error) {
	if m.GetPatternStatsFn != nil {
		return m.GetPatternStatsFn(ctx)
	}
	return []domain.PatternStats{}, nil
}

// MockPatternStatsRepository 模拟模式统计仓储。
type MockPatternStatsRepository struct {
	GetPatternStatsFn func(ctx context.Context) ([]domain.PatternStats, error)
}

func (m *MockPatternStatsRepository) GetPatternStats(ctx context.Context) ([]domain.PatternStats, error) {
	if m.GetPatternStatsFn != nil {
		return m.GetPatternStatsFn(ctx)
	}
	return []domain.PatternStats{}, nil
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

// GetForProject 模拟获取工作区子项目画像
func (m *MockProjectProfileRepository) GetForProject(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
	if m.GetForProjectFn != nil {
		return m.GetForProjectFn(ctx, projectID)
	}
	return m.Get(ctx)
}

// SaveForProject 模拟保存工作区子项目画像
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

// GetSpecForProject 模拟获取工作区子项目规范
func (m *MockProjectProfileRepository) GetSpecForProject(ctx context.Context, projectID string) (*domain.ProjectSpec, error) {
	if m.GetSpecForProjectFn != nil {
		return m.GetSpecForProjectFn(ctx, projectID)
	}
	return nil, nil
}

// SaveSpecForProject 模拟保存工作区子项目规范
func (m *MockProjectProfileRepository) SaveSpecForProject(ctx context.Context, projectID string, spec *domain.ProjectSpec) error {
	if m.SaveSpecForProjectFn != nil {
		return m.SaveSpecForProjectFn(ctx, projectID, spec)
	}
	return nil
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
	AgentCfg     config.AgentConfig
	LearningCfg  config.LearningConfig
	SkillsCfg    config.SkillsConfig
	LoggingCfg   config.LoggingConfig
	ExcludeCfg   config.ExcludeConfig
	Exclude      []string
}

// GetProjectConfig 模拟获取项目配置
func (m *MockConfigReader) GetProjectConfig() config.ProjectConfig { return m.ProjectCfg }

// GetWorkspaceConfig 模拟获取工作区配置
func (m *MockConfigReader) GetWorkspaceConfig() config.WorkspaceConfig { return m.WorkspaceCfg }

// GetAgentConfig 模拟获取 Agent 配置
func (m *MockConfigReader) GetAgentConfig() config.AgentConfig { return m.AgentCfg }

// GetLearningConfig 模拟获取学习配置
func (m *MockConfigReader) GetLearningConfig() config.LearningConfig { return m.LearningCfg }

// GetCurrentLearningConfig 模拟获取 learn current 配置
func (m *MockConfigReader) GetCurrentLearningConfig() config.CurrentLearningConfig {
	return m.LearningCfg.Current
}

// GetSkillsConfig 模拟获取 Skills 配置
func (m *MockConfigReader) GetSkillsConfig() config.SkillsConfig { return m.SkillsCfg }

// GetLoggingConfig 模拟获取日志配置
func (m *MockConfigReader) GetLoggingConfig() config.LoggingConfig { return m.LoggingCfg }

// GetExcludeConfig 模拟获取全局排除配置
func (m *MockConfigReader) GetExcludeConfig() config.ExcludeConfig { return m.ExcludeCfg }

// GetExclude 模拟获取排除配置
func (m *MockConfigReader) GetExclude() []string { return m.Exclude }

// GetToolLocale 模拟获取工具输出语言
func (m *MockConfigReader) GetToolLocale() string {
	if m.ProjectCfg.Locale != "" {
		return m.ProjectCfg.Locale
	}
	return "zh-CN"
}

// GetSkillsLocale 模拟获取 AI 输出、沉淀内容和生成 Skills 使用的语言。
func (m *MockConfigReader) GetSkillsLocale() string {
	if m.SkillsCfg.Locale != "" {
		return m.SkillsCfg.Locale
	}
	return "en-US"
}

// GetEffectiveAgentEngine 模拟获取有效 Agent 引擎
func (m *MockConfigReader) GetEffectiveAgentEngine() string {
	if m.AgentCfg.Engine != "" {
		return m.AgentCfg.Engine
	}
	return "claude"
}

// GetEffectiveAgentCommand 模拟获取有效 Agent 命令
func (m *MockConfigReader) GetEffectiveAgentCommand() string {
	engine := m.GetEffectiveAgentEngine()
	if m.AgentCfg.Commands != nil && m.AgentCfg.Commands[engine] != "" {
		return m.AgentCfg.Commands[engine]
	}
	return engine
}

// GetEffectiveSkillsTarget 模拟获取有效 Skills 目标类型
func (m *MockConfigReader) GetEffectiveSkillsTarget() string {
	return config.EffectiveSkillsTarget(m.AgentCfg, m.SkillsCfg)
}

// GetEffectiveSkillsPath 模拟获取有效 Skills 输出路径
func (m *MockConfigReader) GetEffectiveSkillsPath() string {
	return config.EffectiveSkillsPath(m.GetEffectiveSkillsTarget(), m.SkillsCfg)
}

// GetWorkspaceProjects 模拟获取工作区子项目配置
func (m *MockConfigReader) GetWorkspaceProjects() []config.WorkspaceProjectConfig {
	projects := make([]config.WorkspaceProjectConfig, len(m.WorkspaceCfg.Projects))
	copy(projects, m.WorkspaceCfg.Projects)
	return projects
}
