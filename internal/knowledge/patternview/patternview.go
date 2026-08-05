// Package patternview 提供模式相似度、确定性合并和 skills 输出视图策略。
package patternview

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

const mergeThreshold = 0.82

var tokenRE = regexp.MustCompile(`[A-Za-z0-9_\p{Han}]+`)

// Compact 将候选模式按确定性规则合并为更少的代表模式。
func Compact(candidates, existing []domain.Pattern) []domain.Pattern {
	merger := newMerger(existing, len(candidates))
	for _, candidate := range candidates {
		merger.add(candidate)
	}
	return merger.patterns()
}

// Render 构建供 skills 输出消费的模式视图。
//
// 入库阶段可以保留高召回候选；输出阶段需要在同一 workspace 语义范围内
// 合并同义或高度重叠的候选，避免把原始候选清单直接暴露给 Agent。
func Render(patterns []domain.Pattern) []domain.Pattern {
	if len(patterns) == 0 {
		return nil
	}
	groups := make(map[string][]domain.Pattern)
	var keys []string
	for _, pattern := range patterns {
		key := renderScopeKey(pattern)
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], pattern)
	}

	rendered := make([]domain.Pattern, 0, len(patterns))
	for _, key := range keys {
		rendered = append(rendered, Compact(groups[key], nil)...)
	}
	return rendered
}

// Similarity 返回两个模式的语义相似度。跨 workspace 语义范围或高风险边界不同的
// 模式直接视为不可合并。
func Similarity(left, right domain.Pattern) float64 {
	if left.Category != "" && right.Category != "" && left.Category != right.Category {
		return 0
	}
	if !compatibleScope(left, right) {
		return 0
	}
	if domain.IsHighRiskOperationalPattern(left) != domain.IsHighRiskOperationalPattern(right) {
		return 0
	}

	score := 0.0
	if left.Category != "" && left.Category == right.Category {
		score += 0.2
	}
	if SameScope(left, right) {
		score += 0.12
	}
	if businessMethodOverlap(left.BusinessMethod, right.BusinessMethod) {
		score += 0.2
	}
	sharedEvidence := sharedEvidenceCount(left, right)
	evidenceOverlap := evidenceOverlap(left, right)
	score += 0.24 * evidenceOverlap
	if sharedEvidence >= 2 {
		score += 0.16
		if score < mergeThreshold {
			score = mergeThreshold
		}
	}
	score += 0.26 * jaccard(Tokens(left.Name+" "+left.Rule), Tokens(right.Name+" "+right.Rule))
	score += 0.16 * jaccard(Tokens(left.Description), Tokens(right.Description))
	score += 0.06 * jaccard(Tokens(left.GoodExample), Tokens(right.GoodExample))

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

// SameScope 判断两个模式是否属于完全相同的显式 workspace 语义范围。
func SameScope(left, right domain.Pattern) bool {
	if !hasScope(left) {
		return false
	}
	return strings.TrimSpace(left.ProjectID) == strings.TrimSpace(right.ProjectID) &&
		strings.TrimSpace(left.ScopePath) == strings.TrimSpace(right.ScopePath) &&
		strings.TrimSpace(left.WorkspaceRole) == strings.TrimSpace(right.WorkspaceRole)
}

// Tokens 返回用于相似度比较的语言无关 token 集。
func Tokens(value string) map[string]struct{} {
	matches := tokenRE.FindAllString(strings.ToLower(value), -1)
	result := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		addSemanticTokens(result, match)
	}
	return result
}

// MergeKeepingBest 合并两个模式，并保留质量更高的模式作为代表。
func MergeKeepingBest(left, right domain.Pattern) domain.Pattern {
	left = Normalize(left)
	right = Normalize(right)
	primary, secondary := left, right
	if qualityScore(right) > qualityScore(left) {
		primary, secondary = right, left
	}
	primary.Merge(&secondary)
	primary.Merged = true
	primary.MergedFrom = mergedSources(left, right)
	primary.ProjectID = firstNonBlank(primary.ProjectID, secondary.ProjectID)
	primary.ScopePath = firstNonBlank(primary.ScopePath, secondary.ScopePath)
	primary.WorkspaceRole = firstNonBlank(primary.WorkspaceRole, secondary.WorkspaceRole)
	if primary.Source == "" {
		primary.Source = secondary.Source
	}
	if primary.BusinessMethod == nil {
		primary.BusinessMethod = secondary.BusinessMethod
	}
	primary.EvidenceLocations = mergeEvidenceLocations(primary.EvidenceLocations, secondary.EvidenceLocations)
	primary.RefreshMetrics()
	return primary
}

// WithSources 返回带来源 ID 的规范模式副本。
func WithSources(pattern domain.Pattern, mergedFrom []string) domain.Pattern {
	pattern = Normalize(pattern)
	pattern.MergedFrom = stringx.UniqueNonEmpty(mergedFrom)
	pattern.Merged = len(pattern.MergedFrom) > 1
	pattern.BusinessMethod = cloneBusinessMethod(pattern.BusinessMethod)
	pattern.EvidenceLocations = append([]domain.PatternEvidenceLocation(nil), pattern.EvidenceLocations...)
	return pattern
}

