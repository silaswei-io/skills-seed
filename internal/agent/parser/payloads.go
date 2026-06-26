package parser

import (
	"encoding/json"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

type businessMethodPayload struct {
	Name          string              `json:"name"`
	CodeLocation  domain.CodeLocation `json:"code_location"`
	Description   string              `json:"description"`
	Usage         string              `json:"usage"`
	Type          string              `json:"type"`
	Function      string              `json:"function"`
	Prerequisites string              `json:"prerequisites"`
	Returns       string              `json:"returns"`
}

type workspaceSpecPayload struct {
	domain.WorkspaceSpec
	ChangeOrder []json.RawMessage `json:"change_order"`
}

func (p *businessMethodPayload) toDomain(now time.Time) *domain.BusinessMethod {
	if p == nil {
		return nil
	}
	method := &domain.BusinessMethod{
		Name:          p.Name,
		Description:   p.Description,
		Usage:         p.Usage,
		Type:          p.Type,
		Function:      p.Function,
		Prerequisites: p.Prerequisites,
		Returns:       p.Returns,
		CodeLocation:  p.CodeLocation,
	}
	method.NormalizeCodeLocation(nil, now)
	return method
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func businessMethodsToDomain(methods []businessMethodPayload, now time.Time) []domain.BusinessMethod {
	out := make([]domain.BusinessMethod, len(methods))
	for i, method := range methods {
		out[i] = *method.toDomain(now)
	}
	return out
}

func projectProfileResultFromPayload(payload projectProfilePayload, now time.Time) *agent.AnalyzeProjectResult {
	return payload.toAnalyzeProjectResult(now)
}
