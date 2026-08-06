package learn

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

const (
	commandStateLearnCurrent = "learn-current"
	// currentAnalysisPlanContract 标识分析计划必须完整覆盖待分析文件的当前契约。
	currentAnalysisPlanContract = "full-file-coverage-v1"
)

type currentStateSession struct {
	State   *commandstate.State
	Changes *fileanalysis.FileChanges
}

type learnCurrentResumeSummary struct {
	Command             string
	CreatedAt           string
	SourceFiles         string
	LocalPlanInputs     int
	SelectionInputs     string
	SelectedFiles       string
	PendingAnalyzeFiles int
	Focuses             int
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
	selectionSkipped := 0
	for _, record := range session.State.Files {
		switch record.AnalysisStatus {
		case domain.FileAnalysisStatusSelectionSkipped:
			selectionSkipped++
		}
	}
	summary := session.State.InputSummary
	sourceFiles := "-"
	localPlanInputs := currentStateInputCount(session.State)
	selectionInputs := "-"
	selectedFiles := "-"
	if summary != nil {
		sourceFiles = displayCount(summary.SourceFiles)
		if summary.LocalPlanInputFiles > 0 {
			localPlanInputs = summary.LocalPlanInputFiles
		}
		selectionInputs = displayCount(summary.SelectionInputFiles)
		if summary.SelectionInputFiles > 0 {
			selectedFiles = displayCount(summary.SelectedFiles)
		}
	} else if selectionSkipped > 0 {
		selectionInputs = displayCount(len(session.State.Files))
		selectedFiles = displayCount(len(session.State.Files) - selectionSkipped)
	}
	return &learnCurrentResumeSummary{
		Command:             session.State.Command,
		CreatedAt:           session.State.CreatedAt,
		SourceFiles:         sourceFiles,
		LocalPlanInputs:     localPlanInputs,
		SelectionInputs:     selectionInputs,
		SelectedFiles:       selectedFiles,
		PendingAnalyzeFiles: pendingResumeAnalysisFiles(session),
		Focuses:             len(session.State.Agenda.Focuses),
	}
}