// SourceIDs 返回模式的来源 ID 列表。
func SourceIDs(pattern domain.Pattern) []string {
	if len(pattern.MergedFrom) > 0 {
		return pattern.MergedFrom
	}
	if pattern.ID == "" {
		return nil
	}
	return []string{pattern.ID}
}

// Normalize 补齐模式合并前需要的稳定字段。
func Normalize(pattern domain.Pattern) domain.Pattern {
	pattern.ID = strings.TrimSpace(pattern.ID)
	pattern.Name = strings.TrimSpace(pattern.Name)
	pattern.Description = strings.TrimSpace(pattern.Description)
	pattern.Rule = strings.TrimSpace(pattern.Rule)
	pattern.GoodExample = strings.TrimSpace(pattern.GoodExample)
	pattern.BadExample = strings.TrimSpace(pattern.BadExample)
	pattern.ProjectID = strings.TrimSpace(pattern.ProjectID)
	pattern.ScopePath = strings.TrimSpace(pattern.ScopePath)
	pattern.WorkspaceRole = strings.TrimSpace(pattern.WorkspaceRole)
	pattern.Category = domain.NormalizePatternCategory(pattern.Category)
	if pattern.Source == "" {
		pattern.Source = domain.SourceLearned
	}
	if pattern.Frequency <= 0 {
		pattern.Frequency = 1
	}
	return pattern
}

type merger struct {
	accepted   []domain.Pattern
	indexByID  map[string]int
	indexByKey map[string]int
	output     map[string]domain.Pattern
}

func newMerger(existing []domain.Pattern, candidateCount int) *merger {
	merger := &merger{
		accepted:   make([]domain.Pattern, 0, len(existing)+candidateCount),
		indexByID:  make(map[string]int, len(existing)+candidateCount),
		indexByKey: make(map[string]int, len(existing)+candidateCount),
		output:     make(map[string]domain.Pattern, candidateCount),
	}
	for _, pattern := range existing {
		merger.upsertAccepted(Normalize(pattern))
	}
	return merger
}

func (m *merger) add(candidate domain.Pattern) {
	candidate = Normalize(candidate)
	if index, ok := m.acceptedIndex(candidate); ok {
		m.recordMerged(index, candidate)
		return
	}
	bestIndex, bestScore := m.bestMatch(candidate)
	if bestIndex >= 0 && bestScore >= mergeThreshold {
		m.recordMerged(bestIndex, candidate)
		return
	}
	m.appendAccepted(candidate)
	m.output[patternKey(candidate)] = WithSources(candidate, SourceIDs(candidate))
}

func (m *merger) upsertAccepted(pattern domain.Pattern) {
	if index, ok := m.acceptedIndex(pattern); ok {
		m.replaceAccepted(index, MergeKeepingBest(m.accepted[index], pattern))
		return
	}
	m.appendAccepted(pattern)
}

func (m *merger) recordMerged(index int, candidate domain.Pattern) {
	previous := m.accepted[index]
	merged := MergeKeepingBest(previous, candidate)
	m.replaceAccepted(index, merged)
	m.removeMergedOutputs(merged.MergedFrom, merged.ID)
	delete(m.output, patternKey(previous))
	delete(m.output, patternKey(candidate))
	m.output[patternKey(merged)] = WithSources(merged, merged.MergedFrom)
}

func (m *merger) appendAccepted(pattern domain.Pattern) {
	index := len(m.accepted)
	m.indexAccepted(pattern, index)
	m.accepted = append(m.accepted, pattern)
}

func (m *merger) replaceAccepted(index int, pattern domain.Pattern) {
	m.removeAcceptedIndex(m.accepted[index])
	m.accepted[index] = pattern
	m.indexAccepted(pattern, index)
}

func (m *merger) acceptedIndex(pattern domain.Pattern) (int, bool) {
	if pattern.ID != "" {
		index, ok := m.indexByID[pattern.ID]
		return index, ok
	}
	index, ok := m.indexByKey[patternKey(pattern)]
	return index, ok
}

func (m *merger) indexAccepted(pattern domain.Pattern, index int) {
	if pattern.ID != "" {
		m.indexByID[pattern.ID] = index
		return
	}
	m.indexByKey[patternKey(pattern)] = index
}

func (m *merger) removeAcceptedIndex(pattern domain.Pattern) {
	if pattern.ID != "" {
		delete(m.indexByID, pattern.ID)
		return
	}
	delete(m.indexByKey, patternKey(pattern))
}

