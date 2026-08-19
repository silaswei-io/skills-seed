package rule

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	rulestore "github.com/silaswei-io/skills-seed/internal/infra/storage/rule"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

// Optimizer 在项目边界内润色用户提供的规则正文。
type Optimizer interface {
	OptimizeRule(ctx context.Context, req *agent.OptimizeRuleRequest) (*agent.OptimizeContentResult, error)
}

// Service 管理用户维护的规则资源。
type Service struct {
	repo      domain.RuleRepository
	optimizer Optimizer
	project   agent.ProjectContext
}

// NewService 创建规则服务。
func NewService(repo domain.RuleRepository, optimizer Optimizer, project agent.ProjectContext) *Service {
	return &Service{repo: repo, optimizer: optimizer, project: project}
}

// UpsertRequest 描述新增、合并或覆盖规则的请求。
type UpsertRequest struct {
	Name             string
	Content          string
	AffectedProjects []string
	Paths            []string
	Overwrite        bool
}

// UpsertRule 优化并保存用户规则及其明确作用范围。
func (s *Service) UpsertRule(ctx context.Context, req UpsertRequest) (*domain.Rule, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New(i18n.Get("RuleRepositoryMissing"))
	}
	if s.optimizer == nil {
		return nil, errors.New(i18n.Get("RuleOptimizerMissing"))
	}
	name := strings.TrimSpace(req.Name)
	content := strings.TrimSpace(req.Content)
	id := runtimefiles.SafePart(name, "")
	if name == "" || id == "" {
		return nil, errors.New(i18n.Get("RuleNameRequired"))
	}
	if content == "" {
		return nil, errors.New(i18n.Get("RuleContentRequired"))
	}
	existing, err := s.repo.Get(id)
	if err != nil && !errors.Is(err, rulestore.ErrNotFound) {
		return nil, err
	}
	affectedProjects := stringx.UniqueNonBlank(req.AffectedProjects)
	paths := stringx.UniqueNonBlank(req.Paths)
	existingContent := ""
	existingSummary := ""
	existingRouteTerms := []string(nil)
	if existing != nil && !req.Overwrite {
		existingContent = strings.TrimSpace(existing.Content)
		existingSummary = strings.TrimSpace(existing.Summary)
		existingRouteTerms = existing.RouteTerms
		affectedProjects = stringx.UniqueNonBlank(append(existing.AffectedProjects, affectedProjects...))
		paths = stringx.UniqueNonBlank(append(existing.Paths, paths...))
	}
	optimized, err := s.optimizer.OptimizeRule(ctx, &agent.OptimizeRuleRequest{
		Project:          s.project,
		Name:             name,
		ExistingContent:  existingContent,
		Content:          content,
		AffectedProjects: affectedProjects,
		Paths:            paths,
		Overwrite:        req.Overwrite,
	})
	if err != nil {
		return nil, err
	}
	if err := agent.RequireResult(optimized, "OptimizeRule"); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(optimized.Content)
	if content == "" {
		return nil, errors.New(i18n.Get("RuleOptimizerEmptyContent"))
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
	rule := domain.Rule{
		ID:               id,
		Name:             name,
		Content:          content,
		Summary:          summary,
		RouteTerms:       routeTerms,
		AffectedProjects: affectedProjects,
		Paths:            paths,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if existing != nil {
		rule.CreatedAt = existing.CreatedAt
	}
	if err := s.repo.Save(rule); err != nil {
		return nil, err
	}
	return s.repo.Get(id)
}
