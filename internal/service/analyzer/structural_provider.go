package analyzer

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
)

func newStructuralProvider(cfg config.StructuralConfig) structuralProvider {
	treeSitter := newTreeSitterProvider(cfg)
	switch config.NormalizeStructuralProvider(string(cfg.Provider)) {
	case config.StructuralProviderCodeGraph:
		return newCodeGraphProvider(cfg)
	case config.StructuralProviderTreeSitter:
		return treeSitter
	default:
		return &autoStructuralProvider{
			primary:  newCodeGraphProvider(cfg),
			fallback: treeSitter,
		}
	}
}

type autoStructuralProvider struct {
	primary  structuralProvider
	fallback structuralProvider
}

func (p *autoStructuralProvider) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error) {
	data, err := p.primary.Collect(ctx, projectRoot, req)
	if err == nil && data != nil {
		return data, nil
	}
	if err != nil {
		logger.Diagnostic("operation failed",
			"operation", "analyzer.structural_primary_collect",
			"provider", structuralProviderCodeGraph,
			"project_root", projectRoot,
			"error", err,
			"fallback", structuralProviderTreeSitter,
		)
	}
	return p.fallback.Collect(ctx, projectRoot, req)
}

func (p *autoStructuralProvider) withPolicy(policy fileanalysis.SelectionPolicy) structuralProvider {
	next := *p
	if aware, ok := next.fallback.(policyAwareStructuralProvider); ok {
		next.fallback = aware.withPolicy(policy)
	}
	return &next
}
