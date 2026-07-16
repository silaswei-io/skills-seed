package learn

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
)

type learnCurrentRunningUnits struct {
	mu        sync.Mutex
	labels    map[int]string
	order     []int
	completed int
	total     int
}

func newLearnCurrentRunningUnits(state *commandstate.State, plannedUnits []domain.AnalysisUnit) *learnCurrentRunningUnits {
	completed, total := learnCurrentPendingUnitProgress(state, plannedUnits)
	return &learnCurrentRunningUnits{
		labels:    make(map[int]string),
		completed: completed,
		total:     total,
	}
}

func (r *learnCurrentRunningUnits) start(index int, label string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.labels[index]; !exists {
		r.order = append(r.order, index)
		sort.Ints(r.order)
	}
	r.labels[index] = label
	return r.runningTextLocked()
}

func (r *learnCurrentRunningUnits) finish(index int, completed bool) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.labels, index)
	if completed {
		r.completed++
	}
	out := r.order[:0]
	for _, candidate := range r.order {
		if _, exists := r.labels[candidate]; exists {
			out = append(out, candidate)
		}
	}
	r.order = out
	return r.runningTextLocked()
}

func (r *learnCurrentRunningUnits) progressParams(parallelism int) map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.completed + 1
	if current > r.total {
		current = r.total
	}
	if current < 1 {
		current = 1
	}
	return map[string]interface{}{
		"Current":      current,
		"Total":        r.total,
		"Parallelism":  parallelism,
		"Running":      r.runningTextLocked(),
		"RunningCount": len(r.order),
	}
}

func (r *learnCurrentRunningUnits) runningTextLocked() string {
	if len(r.order) == 0 {
		return "-"
	}
	names := make([]string, 0, 2)
	for _, index := range r.order {
		if label := r.labels[index]; label != "" {
			names = append(names, shortenRunes(label, learnCurrentRunningSubjectMaxRunes))
		}
		if len(names) >= 2 {
			break
		}
	}
	if len(names) == 0 {
		return "-"
	}
	if extra := len(r.order) - len(names); extra > 0 {
		names = append(names, fmt.Sprintf("+%d", extra))
	}
	return strings.Join(names, ", ")
}

type aiFileSelectionSummary struct {
	Applied        bool
	Attempted      bool
	CandidateCount int
	SelectedCount  int
	SkippedCount   int
	Reason         string
	Status         string
}

func currentStateInputSummary(changes *fileanalysis.FileChanges, selectionPlan currentFileSelectionPlan, selectionSummary aiFileSelectionSummary) commandstate.InputSummary {
	localPlanInputs := len(selectionPlan.Candidates)
	aiSelectionInputs := 0
	aiSelectedFiles := 0
	aiSkippedFiles := 0
	if selectionSummary.Attempted || selectionSummary.Applied {
		aiSelectionInputs = selectionSummary.CandidateCount
	}
	if selectionSummary.Applied {
		aiSelectedFiles = selectionSummary.SelectedCount
		aiSkippedFiles = selectionSummary.SkippedCount
	}
	sourceFiles := 0
	if changes != nil {
		sourceFiles = changes.SourceFileCount
	}
	return commandstate.InputSummary{
		SourceFiles:         sourceFiles,
		LocalPlanInputFiles: localPlanInputs,
		SelectionInputFiles: aiSelectionInputs,
		SelectedFiles:       aiSelectedFiles,
		SkippedFiles:        aiSkippedFiles,
	}
}

func learnCurrentProgressDetail(baseLabel, detailKey string, params map[string]interface{}) string {
	if params == nil {
		params = map[string]interface{}{}
	}
	params["Label"] = baseLabel
	return i18n.GetWithParams(detailKey, params)
}

func learnCurrentProgressSubject(unit domain.AnalysisUnit) string {
	subject := strings.TrimSpace(unit.Name)
	if subject == "" {
		subject = strings.TrimSpace(unit.ID)
	}
	if subject == "" {
		subject = "unit"
	}
	return shortenRunes(subject, learnCurrentProgressSubjectMaxRunes)
}

func shortenRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func learnCurrentUnitProgress(state *commandstate.State, fallbackIndex, fallbackTotal int, unit domain.AnalysisUnit) (int, int) {
	if state == nil || len(state.Units) == 0 {
		return fallbackIndex, fallbackTotal
	}
	for index, planned := range state.Units {
		if analysisUnitSame(planned, unit) {
			return index + 1, len(state.Units)
		}
	}
	return fallbackIndex, len(state.Units)
}

