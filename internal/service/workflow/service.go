package workflow

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/workflow"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

// Optimizer 整理用户提供的工作流正文。
type Optimizer interface {
	OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeContentResult, error)
}

// Service 管理用户维护的工作流资源。
type Service struct {
	repo      domain.WorkflowRepository
	optimizer Optimizer
	project   agent.ProjectContext
}

// NewService 创建工作流服务。
func NewService(repo domain.WorkflowRepository, optimizer Optimizer, project agent.ProjectContext) *Service {
	return &Service{repo: repo, optimizer: optimizer, project: project}
}

// UpsertRequest 描述新增、合并或覆盖工作流的请求。
type UpsertRequest struct {
	Name      string
	Content   string
	Overwrite bool
}

// UpsertWorkflow 优化并保存用户工作流。
func (s *Service) UpsertWorkflow(ctx context.Context, req UpsertRequest) (*domain.Workflow, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New(i18n.Get("WorkflowRepositoryMissing"))
	}
	if s.optimizer == nil {
		return nil, errors.New(i18n.Get("WorkflowOptimizerMissing"))
	}
	name := strings.TrimSpace(req.Name)
	content := strings.TrimSpace(req.Content)
	id := runtimefiles.SafePart(name, "")
	if name == "" || id == "" {
		return nil, errors.New(i18n.Get("WorkflowNameRequired"))
	}
	if content == "" {
		return nil, errors.New(i18n.Get("WorkflowContentRequired"))
	}

	existing, err := s.repo.Get(id)
	if err != nil && !errors.Is(err, workflow.ErrNotFound) {
		return nil, err
	}
	existingContent := ""
	existingSummary := ""
	var existingRouteTerms []string
	if existing != nil && !req.Overwrite {
		existingContent = strings.TrimSpace(existing.Content)
		existingSummary = strings.TrimSpace(existing.Summary)
		existingRouteTerms = existing.RouteTerms
	}
	optimized, err := s.optimizer.OptimizeWorkflow(ctx, &agent.OptimizeWorkflowRequest{
		Project:         s.project,
		Name:            name,
		ExistingContent: existingContent,
		Content:         content,
		Overwrite:       req.Overwrite,
	})
	if err != nil {
		return nil, err
	}
	if err := agent.RequireResult(optimized, "OptimizeWorkflow"); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(optimized.Content)
	if content == "" {
		return nil, errors.New(i18n.Get("WorkflowOptimizerEmptyContent"))
	}
	summary := strings.TrimSpace(optimized.Summary)
	routeTerms := stringx.UniqueNonBlank(optimized.RouteTerms)
	if existing != nil && !req.Overwrite {
		if summary == "" {
			summary = existingSummary
		}
		routeTerms = stringx.UniqueNonBlank(append(existingRouteTerms, routeTerms...))
	}

	now := time.Now()
	workflow := domain.Workflow{ID: id, Name: name, Content: content, Summary: summary, RouteTerms: routeTerms, CreatedAt: now, UpdatedAt: now}
	if existing != nil {
		workflow.CreatedAt = existing.CreatedAt
		workflow.Scripts = existing.Scripts
	}
	if err := s.repo.Save(workflow); err != nil {
		return nil, err
	}
	return s.repo.Get(id)
}
