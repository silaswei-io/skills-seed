package knowledge

import (
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

const (
	businessFallbackGroupID = "other"
	maxBusinessGroupSignals = 5
)

type BusinessGroup struct {
	ID                  string
	Title               string
	Path                string
	Summary             BusinessGroupSummary
	Patterns            []domain.Pattern
	Locations           []BusinessLocation
	Signals             []string
	RouteTerms          []string
	HasDevelopmentFocus bool
}

type BusinessGroupSummary struct {
	Description string
	Keywords    []string
	PrimaryPath string
	IsFallback  bool
}

type BusinessLocation struct {
	Path        string
	Symbol      string
	Kind        string
	Description string
	Confidence  float64
}

func BusinessPatternGroups(locale string, patterns []domain.Pattern) []BusinessGroup {
	groupsByID := make(map[string]*BusinessGroup)
	order := make([]string, 0)

	for _, pattern := range patterns {
		key := businessPatternGroupKey(pattern)
		if key.ID == "" {
			key = businessFallbackGroupKey(locale)
		}
		group, ok := groupsByID[key.ID]
		if !ok {
			group = &BusinessGroup{
				ID:                  key.ID,
				Title:               key.Title,
				Path:                "./business/" + key.ID + ".md",
				RouteTerms:          append([]string(nil), key.RouteTerms...),
				HasDevelopmentFocus: key.HasDevelopmentFocus,
			}
			groupsByID[key.ID] = group
			order = append(order, key.ID)
		}
		group.Patterns = append(group.Patterns, pattern)
		group.Locations = mergeBusinessLocations(group.Locations, businessPatternLocations(pattern))
		group.Signals = mergeBusinessSignals(group.Signals, businessPatternSignals(pattern))
		group.RouteTerms = mergeBusinessSignals(group.RouteTerms, businessPatternRouteTerms(pattern))
	}

	sort.SliceStable(order, func(i, j int) bool {
		left := groupsByID[order[i]]
		right := groupsByID[order[j]]
		if len(left.Patterns) != len(right.Patterns) {
			return len(left.Patterns) > len(right.Patterns)
		}
		return left.Title < right.Title
	})

	groups := make([]BusinessGroup, 0, len(order))
	for _, id := range order {
		group := groupsByID[id]
		sortBusinessLocations(group.Locations)
		group.Signals = limitStrings(group.Signals, maxBusinessGroupSignals)
		group.Summary = buildBusinessGroupSummary(locale, *group)
		groups = append(groups, *group)
	}
	return groups
}

type businessGroupKey struct {
	ID                  string
	Title               string
	RouteTerms          []string
	HasDevelopmentFocus bool
}

func businessPatternGroupKey(pattern domain.Pattern) businessGroupKey {
	if focus := pattern.DevelopmentFocus.Clone(); focus != nil {
		return businessGroupKey{
			ID:                  focus.ID,
			Title:               focus.Name,
			RouteTerms:          focus.RouteTerms,
			HasDevelopmentFocus: true,
		}
	}
	// ScopePath 是项目边界信息，不等同于产品/领域名称；没有显式焦点时
	// 统一进入 fallback，由模式名和源码证据继续定位，避免机械生成伪业务域。
	return businessGroupKey{}
}

func SplitBusinessGroupWords(text string) []string {
	return splitBusinessGroupWords(text)
}

func splitBusinessGroupWords(text string) []string {
	var normalized strings.Builder
	var previous rune
	for _, r := range strings.TrimSpace(text) {
		switch {
		case unicode.IsUpper(r) && previous != 0 && (unicode.IsLower(previous) || unicode.IsDigit(previous)):
			normalized.WriteRune(' ')
			normalized.WriteRune(unicode.ToLower(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			normalized.WriteRune(unicode.ToLower(r))
		default:
			normalized.WriteRune(' ')
		}
		previous = r
	}

	fields := strings.Fields(normalized.String())
	words := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, "-_")
		if field == "" {
			continue
		}
		words = append(words, field)
	}
	return words
}

func TitleFromWords(words []string) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		parts = append(parts, string(runes))
	}
	return strings.Join(parts, " ")
}

