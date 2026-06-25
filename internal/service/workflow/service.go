package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	workflowstore "github.com/silaswei-io/skills-seed/internal/infra/storage/workflow"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// Optimizer 将用户口语化输入整理为标准工作流。
type Optimizer interface {
	OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error)
}

// Service 管理用户工作流资源。
type Service struct {
	repo      domain.WorkflowRepository
	optimizer Optimizer
	language  string
}

// NewService 创建工作流服务。
func NewService(repo domain.WorkflowRepository, optimizer Optimizer, language string) *Service {
	return &Service{repo: repo, optimizer: optimizer, language: language}
}

// UpsertRequest 描述新增或更新工作流的请求。
type UpsertRequest struct {
	Name      string
	Context   string
	Overwrite bool
}

// UpsertWorkflow 创建或更新用户工作流。
func (s *Service) UpsertWorkflow(ctx context.Context, req UpsertRequest) (*domain.Workflow, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("workflow repository is not configured")
	}
	if s.optimizer == nil {
		return nil, fmt.Errorf("workflow optimizer is not configured")
	}
	context := strings.TrimSpace(req.Context)
	if context == "" {
		return nil, fmt.Errorf("workflow --context is required")
	}

	name := strings.TrimSpace(req.Name)
	id := runtimefiles.SafePart(name, "")
	now := time.Now()
	var existing *domain.Workflow
	if id != "" {
		var err error
		existing, err = s.repo.Get(id)
		if err != nil && !errors.Is(err, workflowstore.ErrNotFound) {
			return nil, err
		}
	}
	optimized, err := s.optimizer.OptimizeWorkflow(ctx, &agent.OptimizeWorkflowRequest{
		ID:              id,
		Name:            name,
		Context:         context,
		ExistingContent: existingWorkflowContent(existing, req.Overwrite),
		Overwrite:       req.Overwrite,
		Language:        s.language,
	})
	if err != nil {
		return nil, err
	}
	optimizedContent := strings.TrimSpace(optimized.Content)
	if optimizedContent == "" {
		return nil, fmt.Errorf("workflow optimizer returned empty content")
	}
	title := strings.TrimSpace(optimized.Title)
	if title == "" {
		title = name
	}
	if title == "" {
		return nil, fmt.Errorf("workflow optimizer returned empty title")
	}
	if id == "" {
		id, err = s.newGeneratedWorkflowID(title, context)
		if err != nil {
			return nil, err
		}
		if id == "" {
			return nil, fmt.Errorf("workflow optimizer returned unsafe title")
		}
	}
	workflow := domain.Workflow{
		ID:        id,
		Name:      title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing != nil {
		workflow = *existing
		if strings.TrimSpace(workflow.Name) == "" {
			workflow.Name = title
		}
		workflow.UpdatedAt = now
	}
	nextContext := domain.WorkflowContext{
		Content:   context,
		CreatedAt: now,
	}
	if req.Overwrite {
		workflow.Contexts = []domain.WorkflowContext{nextContext}
	} else {
		workflow.Contexts = append(workflow.Contexts, nextContext)
	}
	workflow.Content = optimizedContent
	workflow.Name = title
	if err := s.repo.Save(workflow); err != nil {
		return nil, err
	}
	return s.repo.Get(id)
}

func (s *Service) newGeneratedWorkflowID(title, context string) (string, error) {
	baseID := workflowIDFromGeneratedTitle(title)
	if baseID == "" {
		return "", nil
	}
	if _, err := s.repo.Get(baseID); errors.Is(err, workflowstore.ErrNotFound) {
		return baseID, nil
	} else if err != nil {
		return "", err
	}

	workflows, err := s.repo.List()
	if err != nil {
		return "", err
	}
	used := make(map[string]struct{}, len(workflows))
	for _, workflow := range workflows {
		used[workflow.ID] = struct{}{}
	}
	for i := 2; ; i++ {
		id := fmt.Sprintf("%s-%d", baseID, i)
		if _, ok := used[id]; ok {
			continue
		}
		if _, err := s.repo.Get(id); errors.Is(err, workflowstore.ErrNotFound) {
			return id, nil
		} else if err != nil {
			return "", err
		}
	}
}

func workflowIDFromGeneratedTitle(title string) string {
	return runtimefiles.SafePart(title, "")
}

func existingWorkflowContent(workflow *domain.Workflow, overwrite bool) string {
	if workflow == nil || overwrite {
		return ""
	}
	if strings.TrimSpace(workflow.Content) != "" {
		return strings.TrimSpace(workflow.Content)
	}
	parts := make([]string, 0, len(workflow.Contexts))
	for _, item := range workflow.Contexts {
		if content := strings.TrimSpace(item.Content); content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}
