package learn

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
)

const relatedDeltaPatternsPerFocus = 8

func (r *learnCurrentProjectRun) analyzeDeltaBatch(ctx context.Context, batch learnCurrentBatch, batchFocuses []analyzer.AnalyzeCurrentEvidenceFocus) ([]learnCurrentFocusResult, error) {
	related, err := r.relatedPatternsByFocus(ctx, batchFocuses)
	if err != nil {
		return nil, err
	}
	deltaFocuses := make([]analyzer.AnalyzeCurrentDeltaFocus, 0, len(batchFocuses))
	for _, focus := range batchFocuses {
		deltaFocuses = append(deltaFocuses, analyzer.AnalyzeCurrentDeltaFocus{
			EvidenceFocus:   focus.EvidenceFocus,
			FocusAbsPaths:   focus.FocusAbsPaths,
			RelatedPatterns: related[focus.EvidenceFocus.ID],
		})
	}

	batchLabel := r.analysisBatchRuntimeLabel(r.analysisState, batch)
	result, err := r.cont.AnalyzerSvc.AnalyzeCurrentDeltaBatch(ctx, r.projectRoot, r.projectName, r.currentLanguage, analyzer.AnalyzeCurrentDeltaBatchOptions{
		RuntimeLabel:      batchLabel,
		LearningMode:      r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
		ChangeProfile:     string(r.changeProfile),
		RunContext:        r.codebaseRunContext,
		SharedContextPath: r.sharedLearningContextPath,
		Focuses:           deltaFocuses,
	})
	if err != nil {
		return nil, err
	}

	return r.buildDeltaFocusResults(batch, batchFocuses, result)
}

func (r *learnCurrentProjectRun) buildDeltaFocusResults(batch learnCurrentBatch, batchFocuses []analyzer.AnalyzeCurrentEvidenceFocus, result *analyzer.AnalyzeCurrentDeltaBatchResult) ([]learnCurrentFocusResult, error) {
	if result == nil {
		result = &analyzer.AnalyzeCurrentDeltaBatchResult{}
	}
	resolver := newDeltaFocusResolver(r.projectRoot, batchFocuses)
	decidedFocuses := make(map[string]bool, len(batchFocuses))
	patternsByFocus := make(map[string][]domain.Pattern, len(batchFocuses))
	refreshByFocus := make(map[string]agent.ProfileRefreshRecommendation, len(batchFocuses))
	if result.ProfileRefreshRecommended.Needed {
		for _, focus := range batchFocuses {
			refreshByFocus[focus.EvidenceFocus.ID] = result.ProfileRefreshRecommended
		}
	}
	for _, change := range result.Changes {
		focus, ok := resolver.resolve(change)
		if !ok {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentDeltaBatchUnknownFocus", map[string]interface{}{"Focus": change.FocusID}))
		}
		change.FocusID = focus.ID
		change.FocusName = focus.Name
		decidedFocuses[focus.ID] = true
		if !change.CarriesPattern() {
			continue
		}
		pattern := *change.Proposal
		patternsByFocus[focus.ID] = append(patternsByFocus[focus.ID], pattern)
	}
	for _, focus := range batchFocuses {
		if !decidedFocuses[focus.EvidenceFocus.ID] {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentDeltaBatchMissedFocus", map[string]interface{}{"Focus": focus.EvidenceFocus.ID}))
		}
	}

	results := make([]learnCurrentFocusResult, 0, len(batch.focuses))
	for _, indexed := range batch.focuses {
		results = append(results, buildAnalyzedFocusResult(indexed.focus, indexed.index, patternsByFocus[indexed.focus.ID], refreshByFocus[indexed.focus.ID]))
	}
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
	return results, nil
}

type deltaFocusResolver struct {
	byID    map[string]domain.EvidenceFocus
	byName  map[string]domain.EvidenceFocus
	byFocus map[string]domain.EvidenceFocus
}

func newDeltaFocusResolver(projectRoot string, focuses []analyzer.AnalyzeCurrentEvidenceFocus) deltaFocusResolver {
	resolver := deltaFocusResolver{
		byID:    make(map[string]domain.EvidenceFocus, len(focuses)),
		byName:  make(map[string]domain.EvidenceFocus, len(focuses)),
		byFocus: make(map[string]domain.EvidenceFocus),
	}
	for _, input := range focuses {
		focus := input.EvidenceFocus
		if focus.ID != "" {
			resolver.byID[focus.ID] = focus
		}
		if focus.Name != "" {
			resolver.byName[focus.Name] = focus
		}
		for _, path := range relativeEvidenceFocusPaths(projectRoot, input.FocusAbsPaths) {
			resolver.byFocus[normalizeStatePath(path)] = focus
		}
		for _, path := range focus.EntryPaths {
			resolver.byFocus[normalizeStatePath(path)] = focus
		}
		for _, path := range focus.RelatedPaths {
			resolver.byFocus[normalizeStatePath(path)] = focus
		}
	}
	return resolver
}

