package learn

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
)

const (
	commandStateLearnCurrent = "learn-current"
)

type currentStateSession struct {
	State   *commandstate.State
	Changes *incrementalFileChanges
	Resumed bool
}

type learnCurrentResumeSummary struct {
	Command             string
	CreatedAt           string
	SourceFiles         string
	LocalPlanInputs     int
	AISelectionInputs   string
	AISelectedFiles     string
	PendingAnalyzeFiles int
	Units               int
}

func learnCurrentStateRepo(seedPath, scope string) *commandstate.Repository {
	if strings.TrimSpace(scope) == "" {
		scope = commandStateLearnCurrent
	}
	return commandstate.NewRepository(seedPath, scope)
}

func buildLearnCurrentResumeSummary(session *currentStateSession) *learnCurrentResumeSummary {
	if session == nil || session.State == nil || session.Changes == nil {
		return nil
	}
	aiSkipped := 0
	deleted := 0
	for _, input := range session.State.Inputs {
		switch input.Status {
		case domain.FileAnalysisStatusAISkipped:
			aiSkipped++
		case "deleted":
			deleted++
		}
	}
	summary := session.State.InputSummary
	sourceFiles := "-"
	localPlanInputs := len(session.State.Inputs)
	aiSelectionInputs := "-"
	aiSelectedFiles := "-"
	if summary != nil {
		sourceFiles = displayCount(summary.SourceFiles)
		if summary.LocalPlanInputFiles > 0 {
			localPlanInputs = summary.LocalPlanInputFiles
		}
		aiSelectionInputs = displayCount(summary.SelectionInputFiles)
		if summary.SelectionInputFiles > 0 {
			aiSelectedFiles = displayCount(summary.SelectedFiles)
		}
	} else if aiSkipped > 0 {
		aiSelectionInputs = displayCount(len(session.State.Inputs) - deleted)
		aiSelectedFiles = displayCount(len(session.State.Inputs) - aiSkipped - deleted)
	}
	return &learnCurrentResumeSummary{
		Command:             session.State.Command,
		CreatedAt:           session.State.CreatedAt,
		SourceFiles:         sourceFiles,
		LocalPlanInputs:     localPlanInputs,
		AISelectionInputs:   aiSelectionInputs,
		AISelectedFiles:     aiSelectedFiles,
		PendingAnalyzeFiles: len(analysisCandidatePaths(session.Changes)),
		Units:               len(session.State.Units),
	}
}

func displayCount(count int) string {
	if count <= 0 {
		return "-"
	}
	return strconv.Itoa(count)
}

func learnCurrentStateMode(mode, scope string) string {
	mode = string(config.NormalizeLearningMode(mode))
	scope = string(config.NormalizeLearningScope(scope))
	return mode + "|scope=" + scope
}

