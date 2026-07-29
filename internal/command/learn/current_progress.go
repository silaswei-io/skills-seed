package learn

import (
	"strconv"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

type fileSelectionSummary struct {
	Applied        bool
	CandidateCount int
	SelectedCount  int
	SkippedCount   int
	Reason         string
	Status         string
}

func currentStateInputSummary(changes *fileanalysis.FileChanges, selectionPlan currentFileSelectionPlan, selectionSummary fileSelectionSummary) commandstate.InputSummary {
	localPlanInputs := len(selectionPlan.Candidates)
	selectionInputs := selectionSummary.CandidateCount
	selectedFiles := selectionSummary.SelectedCount
	skippedFiles := selectionSummary.SkippedCount
	if selectionSummary.Applied {
		localPlanInputs = selectionSummary.SelectedCount
	}
	sourceFiles := 0
	if changes != nil {
		sourceFiles = changes.SourceFileCount
	}
	return commandstate.InputSummary{
		SourceFiles:         sourceFiles,
		LocalPlanInputFiles: localPlanInputs,
		SelectionInputFiles: selectionInputs,
		SelectedFiles:       selectedFiles,
		SkippedFiles:        skippedFiles,
	}
}

func learnCurrentProgressDetail(baseLabel, detailKey string, params map[string]interface{}) string {
	if params == nil {
		params = map[string]interface{}{}
	}
	params["Label"] = baseLabel
	return i18n.GetWithParams(detailKey, params)
}

func learnCurrentProgressSubject(focus domain.EvidenceFocus) string {
	subject := strings.TrimSpace(focus.Name)
	if subject == "" {
		subject = strings.TrimSpace(focus.ID)
	}
	if subject == "" {
		subject = i18n.Get("LearnCurrentFallbackFocusName")
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

func learnCurrentFocusProgress(state *commandstate.State, fallbackIndex, fallbackTotal int, focus domain.EvidenceFocus) (int, int) {
	if state == nil || len(state.Agenda.Focuses) == 0 {
		return fallbackIndex, fallbackTotal
	}
	for index, planned := range state.Agenda.Focuses {
		if evidenceFocusSame(planned, focus) {
			return index + 1, len(state.Agenda.Focuses)
		}
	}
	return fallbackIndex, len(state.Agenda.Focuses)
}

func evidenceFocusSame(a, b domain.EvidenceFocus) bool {
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
	selectionInput := "-"
	selectionSelected := "-"
	selectionStatus := i18n.GetWithParams("LearnCurrentFileSelectionSkipped", map[string]interface{}{
		"Reason": r.selectionPlan.SkipReason,
	})
	if r.selectionSummary.CandidateCount > 0 {
		selectionInput = strconv.Itoa(r.selectionSummary.CandidateCount)
		selectionStatus = strings.TrimSpace(r.selectionSummary.Status)
		if selectionStatus == "" {
			selectionStatus = i18n.Get("LearnCurrentFileSelectionApplied")
		}
	}
	if r.selectionSummary.Applied {
		selectionSelected = strconv.Itoa(r.selectionSummary.SelectedCount)
	}
	logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentFileSelectionSummary", map[string]interface{}{
		"ScannedFiles":        r.incrementalChanges.ScannedFileCount,
		"LocalSkippedFiles":   len(r.incrementalChanges.Skipped),
		"SourceFiles":         r.incrementalChanges.SourceFileCount,
		"LocalPlanInputs":     len(r.selectionPlan.Candidates),
		"SelectionInputs":     selectionInput,
		"SelectedFiles":       selectionSelected,
		"SelectionStatus":     selectionStatus,
		"PendingAnalyzeFiles": len(analysisCandidatePaths(r.incrementalChanges)),
	}))
}

func (r *learnCurrentProjectRun) logFileSelectionSummaryOnce() {
	if r.fileSelectionSummaryLogged {
		return
	}
	r.fileSelectionSummaryLogged = true
	r.logFileSelectionSummary()
}

func (r *learnCurrentProjectRun) logDetectedChanges(startedAt time.Time) {
	if r.opts.showDetailedLogs {
		if r.resumeSummary != nil {
			logger.Info(i18n.GetWithParams("LearnCurrentResumeSummary", map[string]interface{}{
				"Command":             r.resumeSummary.Command,
				"CreatedAt":           r.resumeSummary.CreatedAt,
				"SourceFiles":         r.resumeSummary.SourceFiles,
				"LocalPlanInputs":     r.resumeSummary.LocalPlanInputs,
				"SelectionInputs":     r.resumeSummary.SelectionInputs,
				"SelectedFiles":       r.resumeSummary.SelectedFiles,
				"PendingAnalyzeFiles": r.resumeSummary.PendingAnalyzeFiles,
				"Focuses":             r.resumeSummary.Focuses,
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
