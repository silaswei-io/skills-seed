package generator

import (
	"path/filepath"
	"regexp"
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

type patternGroup struct {
	ID        string
	Title     string
	Path      string
	Summary   businessGroupSummary
	Patterns  []domain.Pattern
	Locations []businessLocation
	Signals   []string
}

func businessPatternGroups(locale string, patterns []domain.Pattern) []patternGroup {
	groupsByID := make(map[string]*patternGroup)
	order := make([]string, 0)

	for _, pattern := range patterns {
		key := businessPatternGroupKey(pattern)
		if key.ID == "" {
			key = businessFallbackGroupKey(locale)
		}
		group, ok := groupsByID[key.ID]
		if !ok {
			group = &patternGroup{
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

	groups := make([]patternGroup, 0, len(order))
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
	if key := businessGroupKeyFromPatternText(pattern); key.ID != "" {
		return key
	}
	return businessGroupKey{}
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
		if part == "" {
			continue
		}
		part = strings.TrimSuffix(part, filepath.Ext(part))
		if isGenericBusinessPathPart(part) {
			continue
		}
		if key := businessGroupKeyFromName(part); key.ID != "" {
			return key
		}
	}
	if !isGenericBusinessPathPart(fileStem) {
		return businessGroupKeyFromName(fileStem)
	}
	return businessGroupKey{}
}

func businessGroupKeyFromName(name string) businessGroupKey {
	words := splitBusinessGroupWords(name)
	if len(words) == 0 {
		return businessGroupKey{}
	}
	if len(words) > maxBusinessGroupWords {
		words = words[:maxBusinessGroupWords]
	}
	id := strings.Join(words, "-")
	return businessGroupKey{
		ID:    id,
		Title: titleFromWords(words),
	}
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
		if field == "" || isGenericBusinessNameWord(field) {
			continue
		}
		words = append(words, field)
	}
	return words
}

func titleFromWords(words []string) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		if word == "" {
			continue
		}
		parts = append(parts, strings.ToUpper(word[:1])+word[1:])
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
		Title: localizedText(locale, "其他业务规则", "Other Business Rules"),
	}
}

func isGenericBusinessPathPart(part string) bool {
	part = strings.ToLower(strings.TrimSpace(part))
	if part == "" {
		return true
	}
	part = strings.TrimSuffix(part, filepath.Ext(part))
	generic := map[string]bool{
		"app": true, "application": true, "biz": true, "business": true, "cmd": true,
		"controller": true, "domain": true, "handler": true, "internal": true,
		"logic": true, "model": true, "pkg": true, "repository": true, "route": true,
		"service": true, "services": true, "src": true, "usecase": true, "usecases": true,
	}
	if generic[part] {
		return true
	}
	return regexp.MustCompile(`^(v|go|ts|js|py)?\d+$`).MatchString(part)
}

func isGenericBusinessNameWord(word string) bool {
	generic := map[string]bool{
		"business": true, "case": true, "domain": true, "flow": true, "handler": true,
		"logic": true, "manager": true, "method": true, "pattern": true, "pipeline": true,
		"process": true, "processor": true, "rule": true, "service": true, "state": true,
		"strategy": true, "use": true, "workflow": true,
	}
	return generic[strings.ToLower(word)]
}

type businessGroupSummary struct {
	Description string
	Keywords    []string
	PrimaryPath string
	IsFallback  bool
}

type businessLocation struct {
	Path        string
	Symbol      string
	Kind        string
	Description string
	Confidence  float64
}

func buildBusinessGroupSummary(locale string, group patternGroup) businessGroupSummary {
	summary := businessGroupSummary{
		Description: businessGroupDescription(locale, group.ID, group.Title),
		Keywords:    businessGroupKeywords(group),
		IsFallback:  group.ID == businessFallbackGroupID,
	}
	if len(group.Locations) > 0 {
		summary.PrimaryPath = group.Locations[0].Path
	}
	return summary
}

func businessGroupKeywords(group patternGroup) []string {
	keywords := splitBusinessGroupWords(group.Title)
	if len(keywords) == 0 {
		keywords = splitBusinessGroupWords(group.ID)
	}
	for _, signal := range group.Signals {
		keywords = append(keywords, splitBusinessGroupWords(signal)...)
	}
	return limitStrings(uniqueStrings(keywords), maxBusinessGroupSignals)
}

func businessPatternLocations(pattern domain.Pattern) []businessLocation {
	locations := make([]businessLocation, 0, len(pattern.EvidenceLocations)+1)
	if pattern.BusinessMethod != nil && pattern.BusinessMethod.DisplayLocation() != "" {
		locations = append(locations, businessLocation{
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
		locations = append(locations, businessLocation{
			Path:        evidence.DisplayLocation(),
			Symbol:      evidence.Symbol,
			Kind:        evidence.Kind,
			Description: evidence.Description,
			Confidence:  evidence.Confidence,
		})
	}
	if pattern.ScopePath != "" {
		locations = append(locations, businessLocation{Path: pattern.ScopePath, Kind: "scope"})
	}
	return locations
}

func businessPatternSignals(pattern domain.Pattern) []string {
	return uniqueStrings([]string{
		pattern.Name,
		pattern.ID,
		pattern.Rule,
		pattern.Description,
	})
}

func mergeBusinessLocations(left, right []businessLocation) []businessLocation {
	seen := make(map[string]bool, len(left)+len(right))
	result := make([]businessLocation, 0, len(left)+len(right))
	add := func(location businessLocation) {
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

func sortBusinessLocations(locations []businessLocation) {
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i].Confidence != locations[j].Confidence {
			return locations[i].Confidence > locations[j].Confidence
		}
		return locations[i].Path < locations[j].Path
	})
}

func mergeBusinessSignals(left, right []string) []string {
	return uniqueStrings(append(append([]string{}, left...), right...))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func limitStrings(values []string, limit int) []string {
	if limit > 0 && len(values) > limit {
		return values[:limit]
	}
	return values
}
