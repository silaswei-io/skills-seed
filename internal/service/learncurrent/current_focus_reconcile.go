package learncurrent

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// reconcileEvidenceFocuses 只规范化 Agent 返回的执行计划，不推断或重组业务语义。
func reconcileEvidenceFocuses(focuses []domain.EvidenceFocus, allowedPaths []string) []domain.EvidenceFocus {
	allowedPaths = normalizeStatePaths(allowedPaths)
	allowed := pathSet(allowedPaths)
	normalized := make([]domain.EvidenceFocus, 0, len(focuses))
	for _, focus := range focuses {
		focus = normalizeEvidenceFocusPolicy(focus)
		focus.EntryPaths = filterEvidenceFocusPaths(focus.EntryPaths, allowed, nil)
		entryPaths := pathSet(focus.EntryPaths)
		focus.RelatedPaths = filterEvidenceFocusPaths(focus.RelatedPaths, allowed, entryPaths)
		if len(focus.EntryPaths)+len(focus.RelatedPaths) == 0 {
			continue
		}
		normalized = append(normalized, focus)
	}
	return normalized
}

func normalizeEvidenceFocusPolicy(focus domain.EvidenceFocus) domain.EvidenceFocus {
	focus.Attributes = cleanEvidenceFocusLabels(focus.Attributes)
	focus.RiskSignals = cleanEvidenceFocusLabels(focus.RiskSignals)
	if focus.AnalysisDepth != "" {
		focus.AnalysisDepth = focus.EffectiveAnalysisDepth()
	}
	return focus
}

func cleanEvidenceFocusLabels(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func filterEvidenceFocusPaths(paths []string, allowed, excluded map[string]bool) []string {
	filtered := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = normalizeStatePath(path)
		if path == "" || !allowed[path] || excluded[path] || seen[path] {
			continue
		}
		seen[path] = true
		filtered = append(filtered, path)
	}
	sort.Strings(filtered)
	return filtered
}

func uncoveredAnalysisPaths(focuses []domain.EvidenceFocus, paths []string) []string {
	covered := make(map[string]bool)
	for _, focus := range focuses {
		for _, path := range append(append([]string{}, focus.EntryPaths...), focus.RelatedPaths...) {
			covered[normalizeStatePath(path)] = true
		}
	}
	paths = normalizeStatePaths(paths)
	uncovered := make([]string, 0)
	for _, path := range paths {
		if !covered[path] {
			uncovered = append(uncovered, path)
		}
	}
	return uncovered
}
