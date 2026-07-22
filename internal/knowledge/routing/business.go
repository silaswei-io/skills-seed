package routing

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

const (
	businessFallbackGroupID = "other"
	maxBusinessGroupWords   = 4
	maxBusinessGroupSignals = 5
)

type BusinessGroup struct {
	ID        string
	Title     string
	Path      string
	Summary   BusinessGroupSummary
	Patterns  []domain.Pattern
	Locations []BusinessLocation
	Signals   []string
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
				ID:    key.ID,
				Title: key.Title,
				Path:  "./business/" + key.ID + ".md",
			}
			groupsByID[key.ID] = group
			order = append(order, key.ID)
		}
		group.Patterns = append(group.Patterns, pattern)
		group.Locations = mergeBusinessLocations(group.Locations, businessPatternLocations(pattern))
		group.Signals = mergeBusinessSignals(group.Signals, businessPatternSignals(pattern))
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
	ID    string
	Title string
}

func businessPatternGroupKey(pattern domain.Pattern) businessGroupKey {
	if key := businessGroupKeyFromPath(pattern); key.ID != "" {
		return key
	}
	if key := businessGroupKeyFromEvidence(pattern); key.ID != "" {
		return key
	}
	if pattern.ScopePath != "" {
		return businessGroupKeyFromPathText(pattern.ScopePath)
	}
	if key := businessGroupKeyFromAnalysisUnit(pattern); key.ID != "" {
		return key
	}
	if key := businessGroupKeyFromPatternText(pattern); key.ID != "" {
		return key
	}
	return businessGroupKey{}
}

func businessGroupKeyFromAnalysisUnit(pattern domain.Pattern) businessGroupKey {
	idWords := splitBusinessGroupWords(pattern.AnalysisUnitID)
	if len(idWords) == 0 {
		idWords = splitBusinessGroupWords(pattern.AnalysisUnitName)
	}
	if len(idWords) == 0 {
		return businessGroupKey{}
	}
	title := strings.TrimSpace(pattern.AnalysisUnitName)
	if title == "" {
		title = TitleFromWords(idWords)
	}
	return businessGroupKey{
		ID:    strings.Join(idWords, "-"),
		Title: title,
	}
}

func businessGroupKeyFromPath(pattern domain.Pattern) businessGroupKey {
	if pattern.BusinessMethod == nil {
		return businessGroupKey{}
	}
	return businessGroupKeyFromPathText(pattern.BusinessMethod.DisplayLocation())
}

func businessGroupKeyFromEvidence(pattern domain.Pattern) businessGroupKey {
	for _, location := range pattern.EvidenceLocations {
		if key := businessGroupKeyFromPathText(location.DisplayLocation()); key.ID != "" {
			return key
		}
	}
	return businessGroupKey{}
}

func businessGroupKeyFromPatternText(pattern domain.Pattern) businessGroupKey {
	text := firstNonEmptyString(pattern.Name, pattern.ID, pattern.Rule, pattern.Description)
	return businessGroupKeyFromName(text)
}

func businessGroupKeyFromPathText(location string) businessGroupKey {
	location = strings.TrimSpace(location)
	if location == "" {
		return businessGroupKey{}
	}
	if idx := strings.Index(location, ":"); idx >= 0 {
		location = location[:idx]
	}

	parts := strings.FieldsFunc(filepath.ToSlash(location), func(r rune) bool {
		return r == '/'
	})
	fileStem := ""
	if len(parts) > 0 && filepath.Ext(parts[len(parts)-1]) != "" {
		fileStem = strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
		parts = parts[:len(parts)-1]
	}
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" || isGenericBusinessDirectory(part) {
			continue
		}
		part = strings.TrimSuffix(part, filepath.Ext(part))
		if key := businessGroupKeyFromName(part); key.ID != "" {
			return key
		}
	}
	return businessGroupKeyFromName(normalizeBusinessFileStem(fileStem))
}

func isGenericBusinessDirectory(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "app", "application", "biz", "business", "cmd", "controller", "domain", "handler", "handlers",
		"internal", "logic", "model", "pkg", "repository", "route", "service", "services", "src", "svc",
		"usecase", "usecases":
		return true
	default:
		return false
	}
}

func normalizeBusinessFileStem(value string) string {
	words := splitBusinessGroupWords(value)
	if len(words) >= 2 && words[len(words)-2] == "tp" && isDigits(words[len(words)-1]) {
		words = words[:len(words)-1]
	}
	if len(words) > 1 && isBusinessRoleSuffix(words[len(words)-1]) {
		words = words[:len(words)-1]
	}
	return strings.Join(words, "-")
}

func isBusinessRoleSuffix(value string) bool {
	switch value {
	case "client", "controller", "handler", "logic", "manager", "reloader", "repository", "service":
		return true
	default:
		return false
	}
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func businessGroupKeyFromName(name string) businessGroupKey {
	words := SplitBusinessGroupWords(name)
	if len(words) == 0 {
		return businessGroupKey{}
	}
	if len(words) > maxBusinessGroupWords {
		words = words[:maxBusinessGroupWords]
	}
	id := strings.Join(words, "-")
	return businessGroupKey{
		ID:    id,
		Title: TitleFromWords(words),
	}
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
	keywords := SplitBusinessGroupWords(group.Title)
	if len(keywords) == 0 {
		keywords = SplitBusinessGroupWords(group.ID)
	}
	for _, signal := range group.Signals {
		keywords = append(keywords, SplitBusinessGroupWords(signal)...)
	}
	return limitStrings(uniqueStrings(keywords), maxBusinessGroupSignals)
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
	return uniqueStrings([]string{
		pattern.Name,
		pattern.ID,
	})
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
	return uniqueStrings(append(append([]string{}, left...), right...))
}

func sortBusinessLocations(locations []BusinessLocation) {
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i].Confidence != locations[j].Confidence {
			return locations[i].Confidence > locations[j].Confidence
		}
		return locations[i].Path < locations[j].Path
	})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
