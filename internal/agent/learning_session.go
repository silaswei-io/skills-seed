package agent

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

// LearningSessionRequest 描述一次当前代码学习阶段会话的稳定上下文。
type LearningSessionRequest struct {
	ProjectName     string
	RootPath        string
	Language        string
	Stage           string
	LearningMode    config.LearningMode
	LearningScope   config.LearningScope
	ChangeProfile   string
	UserContext     string
	UserContextPath string
	ResumeSessionID string
}

func (r LearningSessionRequest) AllowedCategories() string {
	return domain.AllowedPatternCategoriesText()
}

// LearningSession 表示一个短生命周期学习对话阶段。
type LearningSession interface {
	SessionID() string
	SelectLearningCandidates(ctx context.Context, req *SelectLearningCandidatesRequest) (*SelectLearningCandidatesResult, error)
	PlanLearningAgenda(ctx context.Context, req *PlanLearningAgendaRequest) (*PlanLearningAgendaResult, error)
	AnalyzeCurrentCodebaseBatch(ctx context.Context, req *AnalyzeCurrentCodebaseBatchRequest) (*AnalyzeCurrentCodebaseBatchResult, error)
	AnalyzeCurrentDeltaBatch(ctx context.Context, req *AnalyzeCurrentDeltaBatchRequest) (*AnalyzeCurrentDeltaBatchResult, error)
	RefreshProjectProfile(ctx context.Context, req *AnalyzeProjectRequest) (*AnalyzeProjectResult, error)
	Close(ctx context.Context) error
}

// LearningSessionProvider 由支持会话复用的 agent 实现。
type LearningSessionProvider interface {
	StartLearningSession(ctx context.Context, req LearningSessionRequest) (LearningSession, error)
}