func pendingResumeAnalysisFiles(session *currentStateSession) int {
	if session == nil || session.State == nil {
		return 0
	}
	if session.State.Analysis == nil {
		return len(analysisCandidatePaths(session.Changes))
	}
	pending := pathSet(nil)
	for _, focus := range pendingEvidenceFocuses(session.State, session.Changes) {
		for _, path := range evidenceFocusPaths(focus, session.Changes) {
			pending[path] = true
		}
	}
	return len(pending)
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

func learnCurrentInvocationHash(configRepo config.Reader, focusPaths []string, profileMode string, force bool) string {
	type invocation struct {
		PlanContract  string                       `json:"plan_contract"`
		FocusPaths    []string                     `json:"focus_paths"`
		ProfileMode   string                       `json:"profile_mode"`
		Force         bool                         `json:"force"`
		CurrentConfig config.CurrentLearningConfig `json:"current_config"`
		ExcludeConfig config.ExcludeConfig         `json:"exclude_config"`
		SkillsConfig  config.SkillsConfig          `json:"skills_config"`
	}
	value := invocation{
		PlanContract: currentAnalysisPlanContract,
		FocusPaths:   normalizeStatePaths(focusPaths),
		ProfileMode:  strings.ToLower(strings.TrimSpace(profileMode)),
		Force:        force,
	}
	if configRepo != nil {
		value.CurrentConfig = configRepo.GetCurrentLearningConfig()
		value.ExcludeConfig = configRepo.GetExcludeConfig()
		value.SkillsConfig = configRepo.GetSkillsConfig()
	}
	data, _ := json.Marshal(value)
	return commandstate.HashText(string(data))
}

func buildStateFiles(changes *fileanalysis.FileChanges) []domain.FileAnalysisRecord {
	if changes == nil {
		return nil
	}
	return append([]domain.FileAnalysisRecord(nil), changes.Records...)
}

func currentStateInputCount(state *commandstate.State) int {
	if state == nil {
		return 0
	}
	return len(state.Files) + len(state.Deleted)
}

func canReuseCurrentState(state *commandstate.State, changes *fileanalysis.FileChanges, projectName, language, mode, userContext, invocationHash string) bool {
	if state == nil || changes == nil {
		return false
	}
	if state.ProjectName != projectName || state.Language != language || state.Mode != mode || state.UserContext != commandstate.HashText(userContext) || state.InvocationHash != invocationHash {
		return false
	}
	planned := make(map[string]domain.FileAnalysisRecord, len(state.Files))
	for _, record := range state.Files {
		planned[normalizeStatePath(record.Path)] = record
	}
	currentFiles := buildStateFiles(changes)
	if len(planned) != len(currentFiles) || !sameStatePaths(state.Deleted, changes.Deleted) {
		return false
	}
	for _, record := range currentFiles {
		existing, ok := planned[normalizeStatePath(record.Path)]
		if !ok || completedAnalysisStatus(existing.AnalysisStatus) != completedAnalysisStatus(record.AnalysisStatus) || existing.Hash != record.Hash {
			return false
		}
	}
	return len(state.Agenda.Focuses) > 0
}

func canResumeCurrentState(state *commandstate.State, projectName, language, mode, userContext, invocationHash string) bool {
	return state != nil &&
		state.ProjectName == projectName &&
		state.Language == language &&
		state.Mode == mode &&
		state.UserContext == commandstate.HashText(userContext) &&
		state.InvocationHash == invocationHash &&
		len(state.Agenda.Focuses) > 0 &&
		currentStateInputCount(state) > 0
}

func currentChangesCoveredByState(state *commandstate.State, changes *fileanalysis.FileChanges) bool {
	if state == nil || changes == nil {
		return false
	}
	planned := make(map[string]domain.FileAnalysisRecord, len(state.Files))
	for _, record := range state.Files {
		planned[normalizeStatePath(record.Path)] = record
	}
	for _, current := range changes.Records {
		existing, ok := planned[normalizeStatePath(current.Path)]
		if !ok || existing.Hash != current.Hash {
			return false
		}
	}
	return sameStatePaths(state.Deleted, changes.Deleted)
}

func sameStatePaths(left, right []string) bool {
	left = normalizeStatePaths(left)
	right = normalizeStatePaths(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func currentStateInputsMatchProject(projectRoot string, files []domain.FileAnalysisRecord, deleted []string) bool {
	for _, record := range files {
		path := filepath.Join(projectRoot, filepath.FromSlash(normalizeStatePath(record.Path)))
		resolved, err := projectpath.CanonicalWithinRoot(projectRoot, path)
		if err != nil {
			return false
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return false
		}
		if record.Hash != "" {
			sum := md5.Sum(data)
			if hex.EncodeToString(sum[:]) != record.Hash {
				return false
			}
		}
	}
	for _, deletedPath := range deleted {
		path := filepath.Join(projectRoot, filepath.FromSlash(normalizeStatePath(deletedPath)))
		resolved, err := projectpath.CanonicalWithinRoot(projectRoot, path)
		if err != nil {
			return false
		}
		if _, err := os.Stat(resolved); !os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func changesFromCurrentState(state *commandstate.State) *fileanalysis.FileChanges {
	if state == nil {
		return nil
	}
	changes := &fileanalysis.FileChanges{Deleted: normalizeStatePaths(state.Deleted)}
	changes.PreviousAnalyzedCount = currentStateInputCount(state)
	for _, stored := range state.Files {
		path := normalizeStatePath(stored.Path)
		if path == "" {
			continue
		}
		record := stored
		record.Path = path
		changes.Records = append(changes.Records, record)
		changes.AddedOrModified = append(changes.AddedOrModified, path)
	}
	sort.Strings(changes.AddedOrModified)
	sort.Strings(changes.Deleted)
	sort.Slice(changes.Records, func(i, j int) bool { return changes.Records[i].Path < changes.Records[j].Path })
	return changes
}

func filterCompletedStateChanges(changes *fileanalysis.FileChanges, analyzed []domain.FileAnalysisRecord) *fileanalysis.FileChanges {
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
	case domain.FileAnalysisStatusSelectionSkipped:
		return completedAnalysisStatus(tracked.AnalysisStatus) == domain.FileAnalysisStatusSelectionSkipped ||
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
	invocationHash string,
) (*currentStateSession, error) {
	state, err := repo.Load(ctx)
	if err != nil {
		if err == commandstate.ErrStateNotFound {
			return nil, nil
		}
		if errors.Is(err, commandstate.ErrUnsupportedSchemaVersion) {
			return nil, repo.Clear()
		}
		return nil, err
	}
	if !canResumeCurrentState(state, projectName, language, mode, userContext, invocationHash) {
		if err := repo.Clear(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	analyzedRecords, err := tracker.ListAnalyzedFiles(ctx, domain.FileAnalysisScope{})
	if err != nil {
		return nil, err
	}
	return &currentStateSession{
		State:   state,
		Changes: filterCompletedStateChanges(changesFromCurrentState(state), analyzedRecords),
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
	changes *fileanalysis.FileChanges,
	inputSummary commandstate.InputSummary,
	changeProfile currentChangeProfile,
	userContext string,
	invocationHash string,
) (*commandstate.State, error) {
	state, err := repo.Load(ctx)
	if err != nil && err != commandstate.ErrStateNotFound {
		return nil, err
	}
	stateMode := learnCurrentStateMode(mode, scope)
	if canReuseCurrentState(state, changes, projectName, language, stateMode, userContext, invocationHash) {
		return state, nil
	}
	var focuses []domain.EvidenceFocus
	if len(focusRelPaths) > 0 {
		focuses, err = analyzerSvc.PlanLearningAgenda(ctx, &analyzer.PlanLearningAgendaRequest{
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
		focuses = reconcileEvidenceFocuses(focuses, focusRelPaths)
	}
	state = commandstate.NewStateWithMode(repo.Command(), projectName, language, stateMode, userContext, buildStateFiles(changes), changes.Deleted, focuses).
		WithInvocationHash(invocationHash).
		WithInputSummary(inputSummary).
		WithChangeProfile(string(changeProfile))
	if err := repo.Save(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func pendingEvidenceFocuses(state *commandstate.State, changes *fileanalysis.FileChanges) []domain.EvidenceFocus {
	if state == nil || changes == nil {
		return nil
	}
	pending := pathSet(analysisCandidatePaths(changes))
	var completed []domain.EvidenceFocus
	if state.Analysis != nil {
		completed = state.Analysis.CompletedFocuses
	}
	focuses := make([]domain.EvidenceFocus, 0, len(state.Agenda.Focuses))
	for _, focus := range state.Agenda.Focuses {
		if evidenceFocusIncluded(completed, focus) {
			continue
		}
		if len(intersectFocusPaths(focus, pending)) == 0 {
			continue
		}
		focuses = append(focuses, focus)
	}
	return focuses
}

func evidenceFocusIncluded(focuses []domain.EvidenceFocus, target domain.EvidenceFocus) bool {
	for _, focus := range focuses {
		if evidenceFocusSame(focus, target) {
			return true
		}
	}
	return false
}

func subtractStatePaths(all, selected []string) []string {
	selectedSet := pathSet(selected)
	out := make([]string, 0)
	for _, path := range normalizeStatePaths(all) {
		if !selectedSet[path] {
			out = append(out, path)
		}
	}
	return out
}

func sortedBoolPaths(paths map[string]bool) []string {
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func evidenceFocusPaths(focus domain.EvidenceFocus, changes *fileanalysis.FileChanges) []string {
	if changes == nil {
		return nil
	}
	allowed := pathSet(analysisCandidatePaths(changes))
	return intersectFocusPaths(focus, allowed)
}

func analysisCandidatePaths(changes *fileanalysis.FileChanges) []string {
	if changes == nil {
		return nil
	}
	paths := make([]string, 0, len(changes.Records)+len(changes.Deleted))
	for _, record := range changes.Records {
		if record.AnalysisStatus == domain.FileAnalysisStatusSelectionSkipped {
			continue
		}
		paths = append(paths, record.Path)
	}
	paths = append(paths, changes.Deleted...)
	sort.Strings(paths)
	return paths
}

func normalizeStatePath(path string) string {
	return pathx.CleanRelative(path)
}

func normalizeStatePaths(paths []string) []string {
	return pathx.CleanRelativeList(paths)
}

func pathSet(paths []string) map[string]bool {
	return pathx.CleanRelativeSet(paths)
}

func intersectFocusPaths(focus domain.EvidenceFocus, allowed map[string]bool) []string {
	paths := append([]string{}, focus.EntryPaths...)
	paths = append(paths, focus.RelatedPaths...)
	paths = normalizeStatePaths(paths)
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if allowed[path] {
			out = append(out, path)
		}
	}
	return out
}
