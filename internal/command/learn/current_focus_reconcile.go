package learn

import (
	"sort"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// reconcileEvidenceFocuses 把 AI 计划收敛为抽象焦点优先、且覆盖候选集合的执行计划。
func reconcileEvidenceFocuses(focuses []domain.EvidenceFocus, allowedPaths []string) []domain.EvidenceFocus {
	allowedPaths = normalizeStatePaths(allowedPaths)
	allowed := pathSet(allowedPaths)
	normalized := make([]domain.EvidenceFocus, 0, len(focuses)+1)
	for _, focus := range focuses {
		focus.EntryPaths = filterEvidenceFocusPaths(focus.EntryPaths, allowed, nil)
		entryPaths := pathSet(focus.EntryPaths)
		focus.RelatedPaths = filterEvidenceFocusPaths(focus.RelatedPaths, allowed, entryPaths)
		if len(focus.EntryPaths)+len(focus.RelatedPaths) == 0 {
			continue
		}
		normalized = append(normalized, focus)
	}
	normalized = mergeSupportEvidenceFocuses(normalized)

	uncovered := uncoveredAnalysisPaths(normalized, allowedPaths)
	normalized, uncovered = attachUncoveredToExistingFocuses(normalized, uncovered)
	if len(uncovered) == 1 {
		fallback := fallbackEvidenceFocus(uncovered)
		fallback.ID = uniqueEvidenceFocusID(fallback.ID, normalized)
		normalized = append(normalized, fallback)
	}
	if len(uncovered) > 1 {
		normalized = append(normalized, groupedFallbackEvidenceFocuses(uncovered, normalized)...)
	}
	return normalized
}

func fallbackEvidenceFocus(paths []string) domain.EvidenceFocus {
	return fallbackEvidenceFocusForGroup("", paths)
}

func fallbackEvidenceFocusForGroup(group string, paths []string) domain.EvidenceFocus {
	name := i18n.Get("LearnCurrentFallbackFocusName")
	reason := i18n.Get("LearnCurrentFallbackFocusReason")
	if strings.TrimSpace(group) != "" {
		params := map[string]interface{}{"Group": group}
		name = i18n.GetWithParams("LearnCurrentFallbackFocusNameWithGroup", params)
		reason = i18n.GetWithParams("LearnCurrentFallbackFocusReasonWithGroup", params)
	}
	return domain.EvidenceFocus{
		ID:           "current-codebase",
		Name:         name,
		RouteTerms:   []string{i18n.Get("LearnCurrentFallbackFocusRouteChange"), i18n.Get("LearnCurrentFallbackFocusRouteLearning")},
		EntryPaths:   normalizeStatePaths(paths),
		RelatedPaths: nil,
		ScopeReason:  reason,
	}
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

func uniqueEvidenceFocusID(id string, focuses []domain.EvidenceFocus) string {
	used := make(map[string]bool, len(focuses))
	for _, focus := range focuses {
		used[focus.ID] = true
	}
	if !used[id] {
		return id
	}
	for suffix := 2; ; suffix++ {
		candidate := id + "-" + strconv.Itoa(suffix)
		if !used[candidate] {
			return candidate
		}
	}
}

func mergeSupportEvidenceFocuses(focuses []domain.EvidenceFocus) []domain.EvidenceFocus {
	out := make([]domain.EvidenceFocus, 0, len(focuses))
	for _, focus := range focuses {
		if !supportEvidenceFocus(focus) || len(out) == 0 {
			out = append(out, focus)
			continue
		}
		index := bestEvidenceFocusIndex(out, evidenceFocusAllPaths(focus))
		if index < 0 {
			out = append(out, focus)
			continue
		}
		out[index].RelatedPaths = mergeStatePaths(out[index].RelatedPaths, append(focus.EntryPaths, focus.RelatedPaths...))
	}
	return out
}

func supportEvidenceFocus(focus domain.EvidenceFocus) bool {
	if len(focus.EntryPaths)+len(focus.RelatedPaths) > 3 {
		return false
	}
	text := strings.ToLower(strings.Join(append([]string{focus.ID, focus.Name}, focus.RouteTerms...), " "))
	for _, term := range []string{"version", "版本", "i18n", "国际化", "configuration", "config", "配置", "constant", "常量", "types", "类型", "common", "通用"} {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func attachUncoveredToExistingFocuses(focuses []domain.EvidenceFocus, paths []string) ([]domain.EvidenceFocus, []string) {
	var remaining []string
	for _, path := range paths {
		index := bestEvidenceFocusIndex(focuses, []string{path})
		if index < 0 {
			remaining = append(remaining, path)
			continue
		}
		focuses[index].RelatedPaths = mergeStatePaths(focuses[index].RelatedPaths, []string{path})
	}
	return focuses, remaining
}

func bestEvidenceFocusIndex(focuses []domain.EvidenceFocus, paths []string) int {
	bestIndex := -1
	bestScore := 0
	for i, focus := range focuses {
		score := evidenceFocusPathScore(focus, paths)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestScore < 2 {
		return -1
	}
	return bestIndex
}

func evidenceFocusPathScore(focus domain.EvidenceFocus, paths []string) int {
	score := 0
	focusPaths := evidenceFocusAllPaths(focus)
	for _, path := range paths {
		for _, focusPath := range focusPaths {
			if samePathFamily(path, focusPath) {
				score += 3
			}
		}
		for _, word := range pathWords(path) {
			if evidenceFocusHasWord(focus, word) {
				score++
			}
		}
	}
	return score
}

func evidenceFocusAllPaths(focus domain.EvidenceFocus) []string {
	return append(append([]string{}, focus.EntryPaths...), focus.RelatedPaths...)
}

func evidenceFocusHasWord(focus domain.EvidenceFocus, word string) bool {
	if len(word) < 3 {
		return false
	}
	text := strings.ToLower(strings.Join(append([]string{focus.ID, focus.Name}, append(focus.RouteTerms, evidenceFocusAllPaths(focus)...)...), " "))
	return strings.Contains(text, strings.ToLower(word))
}

func samePathFamily(left, right string) bool {
	leftParts := strings.Split(normalizeStatePath(left), "/")
	rightParts := strings.Split(normalizeStatePath(right), "/")
	if len(leftParts) < 2 || len(rightParts) < 2 || leftParts[0] != rightParts[0] {
		return false
	}
	if leftParts[0] == "plugins" && len(leftParts) > 2 && len(rightParts) > 2 {
		return leftParts[1] == rightParts[1] && leftParts[2] == rightParts[2]
	}
	return leftParts[1] == rightParts[1]
}

func groupedFallbackEvidenceFocuses(paths []string, existing []domain.EvidenceFocus) []domain.EvidenceFocus {
	groups := make(map[string][]string)
	for _, path := range normalizeStatePaths(paths) {
		key := fallbackGroupKey(path)
		groups[key] = append(groups[key], path)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]domain.EvidenceFocus, 0, len(keys))
	used := append([]domain.EvidenceFocus(nil), existing...)
	for _, key := range keys {
		focus := fallbackEvidenceFocusForGroup(key, groups[key])
		id := "current-codebase"
		if key != "" {
			id += "-" + pathIDPart(key)
		}
		focus.ID = uniqueEvidenceFocusID(id, used)
		out = append(out, focus)
		used = append(used, focus)
	}
	return out
}

func fallbackGroupKey(path string) string {
	parts := strings.Split(normalizeStatePath(path), "/")
	switch {
	case len(parts) <= 1:
		return ""
	case len(parts) >= 6 && parts[0] == "plugins" && parts[2] == "internal" && learningPathLayer(parts[3]):
		return strings.Join(parts[:5], "/")
	case len(parts) >= 4 && parts[0] == "plugins":
		return strings.Join(parts[:4], "/")
	case len(parts) >= 3 && parts[0] == "desc" && parts[1] == "api":
		return strings.Join(parts[:3], "/")
	case len(parts) >= 2 && parts[0] == "desc":
		return strings.Join(parts[:2], "/")
	case len(parts) >= 4 && parts[0] == "internal" && learningPathLayer(parts[1]):
		return strings.Join(parts[:3], "/")
	case parts[0] == "internal":
		return ""
	case len(parts) >= 2:
		return strings.Join(parts[:2], "/")
	default:
		return ""
	}
}

func learningPathLayer(part string) bool {
	switch part {
	case "handler", "logic", "service", "types", "model", "middleware", "helper":
		return true
	default:
		return false
	}
}

func pathIDPart(path string) string {
	id := strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-").Replace(strings.ToLower(path))
	id = strings.Trim(id, "-")
	if id == "" {
		return "misc"
	}
	return id
}

func pathWords(path string) []string {
	normalized := strings.NewReplacer("/", " ", "-", " ", "_", " ", ".", " ").Replace(strings.ToLower(path))
	words := strings.Fields(normalized)
	out := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) >= 3 {
			out = append(out, word)
		}
	}
	return out
}

func mergeStatePaths(left, right []string) []string {
	seen := pathSet(left)
	for _, path := range normalizeStatePaths(right) {
		seen[path] = true
	}
	return sortedBoolPaths(seen)
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