func learnCurrentPendingUnitProgress(state *commandstate.State, plannedUnits []domain.AnalysisUnit) (int, int) {
	total := len(plannedUnits)
	if state != nil && len(state.Units) > 0 {
		total = len(state.Units)
	}
	completed := total - len(plannedUnits)
	if completed < 0 {
		completed = 0
	}
	return completed, total
}

func analysisUnitSame(a, b domain.AnalysisUnit) bool {
	if a.ID != "" || b.ID != "" {
		return a.ID == b.ID
	}
	return a.Name == b.Name
}

func (r *learnCurrentProjectRun) detail(baseLabel, detailKey string, params map[string]interface{}) string {
	r.progressDetailMu.Lock()
	defer r.progressDetailMu.Unlock()
	return r.steps.Detail(baseLabel, learnCurrentProgressDetail(baseLabel, detailKey, params))
}

func (r *learnCurrentProjectRun) logFileSelectionSummary() {
	if !r.opts.showDetailedLogs {
		return
	}
	if r.resumeSummary != nil {
		return
	}
	aiInput := "-"
	aiSelected := "-"
	aiStatus := i18n.GetWithParams("LearnCurrentFileSelectionSkipped", map[string]interface{}{
		"Reason": r.selectionPlan.SkipReason,
	})
	if r.selectionSummary.Attempted {
		aiInput = strconv.Itoa(r.selectionSummary.CandidateCount)
		aiStatus = strings.TrimSpace(r.selectionSummary.Status)
		if aiStatus == "" {
			aiStatus = i18n.Get("LearnCurrentAIFileSelectorFallback")
		}
	}
	if r.selectionSummary.Applied {
		aiSelected = strconv.Itoa(r.selectionSummary.SelectedCount)
	}
	logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentFileSelectionSummary", map[string]interface{}{
		"ScannedFiles":        r.incrementalChanges.ScannedFileCount,
		"LocalSkippedFiles":   len(r.incrementalChanges.Skipped),
		"SourceFiles":         r.incrementalChanges.SourceFileCount,
		"LocalPlanInputs":     len(r.selectionPlan.Candidates),
		"AISelectionInputs":   aiInput,
		"AISelectedFiles":     aiSelected,
		"AISelectionStatus":   aiStatus,
		"PendingAnalyzeFiles": len(analysisCandidatePaths(r.incrementalChanges)),
	}))
}

func (r *learnCurrentProjectRun) logDetectedChanges(startedAt time.Time) {
	if r.opts.showDetailedLogs {
		if r.resumeSummary != nil {
			logger.Info(i18n.GetWithParams("LearnCurrentResumeSummary", map[string]interface{}{
				"Command":             r.resumeSummary.Command,
				"CreatedAt":           r.resumeSummary.CreatedAt,
				"SourceFiles":         r.resumeSummary.SourceFiles,
				"LocalPlanInputs":     r.resumeSummary.LocalPlanInputs,
				"AISelectionInputs":   r.resumeSummary.AISelectionInputs,
				"AISelectedFiles":     r.resumeSummary.AISelectedFiles,
				"PendingAnalyzeFiles": r.resumeSummary.PendingAnalyzeFiles,
				"Units":               r.resumeSummary.Units,
			}))
		} else {
			logger.Info(i18n.GetWithParams("LearnCurrentIncrementalSummary", map[string]interface{}{
				"ScannedFiles":      r.incrementalChanges.ScannedFileCount,
				"LocalSkippedFiles": len(r.incrementalChanges.Skipped),
				"SourceFiles":       r.incrementalChanges.SourceFileCount,
				"Changed":           len(r.incrementalChanges.AddedOrModified),
				"Deleted":           len(r.incrementalChanges.Deleted),
				"Unchanged":         len(r.incrementalChanges.Unchanged),
				"LocalPlanInputs":   len(analysisCandidatePaths(r.incrementalChanges)),
			}))
			if len(r.incrementalChanges.ExcludedGeneratedSkillDirs) > 0 {
				logger.Info(i18n.GetWithParams("LearnCurrentGeneratedSkillsExcluded", map[string]interface{}{
					"Paths": strings.Join(r.incrementalChanges.ExcludedGeneratedSkillDirs, ", "),
				}))
			}
		}
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.detect_changes",
		"duration", time.Since(startedAt),
		"changed_count", len(r.incrementalChanges.AddedOrModified),
		"deleted_count", len(r.incrementalChanges.Deleted),
		"unchanged_count", len(r.incrementalChanges.Unchanged),
		"skipped_count", len(r.incrementalChanges.Skipped),
	)
}
