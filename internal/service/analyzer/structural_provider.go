package analyzer

import (
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

func newStructuralProvider(cfg config.StructuralConfig) structuralProvider {
	switch config.NormalizeStructuralProvider(string(cfg.Provider)) {
	case config.StructuralProviderTreeSitter:
		return newTreeSitterProvider(cfg)
	default:
		return newCodeGraphProvider(cfg)
	}
}
