package generator

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

const (
	businessFallbackGroupID = "other"
	maxBusinessGroupWords   = 4
)

type patternGroup struct {
	ID          string
	Title       string
	Description string
	Path        string
	Patterns    []domain.Pattern
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
				ID:          key.ID,
				Title:       key.Title,
				Description: businessGroupDescription(locale, key.ID, key.Title),
				Path:        "./business/" + key.ID + ".md",
			}
			groupsByID[key.ID] = group
			order = append(order, key.ID)
		}
		group.Patterns = append(group.Patterns, pattern)
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
	if pattern.ScopePath != "" {
		return businessGroupKeyFromPathText(pattern.ScopePath)
	}
	return businessGroupKey{}
}

func businessGroupKeyFromPath(pattern domain.Pattern) businessGroupKey {
	if pattern.BusinessMethod == nil {
		return businessGroupKey{}
	}
	return businessGroupKeyFromPathText(pattern.BusinessMethod.DisplayLocation())
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
		return localizedText(
			locale,
			"没有稳定代码位置或子域归属的业务规则",
			"Business rules without a stable code location or domain ownership",
		)
	}
	return localizedText(
		locale,
		"与 "+title+" 相关的业务规则、状态约束、错误语义和数据转换证据",
		"Business rules, state constraints, error semantics, and data transformation evidence related to "+title,
	)
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
