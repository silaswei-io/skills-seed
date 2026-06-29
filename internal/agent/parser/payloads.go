package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

type businessMethodPayload struct {
	Name          string              `json:"name"`
	CodeLocation  domain.CodeLocation `json:"code_location"`
	Description   string              `json:"description"`
	Usage         string              `json:"usage"`
	Type          string              `json:"type"`
	Function      string              `json:"function"`
	Prerequisites flexibleText        `json:"prerequisites"`
	Returns       flexibleText        `json:"returns"`
}

type workspaceSpecPayload struct {
	domain.WorkspaceSpec
	ChangeOrder []json.RawMessage `json:"change_order"`
}

type patternPayload struct {
	ID                string                           `json:"id"`
	Name              string                           `json:"name"`
	Category          string                           `json:"category"`
	Description       string                           `json:"description"`
	GoodExample       string                           `json:"good_example"`
	BadExample        string                           `json:"bad_example"`
	Rule              string                           `json:"rule"`
	Confidence        float64                          `json:"confidence"`
	Frequency         int                              `json:"frequency"`
	AnalysisUnitID    string                           `json:"analysis_unit_id"`
	AnalysisUnitName  string                           `json:"analysis_unit_name"`
	EvidenceLocations []domain.PatternEvidenceLocation `json:"evidence_locations"`
	BusinessMethod    *businessMethodPayload           `json:"business_method"`
}

type curatedPatternPayload struct {
	ID                string                           `json:"id"`
	Name              string                           `json:"name"`
	Category          string                           `json:"category"`
	Description       string                           `json:"description"`
	GoodExample       string                           `json:"good_example"`
	BadExample        string                           `json:"bad_example"`
	Rule              string                           `json:"rule"`
	Confidence        float64                          `json:"confidence"`
	Frequency         int                              `json:"frequency"`
	MergedFrom        []string                         `json:"merged_from"`
	MergeReason       string                           `json:"merge_reason"`
	SimilarityScore   float64                          `json:"similarity_score"`
	Source            string                           `json:"source"`
	EvidenceLocations []domain.PatternEvidenceLocation `json:"evidence_locations"`
	BusinessMethod    *businessMethodPayload           `json:"business_method"`
	ProjectID         string                           `json:"project_id"`
	ScopePath         string                           `json:"scope_path"`
	WorkspaceRole     string                           `json:"workspace_role"`
}

func (p patternPayload) toDomainPattern(source domain.Source, now time.Time) domain.Pattern {
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
		EvidenceLocations: p.EvidenceLocations,
		AnalysisUnitID:    p.AnalysisUnitID,
		AnalysisUnitName:  p.AnalysisUnitName,
	}
	if source == domain.SourceUserDefined {
		pattern.UpdatedAt = now
	}
	pattern.BusinessMethod = p.BusinessMethod.toDomain(pattern.CreatedAt)
	return pattern
}

func (p curatedPatternPayload) toCuratedPattern(now time.Time) agent.CuratedPattern {
	return agent.CuratedPattern{
		ID:                p.ID,
		Name:              p.Name,
		Category:          p.Category,
		Description:       p.Description,
		GoodExample:       p.GoodExample,
		BadExample:        p.BadExample,
		Rule:              p.Rule,
		Confidence:        p.Confidence,
		Frequency:         p.Frequency,
		MergedFrom:        p.MergedFrom,
		MergeReason:       p.MergeReason,
		SimilarityScore:   p.SimilarityScore,
		Source:            p.Source,
		EvidenceLocations: p.EvidenceLocations,
		BusinessMethod:    p.BusinessMethod.toDomain(now),
		ProjectID:         p.ProjectID,
		ScopePath:         p.ScopePath,
		WorkspaceRole:     p.WorkspaceRole,
	}
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
		Prerequisites: p.Prerequisites.string(),
		Returns:       p.Returns.string(),
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

func patternsToDomain(patterns []patternPayload, source domain.Source, now time.Time) []domain.Pattern {
	out := make([]domain.Pattern, len(patterns))
	for i, pattern := range patterns {
		out[i] = pattern.toDomainPattern(source, now)
	}
	return out
}

func parseJSONPayload(jsonStr string, target any) error {
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	return nil
}

type flexibleStringList []string

func (l *flexibleStringList) UnmarshalJSON(data []byte) error {
	values, err := parseFlexibleStringList(data)
	if err != nil {
		return err
	}
	*l = values
	return nil
}

func (l flexibleStringList) strings() []string {
	return nonNilStrings([]string(l))
}

type flexibleText string

func (v *flexibleText) UnmarshalJSON(data []byte) error {
	text, err := parseFlexibleText(data)
	if err != nil {
		return err
	}
	*v = flexibleText(text)
	return nil
}

func (v flexibleText) string() string {
	return string(v)
}

func parseFlexibleStringList(data []byte) ([]string, error) {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return []string{}, nil
	}

	var values []string
	if err := json.Unmarshal(trimmed, &values); err == nil {
		return nonNilStrings(values), nil
	}

	var value string
	if err := json.Unmarshal(trimmed, &value); err == nil {
		if value == "" {
			return []string{}, nil
		}
		return []string{value}, nil
	}

	return nil, fmt.Errorf("expected string or string array")
}

func parseFlexibleText(data []byte) (string, error) {
	values, err := parseFlexibleStringList(data)
	if err != nil {
		return "", err
	}
	return strings.Join(values, "; "), nil
}
