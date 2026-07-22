package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	jsonrepair "github.com/silaswei-io/jsonrepair-go"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
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
		AnalysisUnitID:    p.AnalysisUnitID,
		AnalysisUnitName:  p.AnalysisUnitName,
	}
	if source == domain.SourceUserDefined {
		pattern.UpdatedAt = now
	}
	pattern.BusinessMethod = businessMethodToDomain(p.BusinessMethod, pattern.CreatedAt)
	return pattern
}

func curatedPatternToAgent(p aicontract.CuratedPatternOutput) agent.CuratedPattern {
	return agent.CuratedPattern{
		ID:          p.ID,
		Name:        p.Name,
		Category:    p.Category,
		Description: p.Description,
		Rule:        p.Rule,
		Confidence:  p.Confidence,
		SourceIDs:   p.SourceIDs,
	}
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
	if values == nil {
		return []string{}
	}
	return values
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

func patternsToDomain(patterns []aicontract.PatternOutput, source domain.Source, now time.Time) []domain.Pattern {
	out := make([]domain.Pattern, len(patterns))
	for i, pattern := range patterns {
		out[i] = patternToDomain(pattern, source, now)
	}
	return out
}

func parseJSONPayload(jsonStr string, target any) error {
	normalized, err := jsonrepair.Repair(jsonStr)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	decoder := json.NewDecoder(strings.NewReader(normalized))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	return nil
}
