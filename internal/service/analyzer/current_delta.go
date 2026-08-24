package analyzer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/repositoryscopeconfig"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

func (s *AnalyzerService) AnalyzeCurrentDeltaBatch(ctx context.Context, projectRoot, projectName, language string, opts AnalyzeCurrentDeltaBatchOptions) (*AnalyzeCurrentDeltaBatchResult, error) {
	startedAt := time.Now()
	runContext := opts.RunContext
	if runContext == nil {
		var err error
		runContext, err = s.BuildCodebaseRunContext(ctx, projectRoot, language, AnalyzeCodebaseOptions{UseSnapshotDiffs: true})
		if err != nil {
			return nil, err
		}
	}

	focuses := make([]agent.AnalyzeCurrentDeltaFocus, 0, len(opts.Focuses))
	focusByID := make(map[string]map[string]bool, len(opts.Focuses))
	relatedByFocus := make(map[string]map[string]domain.Pattern, len(opts.Focuses))
	for _, focus := range opts.Focuses {
		focusPaths := projectpath.Relative(projectRoot, focus.FocusAbsPaths)
		focuses = append(focuses, agent.AnalyzeCurrentDeltaFocus{
			EvidenceFocus:   focus.EvidenceFocus,
			FocusPaths:      focusPaths,
			ContextFiles:    filterSampleFilesByFocus(runContext.SampleFiles, focusPaths),
			DiffFiles:       filterDiffFilesByFocus(runContext.DiffFiles, focusPaths),
			RelatedPatterns: append([]domain.Pattern(nil), focus.RelatedPatterns...),
		})
		focusByID[focus.EvidenceFocus.ID] = relPathSet(focusPaths)
		relatedByFocus[focus.EvidenceFocus.ID] = relatedPatternIndex(focus.RelatedPatterns)
	}

	focusPaths := batchDeltaFocusPaths(focuses)
	guidance, err := s.loadMaintainedGuidance()
	if err != nil {
		return nil, fmt.Errorf("load user-maintained learning guidance: %w", err)
	}
	agentReq := &agent.AnalyzeCurrentDeltaBatchRequest{
		ProjectName:        projectName,
		RootPath:           projectRoot,
		Language:           language,
		LearningMode:       opts.LearningMode,
		RuntimeLabel:       opts.RuntimeLabel,
		SharedContextPath:  opts.SharedContextPath,
		Focuses:            focuses,
		Structure:          focusedStructure(focusPaths),
		UserContext:        runtimecontext.UserContext(ctx),
		MaintainedGuidance: guidance,
		ChangeProfile:      opts.ChangeProfile,
	}

	structuralContext, err := s.collectStructuralContext(ctx, projectRoot, structuralContextRequest{
		ProjectName: projectName,
		Language:    language,
		Purpose:     "diff anchored current codebase delta analysis",
		FocusPaths:  focusPaths,
		SeedPaths:   batchDeltaSeedPaths(focuses),
	})
	if err != nil {
		return nil, err
	}
	agentReq.StructuralContext = structuralContext

	result, err := s.agent.AnalyzeCurrentDeltaBatch(ctx, agentReq)
	if err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_current_delta_batch",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}
	if err := agent.RequireResult(result, "AnalyzeCurrentDeltaBatch"); err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}

	changes, err := s.validateDeltaChanges(ctx, projectRoot, result.Changes, focusByID, relatedByFocus)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_current_delta_batch",
		"duration", time.Since(startedAt),
		"changes_count", len(changes),
		"profile_refresh_recommended", result.ProfileRefreshRecommended.Needed,
	)
	return &AnalyzeCurrentDeltaBatchResult{
		Changes:                   changes,
		ProfileRefreshRecommended: result.ProfileRefreshRecommended,
		Conversation:              result.Conversation,
	}, nil
}

