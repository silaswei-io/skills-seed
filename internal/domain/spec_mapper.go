package domain

import (
	"sort"
	"strings"
	"time"
)

const (
	projectSpecMaxRules    = 30
	projectSpecMaxGuidance = 40
)

// WorkspaceProjectOverride 描述工作区子项目覆盖字段，
// 用于 NewProjectSpecFromProfile 中避免 domain → config 依赖。
type WorkspaceProjectOverride struct {
	ID       string
	Path     string
	Type     string
	Language string
}

// NewProjectSpecFromProfile 根据 profile 和 patterns 构建项目规范。
// project 参数为可选的工作区子项目覆盖。
func NewProjectSpecFromProfile(profile *ProjectProfile, patterns []Pattern, project WorkspaceProjectOverride) *ProjectSpec {
	if profile == nil {
		return nil
	}
	profile = CleanProjectProfile(profile)

	spec := &ProjectSpec{
		ProjectName:        profile.ProjectName,
		Language:           profile.Language,
		Summary:            profile.Summary,
		ConfigPatterns:     append([]string(nil), profile.ConfigPatterns...),
		FrameworkPatterns:  append([]string(nil), profile.FrameworkPatterns...),
		ValidationCommands: CleanValidationCommands(profile.ValidationCommands),
		GeneratedAt:        time.Now().Format("2006-01-02 15:04:05"),
	}
	if project.ID != "" {
		spec.ProjectID = project.ID
		spec.ProjectName = project.ID
		spec.ScopePath = project.Path
		spec.WorkspaceRole = project.Type
		if project.Language != "" {
			spec.Language = project.Language
		}
	}
	if spec.ProjectName == "" {
		spec.ProjectName = "project"
	}
	if spec.Language == "" {
		spec.Language = "unknown"
	}

	for _, layer := range profile.Layers {
		spec.Boundaries = append(spec.Boundaries, ProjectSpecBoundary{
			Type:             "layer",
			Name:             layer.Name,
			Description:      layer.Description,
			Responsibilities: append([]string(nil), layer.Responsibilities...),
			Paths:            append([]string(nil), layer.Files...),
		})
	}
	for _, module := range profile.KeyModules {
		spec.Boundaries = append(spec.Boundaries, ProjectSpecBoundary{
			Type:             "module",
			Name:             module.Name,
			Description:      module.Description,
			Responsibilities: append([]string(nil), module.Responsibilities...),
			Paths:            []string{module.Path},
		})
	}

	rules, guidance := projectSpecPatternRules(patterns)
	spec.PatternRules = rules
	spec.PatternGuidance = guidance

	for _, method := range profile.BusinessMethods {
		spec.Touchpoints = append(spec.Touchpoints, ProjectSpecTouchpoint{
			Kind:        "business_method",
			Name:        method.Name,
			Path:        method.DisplayLocation(),
			Description: method.Description,
		})
	}
	for _, utility := range profile.CommonUtils {
		spec.Touchpoints = append(spec.Touchpoints, ProjectSpecTouchpoint{
			Kind:        "common_utility",
			Name:        utility.Name,
			Path:        utility.File,
			Description: utility.Description,
		})
	}

	return spec
}

func projectSpecPatternRules(patterns []Pattern) ([]ProjectSpecPatternRule, []ProjectSpecPatternRule) {
	collapsed := collapseProjectSpecPatterns(StrongestPatterns(patterns, 0))
	rules := make([]ProjectSpecPatternRule, 0)
	guidance := make([]ProjectSpecPatternRule, 0)
	for _, pattern := range collapsed {
		rule := ProjectSpecPatternRule{
			Name:        pattern.Name,
			Category:    string(pattern.Category),
			Description: pattern.Description,
			Rule:        pattern.Rule,
			Confidence:  pattern.Confidence,
			Frequency:   pattern.Frequency,
		}
		if patternIsGlobalSpecRule(pattern) {
			rules = append(rules, rule)
		} else if patternIsUsefulProjectSpecGuidance(pattern) {
			rule.Rule = ""
			rule.Description = projectSpecGuidanceText(pattern)
			guidance = append(guidance, rule)
		}
	}
	if len(rules) > projectSpecMaxRules {
		guidance = append(guidance, rules[projectSpecMaxRules:]...)
		rules = rules[:projectSpecMaxRules]
	}
	if len(guidance) > projectSpecMaxGuidance {
		guidance = guidance[:projectSpecMaxGuidance]
	}
	return rules, guidance
}