func (r deltaFocusResolver) resolve(change domain.KnowledgeChange) (domain.EvidenceFocus, bool) {
	if focus, ok := r.byID[change.FocusID]; ok {
		return focus, true
	}
	if change.FocusName != "" {
		if focus, ok := r.byName[change.FocusName]; ok {
			return focus, true
		}
	}
	for _, anchor := range change.Anchors {
		if focus, ok := r.byFocus[normalizeStatePath(anchor.Path)]; ok {
			return focus, true
		}
	}
	return domain.EvidenceFocus{}, false
}

func (r *learnCurrentProjectRun) useDeltaAnalysis() bool {
	return r.changeProfile != "" && r.changeProfile != currentChangeProfileInitial
}

func (r *learnCurrentProjectRun) relatedPatternsByFocus(ctx context.Context, focuses []analyzer.AnalyzeCurrentEvidenceFocus) (map[string][]domain.Pattern, error) {
	out := make(map[string][]domain.Pattern, len(focuses))
	if r.cont == nil || r.cont.PatternRepo == nil {
		return out, nil
	}
	patterns, err := r.cont.PatternRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("LearnCurrentLoadRelatedDeltaPatternsFailed"), err)
	}
	for _, focus := range focuses {
		focusPaths := relativeEvidenceFocusPaths(r.projectRoot, focus.FocusAbsPaths)
		out[focus.EvidenceFocus.ID] = selectRelatedDeltaPatterns(focus.EvidenceFocus, focusPaths, patterns, relatedDeltaPatternsPerFocus)
	}
	return out, nil
}

func relativeEvidenceFocusPaths(projectRoot string, absPaths []string) []string {
	paths := make([]string, 0, len(absPaths))
	for _, path := range absPaths {
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			continue
		}
		paths = append(paths, filepath.ToSlash(rel))
	}
	sort.Strings(paths)
	return paths
}

func selectRelatedDeltaPatterns(focus domain.EvidenceFocus, focusPaths []string, patterns []domain.Pattern, limit int) []domain.Pattern {
	type scored struct {
		pattern domain.Pattern
		score   int
	}
	focusSet := pathSet(focusPaths)
	focusTerms := evidenceFocusTerms(focus)
	scoredPatterns := make([]scored, 0, len(patterns))
	for _, pattern := range patterns {
		if !pattern.IsActive() {
			continue
		}
		score := relatedDeltaPatternScore(focus, focusSet, focusTerms, pattern)
		if score <= 0 {
			continue
		}
		scoredPatterns = append(scoredPatterns, scored{pattern: pattern, score: score})
	}
	sort.SliceStable(scoredPatterns, func(i, j int) bool {
		if scoredPatterns[i].score == scoredPatterns[j].score {
			return scoredPatterns[i].pattern.ID < scoredPatterns[j].pattern.ID
		}
		return scoredPatterns[i].score > scoredPatterns[j].score
	})
	if limit > 0 && len(scoredPatterns) > limit {
		scoredPatterns = scoredPatterns[:limit]
	}
	out := make([]domain.Pattern, len(scoredPatterns))
	for i, item := range scoredPatterns {
		out[i] = item.pattern
	}
	return out
}

func relatedDeltaPatternScore(focus domain.EvidenceFocus, focusSet map[string]bool, focusTerms map[string]bool, pattern domain.Pattern) int {
	score := 0
	if pattern.ScopePath != "" && focusSet[normalizeStatePath(pattern.ScopePath)] {
		score += 5
	}
	for _, location := range pattern.EvidenceLocations {
		if focusSet[normalizeStatePath(location.Path)] {
			score += 5
			break
		}
	}
	for token := range patternTextTerms(pattern) {
		if focusTerms[token] {
			score++
		}
	}
	return score
}

func evidenceFocusTerms(focus domain.EvidenceFocus) map[string]bool {
	values := append([]string{focus.ID, focus.Name, focus.ScopeReason}, focus.RouteTerms...)
	values = append(values, focus.EntryPaths...)
	values = append(values, focus.RelatedPaths...)
	return splitRelatedTerms(values...)
}

func patternTextTerms(pattern domain.Pattern) map[string]bool {
	values := []string{pattern.ID, pattern.Name, pattern.Description, pattern.Rule, pattern.ScopePath}
	for _, location := range pattern.EvidenceLocations {
		values = append(values, location.Path, location.Symbol, location.Kind)
	}
	return splitRelatedTerms(values...)
}

func splitRelatedTerms(values ...string) map[string]bool {
	terms := make(map[string]bool)
	for _, value := range values {
		for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= '\u4e00' && r <= '\u9fff')
		}) {
			if token != "" {
				terms[token] = true
			}
		}
	}
	return terms
}
