package analyzer

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
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
	}

	focusPaths := batchDeltaFocusPaths(focuses)
	agentReq := &agent.AnalyzeCurrentDeltaBatchRequest{
		ProjectName:   projectName,
		RootPath:      projectRoot,
		Language:      language,
		LearningMode:  opts.LearningMode,
		RuntimeLabel:  opts.RuntimeLabel,
		Focuses:       focuses,
		Structure:     focusedStructure(focusPaths),
		UserContext:   runtimecontext.UserContext(ctx),
		ChangeProfile: opts.ChangeProfile,
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

	if opts.LearningSession == nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), fmt.Errorf("%s", i18n.Get("AnalyzerLearningSessionRequiredForDelta")))
	}
	result, err := opts.LearningSession.AnalyzeCurrentDeltaBatch(ctx, agentReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_current_delta_batch",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}
	if err := agent.RequireResult(result, "AnalyzeCurrentDeltaBatch"); err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}

	changes, err := s.validateDeltaChanges(ctx, projectRoot, result.Changes, focusByID)
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
	}, nil
}

func (s *AnalyzerService) validateDeltaChanges(ctx context.Context, projectRoot string, changes []domain.KnowledgeChange, focusByID map[string]map[string]bool) ([]domain.KnowledgeChange, error) {
	proposals := make([]domain.Pattern, 0)
	for _, change := range changes {
		if !change.CarriesPattern() || !deltaChangeAnchored(change, focusByID) {
			continue
		}
		proposals = append(proposals, *change.Proposal)
	}
	validator, err := newCurrentPatternValidator(ctx, projectRoot, proposals, s.symbolResolver)
	if err != nil {
		return nil, err
	}
	validByID := make(map[string]domain.Pattern, len(proposals))
	for _, pattern := range validator.validatePatterns(proposals) {
		validByID[pattern.ID] = pattern
	}

	validated := make([]domain.KnowledgeChange, 0, len(changes))
	for _, change := range changes {
		if !deltaChangeAnchored(change, focusByID) {
			continue
		}
		if !change.CarriesPattern() {
			validated = append(validated, change)
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
