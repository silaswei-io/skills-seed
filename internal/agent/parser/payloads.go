package parser

import (
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/utils/jsonx"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func patternToDomain(p aicontract.PatternOutput, source domain.Source, now time.Time) domain.Pattern {
	pattern := domain.Pattern{
		ID:                p.ID,
		Name:              p.Name,
		Category:          domain.Category(p.Category),
		Description:       p.Description,
		GoodExample:       p.GoodExample,
		BadExample:        p.BadExample,
		Rule:              p.Rule,
		Confidence:        p.Confidence,
		Frequency:         p.Frequency,
		Source:            source,
		CreatedAt:         now,
		EvidenceLocations: evidenceLocationsToDomain(p.EvidenceLocations),
	}
	if source == domain.SourceUserDefined {
		pattern.UpdatedAt = now
	}
	pattern.BusinessMethod = businessMethodToDomain(p.BusinessMethod, pattern.CreatedAt)
	return pattern
}

func businessMethodToDomain(p *aicontract.BusinessMethodOutput, now time.Time) *domain.BusinessMethod {
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
		CodeLocation: domain.CodeLocation{
			CurrentLocation: p.CodeLocation.CurrentLocation,
		},
	}
	method.NormalizeCodeLocation(nil, now)
	return method
}

func stringsOrEmpty(values []string) []string {
	return stringx.EmptyIfNil(values)
}

func businessMethodsToDomain(methods []aicontract.BusinessMethodOutput, now time.Time) []domain.BusinessMethod {
	out := make([]domain.BusinessMethod, len(methods))
	for i, method := range methods {
		out[i] = *businessMethodToDomain(&method, now)
	}
	return out
}

func evidenceLocationsToDomain(locations []aicontract.EvidenceLocationOutput) []domain.PatternEvidenceLocation {
	out := make([]domain.PatternEvidenceLocation, len(locations))
	for i, location := range locations {
		out[i] = domain.PatternEvidenceLocation{
			Path:        location.Path,
			Line:        location.Line,
			Symbol:      location.Symbol,
			Kind:        location.Kind,
			Description: location.Description,
			Confidence:  location.Confidence,
		}
	}
	return out
}

func diffAnchorsToDomain(anchors []aicontract.DiffAnchorOutput) []domain.PatternDiffAnchor {
	out := make([]domain.PatternDiffAnchor, len(anchors))
	for i, anchor := range anchors {
		out[i] = domain.PatternDiffAnchor{
			Path:        anchor.Path,
			Line:        anchor.Line,
			Symbol:      anchor.Symbol,
			ChangeKind:  anchor.ChangeKind,
			Description: anchor.Description,
		}
	}
	return out
}

func knowledgeChangeToDomain(change aicontract.KnowledgeChangeOutput, now time.Time) domain.KnowledgeChange {
	var proposal *domain.Pattern
	if change.Proposal != nil {
		pattern := patternToDomain(*change.Proposal, domain.SourceLearnedCurrent, now)
		pattern.DiffAnchors = diffAnchorsToDomain(change.Anchors)
		proposal = &pattern
	}
	return domain.KnowledgeChange{
		FocusAction:   domain.KnowledgeFocusAction(change.FocusAction),
		FocusID:       change.FocusID,
		FocusName:     change.FocusName,
		PatternAction: domain.KnowledgePatternAction(change.PatternAction),
		PatternID:     change.PatternID,
		Proposal:      proposal,
		Anchors:       diffAnchorsToDomain(change.Anchors),
		Reason:        change.Reason,
	}
}

func knowledgeChangesToDomain(changes []aicontract.KnowledgeChangeOutput, now time.Time) []domain.KnowledgeChange {
	out := make([]domain.KnowledgeChange, len(changes))
	for i, change := range changes {
		out[i] = knowledgeChangeToDomain(change, now)
	}
	return out
}

func patternsToDomain(patterns []aicontract.PatternOutput, source domain.Source, now time.Time) []domain.Pattern {
	out := make([]domain.Pattern, len(patterns))
	for i, pattern := range patterns {
		out[i] = patternToDomain(pattern, source, now)
	}
	return out
}

func parseJSONPayload(jsonStr string, target any) error {
	if err := jsonx.UnmarshalFromTextStrict(jsonStr, target); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	return nil
}
