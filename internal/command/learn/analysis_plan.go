package learn

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/analysisplan"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
)

func buildPlanInputs(changes *incrementalFileChanges) []analysisplan.FileInput {
	if changes == nil {
		return nil
	}
	inputs := make([]analysisplan.FileInput, 0, len(changes.Records)+len(changes.Deleted))
	for _, record := range changes.Records {
		inputs = append(inputs, analysisplan.FileInput{
			Path:   record.Path,
			Hash:   record.Hash,
			Status: "present",
		})
	}
	for _, path := range changes.Deleted {
		inputs = append(inputs, analysisplan.FileInput{
			Path:   path,
			Status: "deleted",
		})
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	return inputs
}

func canReuseAnalysisPlan(plan *analysisplan.Plan, changes *incrementalFileChanges, projectName, language, userContext string) bool {
	if plan == nil || changes == nil {
		return false
	}
	if plan.ProjectName != projectName || plan.Language != language || plan.UserContext != analysisplan.HashText(userContext) {
		return false
	}
	planned := map[string]analysisplan.FileInput{}
	for _, input := range plan.Inputs {
		planned[normalizePlanPath(input.Path)] = input
	}
	for _, input := range buildPlanInputs(changes) {
		existing, ok := planned[normalizePlanPath(input.Path)]
		if !ok || existing.Status != input.Status || existing.Hash != input.Hash {
			return false
		}
	}
	return len(plan.Units) > 0
}

func loadOrCreateAnalysisPlan(
	ctx context.Context,
	repo *analysisplan.Repository,
	analyzerSvc *analyzer.AnalyzerService,
	projectName string,
	projectRoot string,
	language string,
	focusRelPaths []string,
	changes *incrementalFileChanges,
	userContext string,
) (*analysisplan.Plan, error) {
	plan, err := repo.Load(ctx)
	if err != nil && err != analysisplan.ErrPlanNotFound {
		return nil, err
	}
	if canReuseAnalysisPlan(plan, changes, projectName, language, userContext) {
		return plan, nil
	}
	units, err := analyzerSvc.PlanAnalysisUnits(ctx, &analyzer.PlanAnalysisUnitsRequest{
		ProjectName: projectName,
		RootPath:    projectRoot,
		Language:    language,
		FocusPaths:  focusRelPaths,
		UserContext: userContext,
	})
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		units = []domain.AnalysisUnit{fallbackAnalysisUnit(focusRelPaths)}
	}
	plan = analysisplan.NewPlan(projectName, language, userContext, buildPlanInputs(changes), units)
	if err := repo.Save(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func pendingAnalysisUnits(plan *analysisplan.Plan, changes *incrementalFileChanges) []domain.AnalysisUnit {
	if plan == nil || changes == nil {
		return nil
	}
	pending := pathSet(analysisCandidatePaths(changes))
	units := make([]domain.AnalysisUnit, 0, len(plan.Units))
	for _, unit := range plan.Units {
		if len(intersectUnitPaths(unit, pending)) == 0 {
			continue
		}
		units = append(units, unit)
	}
	return units
}

func fallbackAnalysisUnit(paths []string) domain.AnalysisUnit {
	return domain.AnalysisUnit{
		ID:           "current-codebase",
		Name:         "当前代码变更",
		RouteTerms:   []string{"当前变更", "代码学习"},
		EntryPaths:   normalizePlanPaths(paths),
		RelatedPaths: nil,
		ScopeReason:  "AI 未返回有效业务单元时，使用当前待学习文件作为兜底分析范围",
	}
}

func unitFocusPaths(unit domain.AnalysisUnit, changes *incrementalFileChanges) []string {
	if changes == nil {
		return nil
	}
	allowed := pathSet(analysisCandidatePaths(changes))
	return intersectUnitPaths(unit, allowed)
}

func unitSelectedFiles(unit domain.AnalysisUnit, selectedFiles []domain.FileInfo, changes *incrementalFileChanges) []domain.FileInfo {
	allowed := pathSet(unitFocusPaths(unit, changes))
	out := make([]domain.FileInfo, 0, len(selectedFiles))
	for _, file := range selectedFiles {
		if allowed[normalizePlanPath(file.Path)] {
			out = append(out, file)
		}
	}
	return out
}

func unitAnalyzedRecords(unit domain.AnalysisUnit, changes *incrementalFileChanges) []domain.FileAnalysisRecord {
	allowed := pathSet(unitFocusPaths(unit, changes))
	out := make([]domain.FileAnalysisRecord, 0, len(changes.Records))
	for _, record := range changes.Records {
		if !allowed[normalizePlanPath(record.Path)] {
			continue
		}
		record.AnalysisStatus = domain.FileAnalysisStatusAnalyzed
		record.SelectionReason = ""
		out = append(out, record)
	}
	return out
}

func skippedSelectionRecords(changes *incrementalFileChanges) []domain.FileAnalysisRecord {
	if changes == nil {
		return nil
	}
	out := make([]domain.FileAnalysisRecord, 0, len(changes.Records))
	for _, record := range changes.Records {
		if record.AnalysisStatus == domain.FileAnalysisStatusAISkipped {
			out = append(out, record)
		}
	}
	return out
}

func analysisCandidatePaths(changes *incrementalFileChanges) []string {
	if changes == nil {
		return nil
	}
	paths := make([]string, 0, len(changes.Records)+len(changes.Deleted))
	for _, record := range changes.Records {
		if record.AnalysisStatus == domain.FileAnalysisStatusAISkipped {
			continue
		}
		paths = append(paths, record.Path)
	}
	paths = append(paths, changes.Deleted...)
	sort.Strings(paths)
	return paths
}

func unitCommittedRecords(unit domain.AnalysisUnit, changes *incrementalFileChanges) []domain.FileAnalysisRecord {
	records := unitAnalyzedRecords(unit, changes)
	records = append(records, skippedSelectionRecords(changes)...)
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	return records
}

func commitUnitFileRecords(ctx context.Context, tracker domain.FileAnalysisTracker, records []domain.FileAnalysisRecord) error {
	if len(records) == 0 {
		return nil
	}
	return tracker.SaveAnalyzedFiles(ctx, records)
}

func normalizePlanPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." {
		return ""
	}
	return strings.TrimPrefix(path, "./")
}

func normalizePlanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = normalizePlanPath(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func pathSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = normalizePlanPath(path)
		if path != "" {
			set[path] = true
		}
	}
	return set
}

func intersectUnitPaths(unit domain.AnalysisUnit, allowed map[string]bool) []string {
	paths := append([]string{}, unit.EntryPaths...)
	paths = append(paths, unit.RelatedPaths...)
	paths = normalizePlanPaths(paths)
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if allowed[path] {
			out = append(out, path)
		}
	}
	return out
}
