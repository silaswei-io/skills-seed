package analyzer

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
)

func newStructuralProvider(cfg config.StructuralConfig) structuralProvider {
	switch config.NormalizeStructuralProvider(string(cfg.Provider)) {
	case config.StructuralProviderTreeSitter:
		return newTreeSitterProvider(cfg)
	case config.StructuralProviderCodeGraph:
		return newCodeGraphProvider(cfg)
	default:
		return fallbackStructuralProvider{
			primary:  newCodeGraphProvider(cfg),
			fallback: newTreeSitterProvider(cfg),
		}
	}
}

type fallbackStructuralProvider struct {
	primary  structuralProvider
	fallback structuralProvider
}

func (p fallbackStructuralProvider) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error) {
	data, err := p.primary.Collect(ctx, projectRoot, req)
	if err == nil {
		return data, nil
	}
	return p.fallback.Collect(ctx, projectRoot, req)
}

func (p fallbackStructuralProvider) withPolicy(policy fileanalysis.SelectionPolicy) structuralProvider {
	next := p
	if aware, ok := next.primary.(policyAwareStructuralProvider); ok {
		next.primary = aware.withPolicy(policy)
	}
	if aware, ok := next.fallback.(policyAwareStructuralProvider); ok {
		next.fallback = aware.withPolicy(policy)
	}
	return next
}