func businessGroupDescription(locale, id, title string) string {
	if id == businessFallbackGroupID {
		return i18n.GetForLocale(locale, "GeneratorBusinessGroupFallbackDescription")
	}
	return i18n.GetForLocaleWithParams(locale, "GeneratorBusinessGroupDescription", map[string]interface{}{
		"Title": title,
	})
}

func businessFallbackGroupKey(locale string) businessGroupKey {
	return businessGroupKey{
		ID:    businessFallbackGroupID,
		Title: i18n.GetForLocale(locale, "KnowledgeRoutingBusinessFallbackTitle"),
	}
}

func buildBusinessGroupSummary(locale string, group BusinessGroup) BusinessGroupSummary {
	summary := BusinessGroupSummary{
		Description: businessGroupDescription(locale, group.ID, group.Title),
		Keywords:    businessGroupKeywords(group),
		IsFallback:  group.ID == businessFallbackGroupID,
	}
	if len(group.Locations) > 0 {
		summary.PrimaryPath = group.Locations[0].Path
	}
	return summary
}

func businessGroupKeywords(group BusinessGroup) []string {
	if group.ID == businessFallbackGroupID && len(group.Signals) > 0 {
		return limitStrings(stringx.UniqueNonBlank(group.Signals), maxBusinessGroupSignals)
	}
	if len(group.RouteTerms) > 0 {
		return limitStrings(stringx.UniqueNonBlank(group.RouteTerms), maxBusinessGroupSignals)
	}
	keywords := SplitBusinessGroupWords(group.Title)
	if len(keywords) == 0 {
		keywords = SplitBusinessGroupWords(group.ID)
	}
	return limitStrings(stringx.UniqueNonBlank(keywords), maxBusinessGroupSignals)
}

func businessPatternLocations(pattern domain.Pattern) []BusinessLocation {
	locations := make([]BusinessLocation, 0, len(pattern.EvidenceLocations)+1)
	if pattern.BusinessMethod != nil && pattern.BusinessMethod.DisplayLocation() != "" {
		locations = append(locations, BusinessLocation{
			Path:        pattern.BusinessMethod.DisplayLocation(),
			Symbol:      pattern.BusinessMethod.Name,
			Kind:        "method",
			Description: pattern.BusinessMethod.Description,
			Confidence:  pattern.BusinessMethod.CodeLocation.Confidence,
		})
	}
	for _, evidence := range pattern.EvidenceLocations {
		if evidence.DisplayLocation() == "" {
			continue
		}
		locations = append(locations, BusinessLocation{
			Path:        evidence.DisplayLocation(),
			Symbol:      evidence.Symbol,
			Kind:        evidence.Kind,
			Description: evidence.Description,
			Confidence:  evidence.Confidence,
		})
	}
	if pattern.ScopePath != "" {
		locations = append(locations, BusinessLocation{Path: pattern.ScopePath, Kind: "scope"})
	}
	return locations
}

func businessPatternSignals(pattern domain.Pattern) []string {
	return stringx.UniqueNonBlank([]string{
		pattern.Name,
		pattern.ID,
	})
}

func businessPatternRouteTerms(pattern domain.Pattern) []string {
	if pattern.DevelopmentFocus == nil {
		return nil
	}
	return pattern.DevelopmentFocus.RouteTerms
}

func mergeBusinessLocations(left, right []BusinessLocation) []BusinessLocation {
	seen := make(map[string]bool, len(left)+len(right))
	result := make([]BusinessLocation, 0, len(left)+len(right))
	add := func(location BusinessLocation) {
		key := strings.Join([]string{location.Path, location.Symbol, location.Kind}, "\x00")
		if strings.TrimSpace(location.Path) == "" || seen[key] {
			return
		}
		seen[key] = true
		result = append(result, location)
	}
	for _, location := range left {
		add(location)
	}
	for _, location := range right {
		add(location)
	}
	return result
}

func mergeBusinessSignals(left, right []string) []string {
	return stringx.UniqueNonBlank(append(append([]string{}, left...), right...))
}

func sortBusinessLocations(locations []BusinessLocation) {
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i].Confidence != locations[j].Confidence {
			return locations[i].Confidence > locations[j].Confidence
		}
		return locations[i].Path < locations[j].Path
	})
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
