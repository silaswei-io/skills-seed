package policy

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

type Strength string

const (
	StrengthCore        Strength = "core"
	StrengthConvention  Strength = "convention"
	StrengthLocal       Strength = "local"
	StrengthObservation Strength = "observation"
)

type Decision struct {
	Strength Strength
}

func EvaluatePattern(pattern domain.Pattern) Decision {
	evidenceCount := PatternEvidenceCount(pattern)
	if pattern.Confidence >= 0.90 && pattern.Frequency >= 2 && evidenceCount >= 2 {
		return Decision{Strength: StrengthCore}
	}
	if pattern.Confidence >= 0.85 && (pattern.Frequency >= 2 || evidenceCount >= 2) {
		return Decision{Strength: StrengthConvention}
	}
	if pattern.Confidence >= 0.70 && (evidenceCount >= 1 || strings.TrimSpace(pattern.ScopePath) != "") {
		return Decision{Strength: StrengthLocal}
	}
	return Decision{Strength: StrengthObservation}
}

// ShouldSoftenPatternText 判断模式是否只能作为线索展示，避免弱证据被写成硬规则。
func ShouldSoftenPatternText(pattern domain.Pattern) bool {
	strength := EvaluatePattern(pattern).Strength
	return strength == StrengthLocal || strength == StrengthObservation
}

// DisplayPatternText 返回面向生成产物的模式文本；弱证据模式会降级措辞。
func DisplayPatternText(pattern domain.Pattern, locale string) string {
	text := strings.TrimSpace(pattern.Rule)
	hasRule := text != ""
	if text == "" {
		text = strings.TrimSpace(pattern.Description)
	}
	if text == "" {
		text = strings.TrimSpace(pattern.Name)
	}
	if hasRule && ShouldSoftenPatternText(pattern) {
		return SoftenConstraintText(text, locale)
	}
	return text
}

// SoftenConstraintText 将“必须/禁止”等硬约束措辞降级为需要复核的定位线索。
func SoftenConstraintText(text, locale string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(locale, "en") {
		softened := replaceWordBounded(text, "must", "should")
		softened = replaceWordBounded(softened, "never", "avoid")
		softened = replaceWordBounded(softened, "required", "expected")
		softened = replaceWordBounded(softened, "forbidden", "discouraged")
		return i18n.GetForLocale(locale, "KnowledgePolicySoftenedPrefix") + softened
	}
	replacer := strings.NewReplacer(
		"必须严格", "需要",
		"必须", "需要",
		"严禁", "避免",
		"禁止", "避免",
		"不能", "不应",
		"不要", "避免",
	)
	return i18n.GetForLocale(locale, "KnowledgePolicySoftenedPrefix") + replacer.Replace(text)
}

func PatternEvidenceCount(pattern domain.Pattern) int {
	evidenceCount := len(pattern.EvidenceLocations)
	if pattern.BusinessMethod != nil && strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()) != "" {
		evidenceCount++
	}
	if pattern.Metrics.EvidenceCount > evidenceCount {
		evidenceCount = pattern.Metrics.EvidenceCount
	}
	return evidenceCount
}

func replaceWordBounded(text, old, replacement string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return text
	}
	for i, field := range fields {
		prefix, core, suffix := splitWordPunctuation(field)
		if strings.EqualFold(core, old) {
			fields[i] = prefix + replacement + suffix
		}
	}
	return strings.Join(fields, " ")
}

func splitWordPunctuation(value string) (string, string, string) {
	start := 0
	for start < len(value) && !isASCIIAlpha(value[start]) {
		start++
	}
	end := len(value)
	for end > start && !isASCIIAlpha(value[end-1]) {
		end--
	}
	return value[:start], value[start:end], value[end:]
}

func isASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}
