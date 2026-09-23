package learncurrent

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge/patternview"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
)

func (r *learnCurrentProjectRun) buildDeltaFocusResults(batch learnCurrentBatch, batchFocuses []analyzer.AnalyzeCurrentEvidenceFocus, related map[string][]domain.Pattern, result *analyzer.AnalyzeCurrentDeltaBatchResult) ([]learnCurrentFocusResult, error) {
	if result == nil {
		result = &analyzer.AnalyzeCurrentDeltaBatchResult{}
	}
	resolver := newDeltaFocusResolver(r.projectRoot, batchFocuses)
	decidedFocuses := make(map[string]bool, len(batchFocuses))
	patternsByFocus := make(map[string][]domain.Pattern, len(batchFocuses))
	retiredByFocus := make(map[string][]string, len(batchFocuses))
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
		if change.PatternAction == domain.KnowledgePatternRetire {
			if relatedPatternCanBeRetired(related[focus.ID], change.PatternID) {
				retiredByFocus[focus.ID] = append(retiredByFocus[focus.ID], change.PatternID)
			}
			continue
		}
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
		focusResult := buildAnalyzedFocusResult(indexed.focus, indexed.index, patternsByFocus[indexed.focus.ID], refreshByFocus[indexed.focus.ID], agent.Conversation{})
		focusResult.evidence = result.Evidence[indexed.focus.ID].Clone()
		focusResult.retiredPatternIDs = appendUniquePatternIDs(nil, retiredByFocus[indexed.focus.ID]...)
		results = append(results, focusResult)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
	return results, nil
}

func relatedPatternCanBeRetired(patterns []domain.Pattern, patternID string) bool {
	patternID = strings.TrimSpace(patternID)
	if patternID == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern.ID == patternID && pattern.CanBeRetiredFromCurrentLearning() {
			return true
		}
	}
	return false
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
		out[focus.EvidenceFocus.ID] = selectRelatedDeltaPatterns(focus.EvidenceFocus, focusPaths, patterns)
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

func selectRelatedDeltaPatterns(focus domain.EvidenceFocus, focusPaths []string, patterns []domain.Pattern) []domain.Pattern {
	relatedPaths := append([]string{}, focusPaths...)
	relatedPaths = append(relatedPaths, focus.EntryPaths...)
	relatedPaths = append(relatedPaths, focus.RelatedPaths...)
	out := make([]domain.Pattern, 0)
	for _, pattern := range patterns {
		if !pattern.IsActive() || !patternview.RelatedToPaths(pattern, relatedPaths) {
			continue
		}
		out = append(out, pattern)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