func collapseProjectSpecPatterns(patterns []Pattern) []Pattern {
	byKey := make(map[string]Pattern, len(patterns))
	for _, pattern := range patterns {
		key := projectSpecPatternKey(pattern)
		if key == "" {
			continue
		}
		existing, ok := byKey[key]
		if !ok || projectSpecPatternScore(pattern) > projectSpecPatternScore(existing) {
			if ok {
				pattern.Frequency += existing.Frequency
				pattern.EvidenceLocations = append(pattern.EvidenceLocations, existing.EvidenceLocations...)
			}
			byKey[key] = pattern
			continue
		}
		existing.Frequency += pattern.Frequency
		existing.EvidenceLocations = append(existing.EvidenceLocations, pattern.EvidenceLocations...)
		byKey[key] = existing
	}
	result := make([]Pattern, 0, len(byKey))
	for _, pattern := range byKey {
		pattern.EvidenceLocations = uniquePatternEvidenceLocations(pattern.EvidenceLocations)
		result = append(result, pattern)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := projectSpecPatternScore(result[i])
		right := projectSpecPatternScore(result[j])
		if left != right {
			return left > right
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func projectSpecPatternKey(pattern Pattern) string {
	text := strings.ToLower(strings.TrimSpace(firstNonEmpty(pattern.Rule, pattern.Description, pattern.Name)))
	if text == "" {
		return ""
	}
	return string(pattern.Category) + ":" + strings.Join(strings.Fields(text), " ")
}

func projectSpecPatternScore(pattern Pattern) float64 {
	return pattern.Confidence*10 + float64(pattern.Frequency) + float64(projectSpecEvidenceCount(pattern))*0.5 - pattern.Metrics.GenericPenalty*2
}

func patternHasExecutableEvidence(pattern Pattern) bool {
	return projectSpecEvidenceCount(pattern) > 0
}

func patternIsGlobalSpecRule(pattern Pattern) bool {
	if !pattern.AllowsHardConstraint() || !patternHasExecutableEvidence(pattern) {
		return false
	}
	evidenceCount := projectSpecEvidenceCount(pattern)
	return pattern.Confidence >= 0.90 &&
		pattern.Frequency >= 2 &&
		evidenceCount >= 2 &&
		pattern.Metrics.GenericPenalty < 0.5 &&
		!projectSpecLooksScoped(pattern)
}

func patternIsUsefulProjectSpecGuidance(pattern Pattern) bool {
	if !patternHasExecutableEvidence(pattern) {
		return false
	}
	if pattern.Confidence < 0.85 {
		return false
	}
	evidenceCount := projectSpecEvidenceCount(pattern)
	if pattern.AllowsHardConstraint() {
		return pattern.Frequency >= 2 || evidenceCount >= 2
	}
	return pattern.Frequency >= 2 && evidenceCount >= 2
}

func projectSpecEvidenceCount(pattern Pattern) int {
	evidenceCount := PatternEvidenceFileCount(pattern.EvidenceLocations)
	if evidenceCount == 0 && pattern.BusinessMethod != nil && (strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()) != "" || strings.TrimSpace(pattern.BusinessMethod.Function) != "") {
		evidenceCount++
	}
	return evidenceCount
}

func projectSpecGuidanceText(pattern Pattern) string {
	return strings.TrimSpace(firstNonEmpty(pattern.Description, pattern.Rule, pattern.Name))
}

func projectSpecLooksScoped(pattern Pattern) bool {
	if strings.TrimSpace(pattern.ScopePath) != "" || strings.TrimSpace(pattern.AnalysisUnitID) != "" {
		return true
	}
	paths := map[string]bool{}
	for _, location := range pattern.EvidenceLocations {
		path := topLevelEvidencePath(location.Path)
		if path != "" {
			paths[path] = true
		}
	}
	if len(paths) > 1 {
		return false
	}
	if len(paths) == 1 && len(pattern.EvidenceLocations) > 0 {
		return pattern.Frequency < 3 || projectSpecEvidenceCount(pattern) < 3
	}
	return false
}

func topLevelEvidencePath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "`")
	if path == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func uniquePatternEvidenceLocations(locations []PatternEvidenceLocation) []PatternEvidenceLocation {
	seen := make(map[string]bool, len(locations))
	out := make([]PatternEvidenceLocation, 0, len(locations))
	for _, location := range locations {
		key := location.DisplayLocation() + "\x00" + location.Symbol
		if key == "\x00" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, location)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
