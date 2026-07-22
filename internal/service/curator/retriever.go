package curator

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type scoredPattern struct {
	pattern domain.Pattern
	score   float64
}

var tokenRE = regexp.MustCompile(`[A-Za-z0-9_\p{Han}]+`)

func retrieveRelatedPatterns(candidates, existing []domain.Pattern, limitPerCandidate int) retrievalResult {
	if limitPerCandidate <= 0 {
		limitPerCandidate = relatedPatternsPerCandidate
	}
	byID := make(map[string]domain.Pattern)
	byCandidate := make(map[string][]string, len(candidates))

	for _, candidate := range candidates {
		scored := make([]scoredPattern, 0, len(existing))
		for _, pattern := range existing {
			score := patternSimilarity(candidate, pattern)
			if score <= 0 {
				continue
			}
			scored = append(scored, scoredPattern{pattern: pattern, score: score})
		}
		sort.SliceStable(scored, func(i, j int) bool {
			if scored[i].score == scored[j].score {
				return scored[i].pattern.ID < scored[j].pattern.ID
			}
			return scored[i].score > scored[j].score
		})
		if len(scored) > limitPerCandidate {
			scored = scored[:limitPerCandidate]
		}
		for _, item := range scored {
			byID[item.pattern.ID] = item.pattern
			byCandidate[candidate.ID] = append(byCandidate[candidate.ID], item.pattern.ID)
		}
	}

	related := make([]domain.Pattern, 0, len(byID))
	for _, pattern := range byID {
		related = append(related, pattern)
	}
	sort.SliceStable(related, func(i, j int) bool {
		return related[i].ID < related[j].ID
	})
	return retrievalResult{
		related:             related,
		existingByCandidate: byCandidate,
	}
}

func patternSimilarity(left, right domain.Pattern) float64 {
	if left.Category != "" && right.Category != "" && left.Category != right.Category {
		return 0
	}

	score := 0.0
	if left.Category != "" && left.Category == right.Category {
		score += 0.2
	}
	if sameScope(left, right) {
		score += 0.12
	}
	if businessMethodOverlap(left.BusinessMethod, right.BusinessMethod) {
		score += 0.2
	}
	score += 0.26 * jaccard(tokens(left.Name+" "+left.Rule), tokens(right.Name+" "+right.Rule))
	score += 0.16 * jaccard(tokens(left.Description), tokens(right.Description))
	score += 0.06 * jaccard(tokens(left.GoodExample), tokens(right.GoodExample))

	if strings.EqualFold(strings.TrimSpace(left.Name), strings.TrimSpace(right.Name)) {
		score += 0.18
	}
	if strings.EqualFold(strings.TrimSpace(left.Rule), strings.TrimSpace(right.Rule)) && strings.TrimSpace(left.Rule) != "" {
		score += 0.18
	}
	if score > 1 {
		return 1
	}
	return score
}

func sameScope(left, right domain.Pattern) bool {
	leftProject := strings.TrimSpace(left.ProjectID)
	leftPath := strings.TrimSpace(left.ScopePath)
	leftRole := strings.TrimSpace(left.WorkspaceRole)
	if leftProject == "" && leftPath == "" && leftRole == "" {
		return false
	}
	return leftProject == strings.TrimSpace(right.ProjectID) &&
		leftPath == strings.TrimSpace(right.ScopePath) &&
		leftRole == strings.TrimSpace(right.WorkspaceRole)
}

func businessMethodOverlap(left, right *domain.BusinessMethod) bool {
	if left == nil || right == nil {
		return false
	}
	if strings.TrimSpace(left.Name) != "" && strings.TrimSpace(left.Name) == strings.TrimSpace(right.Name) {
		return true
	}
	if strings.TrimSpace(left.Function) != "" && strings.TrimSpace(left.Function) == strings.TrimSpace(right.Function) {
		return true
	}
	if left.DisplayLocation() != "" && left.DisplayLocation() == right.DisplayLocation() {
		return true
	}
	return false
}

func tokens(value string) map[string]struct{} {
	matches := tokenRE.FindAllString(strings.ToLower(value), -1)
	result := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		addSemanticTokens(result, match)
	}
	return result
}

func addSemanticTokens(result map[string]struct{}, value string) {
	var word strings.Builder
	var han []rune
	flushWord := func() {
		if word.Len() > 0 {
			result[word.String()] = struct{}{}
			word.Reset()
		}
	}
	flushHan := func() {
		if len(han) == 1 {
			result[string(han)] = struct{}{}
		}
		for i := 0; i+1 < len(han); i++ {
			result[string(han[i:i+2])] = struct{}{}
		}
		han = han[:0]
	}

	for _, r := range strings.ToLower(value) {
		if unicode.Is(unicode.Han, r) {
			flushWord()
			han = append(han, r)
			continue
		}
		flushHan()
		word.WriteRune(r)
	}
	flushWord()
	flushHan()
}

func jaccard(left, right map[string]struct{}) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersection := 0
	for token := range left {
		if _, ok := right[token]; ok {
			intersection++
		}
	}
	union := len(left) + len(right) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