func (s *AnalyzerService) validateDeltaChanges(ctx context.Context, projectRoot string, changes []domain.KnowledgeChange, focusByID map[string]map[string]bool, relatedByFocus map[string]map[string]domain.Pattern) ([]domain.KnowledgeChange, error) {
	proposals := make([]domain.Pattern, 0)
	for _, change := range changes {
		if !change.CarriesPattern() || !deltaChangeAnchored(change, focusByID) {
			continue
		}
		proposals = append(proposals, *change.Proposal)
	}
	validByID := make(map[string]domain.Pattern, len(proposals))
	if len(proposals) > 0 {
		validator, err := newCurrentPatternValidator(ctx, projectRoot, proposals, s.symbolResolver)
		if err != nil {
			return nil, err
		}
		for _, pattern := range validator.validatePatterns(proposals) {
			validByID[pattern.ID] = pattern
		}
	}

	retirementTargets := retirementTargets(changes, relatedByFocus)
	var retirementValidator *currentPatternValidator
	if len(retirementTargets) > 0 {
		var err error
		retirementValidator, err = newCurrentPatternValidator(ctx, projectRoot, retirementTargets, s.symbolResolver)
		if err != nil {
			return nil, err
		}
	}
	scope := repositoryscopeconfig.KnowledgeScope(s.configRepo, projectRoot)

	validated := make([]domain.KnowledgeChange, 0, len(changes))
	for _, change := range changes {
		if change.PatternAction == domain.KnowledgePatternRetire {
			pattern, ok := relatedByFocus[change.FocusID][strings.TrimSpace(change.PatternID)]
			if !ok ||
				!pattern.CanBeRetiredFromCurrentLearning() ||
				!deltaChangeAnchored(change, focusByID) ||
				!anchorsTouchPatternEvidence(change.Anchors, pattern.EvidenceLocations) ||
				retirementValidator == nil ||
				retirementValidator.hasLiveEvidence(pattern.EvidenceLocations, scope) {
				continue
			}
			validated = append(validated, change)
			continue
		}
		if !change.CarriesPattern() {
			if deltaChangeScoped(change, focusByID) {
				validated = append(validated, change)
			}
			continue
		}
		if !deltaChangeAnchored(change, focusByID) {
			continue
		}
		pattern, ok := validByID[change.Proposal.ID]
		if !ok {
			continue
		}
		pattern.DiffAnchors = append([]domain.PatternDiffAnchor(nil), change.Anchors...)
		change.Proposal = &pattern
		validated = append(validated, change)
	}
	return validated, nil
}

func relatedPatternIndex(patterns []domain.Pattern) map[string]domain.Pattern {
	index := make(map[string]domain.Pattern, len(patterns))
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern.ID) != "" {
			index[pattern.ID] = pattern
		}
	}
	return index
}

func retirementTargets(changes []domain.KnowledgeChange, relatedByFocus map[string]map[string]domain.Pattern) []domain.Pattern {
	seen := make(map[string]bool)
	targets := make([]domain.Pattern, 0)
	for _, change := range changes {
		if change.PatternAction != domain.KnowledgePatternRetire {
			continue
		}
		pattern, ok := relatedByFocus[change.FocusID][strings.TrimSpace(change.PatternID)]
		if !ok || !pattern.CanBeRetiredFromCurrentLearning() || seen[pattern.ID] {
			continue
		}
		seen[pattern.ID] = true
		targets = append(targets, pattern)
	}
	return targets
}

func anchorsTouchPatternEvidence(anchors []domain.PatternDiffAnchor, locations []domain.PatternEvidenceLocation) bool {
	paths := make(map[string]bool, len(locations))
	for _, location := range locations {
		path := normalizeRelPath(location.Path)
		if path != "" {
			paths[path] = true
		}
	}
	for _, anchor := range anchors {
		if paths[normalizeRelPath(anchor.Path)] {
			return true
		}
	}
	return false
}

func deltaChangeScoped(change domain.KnowledgeChange, focusByID map[string]map[string]bool) bool {
	if _, ok := focusByID[change.FocusID]; ok {
		return true
	}
	return deltaChangeAnchored(change, focusByID)
}

func deltaChangeAnchored(change domain.KnowledgeChange, focusByID map[string]map[string]bool) bool {
	if len(change.Anchors) == 0 {
		return false
	}
	allowed := focusByID[change.FocusID]
	if len(allowed) == 0 && len(focusByID) == 1 {
		for _, paths := range focusByID {
			allowed = paths
		}
	}
	for _, anchor := range change.Anchors {
		path := normalizeRelPath(anchor.Path)
		if path != "" && allowed[path] {
			return true
		}
		if path != "" && len(allowed) == 0 && focusContainsPath(focusByID, path) {
			return true
		}
	}
	return false
}

func focusContainsPath(focusByID map[string]map[string]bool, path string) bool {
	for _, paths := range focusByID {
		if paths[path] {
			return true
		}
	}
	return false
}

func batchDeltaFocusPaths(focuses []agent.AnalyzeCurrentDeltaFocus) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, focus := range focuses {
		for _, path := range focus.FocusPaths {
			path = normalizeRelPath(path)
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func batchDeltaSeedPaths(focuses []agent.AnalyzeCurrentDeltaFocus) []string {
	seen := make(map[string]bool)
	var paths []string
	add := func(path string) {
		path = normalizeRelPath(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, focus := range focuses {
		for _, path := range focus.FocusPaths {
			add(path)
		}
		for _, file := range focus.ContextFiles {
			add(file.Path)
		}
		for _, file := range focus.DiffFiles {
			add(file.Path)
		}
		for _, pattern := range focus.RelatedPatterns {
			for _, location := range pattern.EvidenceLocations {
				add(location.Path)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func relPathSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = normalizeRelPath(path)
		if path != "" {
			set[path] = true
		}
	}
	return set
}