func buildStateInputs(changes *incrementalFileChanges) []commandstate.FileInput {
	if changes == nil {
		return nil
	}
	inputs := make([]commandstate.FileInput, 0, len(changes.Records)+len(changes.Deleted))
	for _, record := range changes.Records {
		status := "present"
		if record.AnalysisStatus == domain.FileAnalysisStatusAISkipped {
			status = domain.FileAnalysisStatusAISkipped
		}
		inputs = append(inputs, commandstate.FileInput{
			Path:   record.Path,
			Hash:   record.Hash,
			Status: status,
		})
	}
	for _, path := range changes.Deleted {
		inputs = append(inputs, commandstate.FileInput{
			Path:   path,
			Status: "deleted",
		})
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	return inputs
}

func canReuseCurrentState(state *commandstate.State, changes *incrementalFileChanges, projectName, language, mode, userContext string) bool {
	if state == nil || changes == nil {
		return false
	}
	if state.ProjectName != projectName || state.Language != language || state.Mode != mode || state.UserContext != commandstate.HashText(userContext) {
		return false
	}
	planned := map[string]commandstate.FileInput{}
	for _, input := range state.Inputs {
		planned[normalizeStatePath(input.Path)] = input
	}
	for _, input := range buildStateInputs(changes) {
		existing, ok := planned[normalizeStatePath(input.Path)]
		if !ok || existing.Status != input.Status || existing.Hash != input.Hash {
			return false
		}
	}
	return len(state.Units) > 0
}

func canResumeCurrentState(state *commandstate.State, projectName, language, mode, userContext string) bool {
	return state != nil &&
		state.ProjectName == projectName &&
		state.Language == language &&
		state.Mode == mode &&
		state.UserContext == commandstate.HashText(userContext) &&
		len(state.Units) > 0 &&
		len(state.Inputs) > 0
}

func changesFromCurrentState(state *commandstate.State) *incrementalFileChanges {
	if state == nil {
		return nil
	}
	changes := &incrementalFileChanges{}
	for _, input := range state.Inputs {
		path := normalizeStatePath(input.Path)
		if path == "" {
			continue
		}
		switch input.Status {
		case "deleted":
			changes.Deleted = append(changes.Deleted, path)
		default:
			record := domain.FileAnalysisRecord{
				Path: path,
				Hash: input.Hash,
			}
			if input.Status == domain.FileAnalysisStatusAISkipped {
				record.AnalysisStatus = domain.FileAnalysisStatusAISkipped
			}
			changes.Records = append(changes.Records, record)
			changes.AddedOrModified = append(changes.AddedOrModified, path)
		}
	}
	sort.Strings(changes.AddedOrModified)
	sort.Strings(changes.Deleted)
	sort.Slice(changes.Records, func(i, j int) bool { return changes.Records[i].Path < changes.Records[j].Path })
	return changes
}

func filterCompletedStateChanges(changes *incrementalFileChanges, analyzed []domain.FileAnalysisRecord) *incrementalFileChanges {
	if changes == nil {
		return nil
	}
	byPath := make(map[string]domain.FileAnalysisRecord, len(analyzed))
	for _, record := range analyzed {
		path := normalizeStatePath(record.Path)
		if path != "" {
			byPath[path] = record
		}
	}

	filtered := *changes
	filtered.Records = nil
	filtered.AddedOrModified = nil
	filtered.Deleted = append([]string{}, changes.Deleted...)
	filtered.Unchanged = nil

	for _, record := range changes.Records {
		path := normalizeStatePath(record.Path)
		if path == "" {
			continue
		}
		record.Path = path
		if tracked, ok := byPath[path]; ok && stateRecordCompleted(record, tracked) {
			continue
		}
		filtered.Records = append(filtered.Records, record)
		filtered.AddedOrModified = append(filtered.AddedOrModified, path)
	}
	sort.Strings(filtered.AddedOrModified)
	sort.Strings(filtered.Deleted)
	sort.Slice(filtered.Records, func(i, j int) bool { return filtered.Records[i].Path < filtered.Records[j].Path })
	return &filtered
}

func stateRecordCompleted(expected, tracked domain.FileAnalysisRecord) bool {
	if expected.Hash != "" && tracked.Hash != expected.Hash {
		return false
	}
	switch expected.AnalysisStatus {
	case domain.FileAnalysisStatusAISkipped:
		return completedAnalysisStatus(tracked.AnalysisStatus) == domain.FileAnalysisStatusAISkipped ||
			completedAnalysisStatus(tracked.AnalysisStatus) == domain.FileAnalysisStatusAnalyzed
	default:
		return completedAnalysisStatus(tracked.AnalysisStatus) == domain.FileAnalysisStatusAnalyzed
	}
}

func completedAnalysisStatus(status string) string {
	if status == "" {
		return domain.FileAnalysisStatusAnalyzed
	}
	return status
}

func restoreCurrentState(
	ctx context.Context,
	repo *commandstate.Repository,
	tracker domain.FileAnalysisTracker,
	projectName string,
	language string,
	mode string,
	userContext string,
) (*currentStateSession, error) {
	state, err := repo.Load(ctx)
	if err != nil {
		if err == commandstate.ErrStateNotFound {
			return nil, nil
		}
		return nil, err
	}
	if !canResumeCurrentState(state, projectName, language, mode, userContext) {
		return nil, nil
	}
	analyzedRecords, err := tracker.ListAnalyzedFiles(ctx, domain.FileAnalysisScope{})
	if err != nil {
		return nil, err
	}
	return &currentStateSession{
		State:   state,
		Changes: filterCompletedStateChanges(changesFromCurrentState(state), analyzedRecords),
		Resumed: true,
	}, nil
}

func loadOrCreateCurrentState(
	ctx context.Context,
	repo *commandstate.Repository,
	analyzerSvc *analyzer.AnalyzerService,
	projectName string,
	projectRoot string,
	language string,
	mode string,
	scope string,
	focusRelPaths []string,
	changes *incrementalFileChanges,
	inputSummary commandstate.InputSummary,
	userContext string,
) (*commandstate.State, error) {
	state, err := repo.Load(ctx)
	if err != nil && err != commandstate.ErrStateNotFound {
		return nil, err
	}
	stateMode := learnCurrentStateMode(mode, scope)
	if canReuseCurrentState(state, changes, projectName, language, stateMode, userContext) {
		return state, nil
	}
	units, err := analyzerSvc.PlanAnalysisUnits(ctx, &analyzer.PlanAnalysisUnitsRequest{
		ProjectName:   projectName,
		RootPath:      projectRoot,
		Language:      language,
		LearningMode:  config.NormalizeLearningMode(mode),
		LearningScope: config.NormalizeLearningScope(scope),
		FocusPaths:    focusRelPaths,
		UserContext:   userContext,
	})
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		units = []domain.AnalysisUnit{fallbackAnalysisUnit(focusRelPaths)}
	}
	state = commandstate.NewStateWithMode(repo.Command(), projectName, language, stateMode, userContext, buildStateInputs(changes), units).
		WithInputSummary(inputSummary)
	if err := repo.Save(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func pendingAnalysisUnits(state *commandstate.State, changes *incrementalFileChanges) []domain.AnalysisUnit {
	if state == nil || changes == nil {
		return nil
	}
	pending := pathSet(analysisCandidatePaths(changes))
	units := make([]domain.AnalysisUnit, 0, len(state.Units))
	for _, unit := range state.Units {
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
		EntryPaths:   normalizeStatePaths(paths),
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
		if allowed[normalizeStatePath(file.Path)] {
			out = append(out, file)
		}
	}
	return out
}

func unitAnalyzedRecords(unit domain.AnalysisUnit, changes *incrementalFileChanges) []domain.FileAnalysisRecord {
	allowed := pathSet(unitFocusPaths(unit, changes))
	out := make([]domain.FileAnalysisRecord, 0, len(changes.Records))
	for _, record := range changes.Records {
		if !allowed[normalizeStatePath(record.Path)] {
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

func normalizeStatePath(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." {
		return ""
	}
	return strings.TrimPrefix(path, "./")
}

func normalizeStatePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = normalizeStatePath(path)
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
		path = normalizeStatePath(path)
		if path != "" {
			set[path] = true
		}
	}
	return set
}

func intersectUnitPaths(unit domain.AnalysisUnit, allowed map[string]bool) []string {
	paths := append([]string{}, unit.EntryPaths...)
	paths = append(paths, unit.RelatedPaths...)
	paths = normalizeStatePaths(paths)
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if allowed[path] {
			out = append(out, path)
		}
	}
	return out
}
