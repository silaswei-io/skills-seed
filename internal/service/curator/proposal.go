package curator

import (
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

type proposal struct {
	Patterns []domain.Pattern
	Dropped  []Drop
}

func proposalFromAgent(result *agent.CuratePatternsResult) *proposal {
	if result == nil {
		return nil
	}
	out := &proposal{
		Patterns: make([]domain.Pattern, 0, len(result.Patterns)),
		Dropped:  make([]Drop, 0, len(result.Dropped)),
	}
	for _, item := range result.Patterns {
		pattern := domain.Pattern{
			ID:          item.ID,
			Name:        item.Name,
			Category:    domain.Category(item.Category),
			Description: item.Description,
			Rule:        item.Rule,
			Confidence:  item.Confidence,
			Merged:      len(item.SourceIDs) > 1,
			MergedFrom:  append([]string(nil), item.SourceIDs...),
		}
		out.Patterns = append(out.Patterns, pattern)
	}
	for _, item := range result.Dropped {
		out.Dropped = append(out.Dropped, Drop{ID: item.ID, Reason: item.Reason})
	}
	return out
}

func cloneBusinessMethod(method *domain.BusinessMethod) *domain.BusinessMethod {
	if method == nil {
		return nil
	}
	cloned := *method
	return &cloned
}

func cloneProposal(value *proposal) *proposal {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Dropped = append([]Drop(nil), value.Dropped...)
	cloned.Patterns = make([]domain.Pattern, len(value.Patterns))
	for i := range value.Patterns {
		cloned.Patterns[i] = value.Patterns[i]
		cloned.Patterns[i].MergedFrom = append([]string(nil), value.Patterns[i].MergedFrom...)
		cloned.Patterns[i].EvidenceLocations = append([]domain.PatternEvidenceLocation(nil), value.Patterns[i].EvidenceLocations...)
		cloned.Patterns[i].BusinessMethod = cloneBusinessMethod(value.Patterns[i].BusinessMethod)
	}
	return &cloned
}
