package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	jsonrepairgo "github.com/silaswei-io/jsonrepair-go"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

type businessMethodPayload struct {
	Name          string               `json:"name"`
	CodeLocation  flexibleCodeLocation `json:"code_location"`
	Description   string               `json:"description"`
	Usage         string               `json:"usage"`
	Type          string               `json:"type"`
	Function      string               `json:"function"`
	Prerequisites flexibleText         `json:"prerequisites"`
	Returns       flexibleText         `json:"returns"`
}

type workspaceSpecPayload struct {
	domain.WorkspaceSpec
	ChangeOrder []json.RawMessage `json:"change_order"`
}

type patternPayload struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Category          string                     `json:"category"`
	Description       string                     `json:"description"`
	GoodExample       string                     `json:"good_example"`
	BadExample        string                     `json:"bad_example"`
	Rule              string                     `json:"rule"`
	Confidence        float64                    `json:"confidence"`
	Frequency         int                        `json:"frequency"`
	AnalysisUnitID    string                     `json:"analysis_unit_id"`
	AnalysisUnitName  string                     `json:"analysis_unit_name"`
	EvidenceLocations []flexibleEvidenceLocation `json:"evidence_locations"`
	BusinessMethod    *businessMethodPayload     `json:"business_method"`
}

type curatedPatternPayload struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Category          string                     `json:"category"`
	Description       string                     `json:"description"`
	GoodExample       string                     `json:"good_example"`
	BadExample        string                     `json:"bad_example"`
	Rule              string                     `json:"rule"`
	Confidence        float64                    `json:"confidence"`
	Frequency         int                        `json:"frequency"`
	MergedFrom        []string                   `json:"merged_from"`
	MergeReason       string                     `json:"merge_reason"`
	SimilarityScore   float64                    `json:"similarity_score"`
	Source            string                     `json:"source"`
	EvidenceLocations []flexibleEvidenceLocation `json:"evidence_locations"`
	BusinessMethod    *businessMethodPayload     `json:"business_method"`
	ProjectID         string                     `json:"project_id"`
	ScopePath         string                     `json:"scope_path"`
	WorkspaceRole     string                     `json:"workspace_role"`
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
		EvidenceLocations: evidenceLocationsToDomain(p.EvidenceLocations),
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
		EvidenceLocations: evidenceLocationsToDomain(p.EvidenceLocations),
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
		CodeLocation:  p.CodeLocation.location(),
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
		if shapeErr := jsonrepairgo.UnmarshalJSONFromText(jsonStr, target); shapeErr == nil {
			return nil
		}
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

type flexibleCodeLocation struct {
	value domain.CodeLocation
}

func (l *flexibleCodeLocation) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		text = strings.TrimSpace(text)
		if text != "" {
			l.value = domain.CodeLocation{CurrentLocation: text}
		}
		return nil
	}

	var location domain.CodeLocation
	if err := json.Unmarshal(data, &location); err != nil {
		return err
	}
	l.value = location
	return nil
}

func (l flexibleCodeLocation) location() domain.CodeLocation {
	return l.value
}

type flexibleEvidenceLocation struct {
	value domain.PatternEvidenceLocation
}

func (l *flexibleEvidenceLocation) UnmarshalJSON(data []byte) error {
	var payload struct {
		Path        string          `json:"path"`
		Line        flexibleInt     `json:"line"`
		Symbol      string          `json:"symbol"`
		Kind        string          `json:"kind"`
		Description string          `json:"description"`
		Confidence  flexibleFloat64 `json:"confidence"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	l.value = domain.PatternEvidenceLocation{
		Path:        payload.Path,
		Line:        payload.Line.int(),
		Symbol:      payload.Symbol,
		Kind:        payload.Kind,
		Description: payload.Description,
		Confidence:  payload.Confidence.float64(),
	}
	return nil
}

func evidenceLocationsToDomain(locations []flexibleEvidenceLocation) []domain.PatternEvidenceLocation {
	out := make([]domain.PatternEvidenceLocation, len(locations))
	for i, location := range locations {
		out[i] = location.value
	}
	return out
}

type flexibleInt int

func (v *flexibleInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	var number int
	if err := json.Unmarshal(data, &number); err == nil {
		*v = flexibleInt(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	number, ok := leadingInt(text)
	if !ok {
		return fmt.Errorf("expected integer or integer string")
	}
	*v = flexibleInt(number)
	return nil
}

func (v flexibleInt) int() int {
	return int(v)
}

type flexibleFloat64 float64

func (v *flexibleFloat64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*v = flexibleFloat64(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return err
	}
	*v = flexibleFloat64(number)
	return nil
}

func (v flexibleFloat64) float64() float64 {
	return float64(v)
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

func leadingInt(text string) (int, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	i := 0
	if text[i] == '-' || text[i] == '+' {
		i++
	}
	startDigits := i
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	if i == startDigits {
		return 0, false
	}
	number, err := strconv.Atoi(text[:i])
	return number, err == nil
}