func (m *merger) bestMatch(candidate domain.Pattern) (int, float64) {
	bestIndex := -1
	bestScore := 0.0
	for i := range m.accepted {
		if candidate.ID != "" && m.accepted[i].ID == candidate.ID {
			continue
		}
		score := Similarity(candidate, m.accepted[i])
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	return bestIndex, bestScore
}

func (m *merger) removeMergedOutputs(mergedFrom []string, keepID string) {
	for _, id := range mergedFrom {
		if id != keepID {
			delete(m.output, id)
		}
	}
}

func (m *merger) patterns() []domain.Pattern {
	result := make([]domain.Pattern, 0, len(m.output))
	for _, pattern := range m.accepted {
		if normalized, ok := m.output[patternKey(pattern)]; ok {
			result = append(result, normalized)
		}
	}
	return result
}

func patternKey(pattern domain.Pattern) string {
	if pattern.ID != "" {
		return pattern.ID
	}
	return strings.Join([]string{
		string(pattern.Category),
		pattern.ProjectID,
		pattern.ScopePath,
		pattern.WorkspaceRole,
		strings.ToLower(pattern.Name),
		strings.ToLower(pattern.Rule),
	}, "\x00")
}

func renderScopeKey(pattern domain.Pattern) string {
	pattern = Normalize(pattern)
	return strings.Join([]string{
		string(pattern.Category),
		pattern.ProjectID,
		pattern.ScopePath,
		pattern.WorkspaceRole,
	}, "\x00")
}

func compatibleScope(left, right domain.Pattern) bool {
	if !hasScope(left) && !hasScope(right) {
		return true
	}
	return strings.TrimSpace(left.ProjectID) == strings.TrimSpace(right.ProjectID) &&
		strings.TrimSpace(left.ScopePath) == strings.TrimSpace(right.ScopePath) &&
		strings.TrimSpace(left.WorkspaceRole) == strings.TrimSpace(right.WorkspaceRole)
}

func hasScope(pattern domain.Pattern) bool {
	return strings.TrimSpace(pattern.ProjectID) != "" ||
		strings.TrimSpace(pattern.ScopePath) != "" ||
		strings.TrimSpace(pattern.WorkspaceRole) != ""
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

func evidenceOverlap(left, right domain.Pattern) float64 {
	leftKeys := evidenceKeys(left)
	rightKeys := evidenceKeys(right)
	return jaccard(leftKeys, rightKeys)
}

func sharedEvidenceCount(left, right domain.Pattern) int {
	leftKeys := evidenceKeys(left)
	rightKeys := evidenceKeys(right)
	count := 0
	for key := range leftKeys {
		if _, ok := rightKeys[key]; ok {
			count++
		}
	}
	return count
}

func evidenceKeys(pattern domain.Pattern) map[string]struct{} {
	keys := make(map[string]struct{}, len(pattern.EvidenceLocations)+1)
	for _, location := range pattern.EvidenceLocations {
		key := evidenceKey(location)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	if pattern.BusinessMethod != nil {
		if location := strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()); location != "" {
			keys[strings.ToLower(location)] = struct{}{}
		}
	}
	return keys
}

func evidenceKey(location domain.PatternEvidenceLocation) string {
	path := strings.ToLower(strings.TrimSpace(location.Path))
	symbol := strings.ToLower(strings.TrimSpace(location.Symbol))
	kind := strings.ToLower(strings.TrimSpace(location.Kind))
	if path == "" {
		return ""
	}
	if symbol != "" {
		return path + "|" + symbol + "|" + kind
	}
	if display := strings.ToLower(strings.TrimSpace(location.DisplayLocation())); display != "" {
		return display
	}
	return path
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

func qualityScore(pattern domain.Pattern) float64 {
	pattern.RefreshMetrics()
	score := pattern.Metrics.EffectiveScore
	if score == 0 {
		score = pattern.Confidence
	}
	score += float64(pattern.Frequency) * 0.01
	if pattern.BusinessMethod != nil {
		score += 0.05
	}
	if strings.TrimSpace(pattern.GoodExample) != "" {
		score += 0.04
	}
	if len(pattern.EvidenceLocations) > 0 {
		score += 0.03
	}
	return score
}

func mergedSources(left, right domain.Pattern) []string {
	values := make([]string, 0, len(left.MergedFrom)+len(right.MergedFrom)+2)
	values = append(values, SourceIDs(left)...)
	values = append(values, SourceIDs(right)...)
	return stringx.UniqueNonEmpty(values)
}

func mergeEvidenceLocations(left, right []domain.PatternEvidenceLocation) []domain.PatternEvidenceLocation {
	out := make([]domain.PatternEvidenceLocation, 0, len(left)+len(right))
	seen := map[string]struct{}{}
	add := func(loc domain.PatternEvidenceLocation) {
		key := loc.DisplayLocation() + "|" + loc.Symbol + "|" + loc.Kind
		if key == "||" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, loc)
	}
	for _, loc := range left {
		add(loc)
	}
	for _, loc := range right {
		add(loc)
	}
	return out
}

func cloneBusinessMethod(method *domain.BusinessMethod) *domain.BusinessMethod {
	if method == nil {
		return nil
	}
	cloned := *method
	return &cloned
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
