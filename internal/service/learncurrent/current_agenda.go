package learncurrent

import (
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

func (r *learnCurrentProjectRun) planLearningAgenda() error {
	planStartedAt := time.Now()
	planLabel := i18n.Get("ProgressLearnCurrentPlanFocuses")
	if r.stateSession != nil {
		developmentFocuses, coverageFocuses := focusKindCounts(r.stateSession.State.Agenda.Focuses)
		planLabel = i18n.GetWithParams("ProgressLearnCurrentPlanFocusesRestored", map[string]interface{}{
			"Focuses":            len(r.stateSession.State.Agenda.Focuses),
			"DevelopmentFocuses": developmentFocuses,
			"CoverageFocuses":    coverageFocuses,
		})
	}
	if err := r.steps.Run(planLabel, func() error {
		focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
		state := (*commandstate.State)(nil)
		if r.stateSession != nil {
			state = r.stateSession.State
		}
		if state == nil {
			var err error
			state, err = loadOrCreateCurrentState(r.ctx, r.stateRepo, r.cont.AnalyzerSvc, r.projectName, r.projectRoot, r.currentLanguage, r.learningMode, focusRelPaths, r.incrementalChanges, currentStateInputSummary(r.incrementalChanges, r.selectionPlan, r.selectionSummary), r.changeProfile, r.opts.userContext, r.currentStateInvocationHash())
			if err != nil {
				return err
			}
		}
		r.analysisState = state
		r.plannedFocuses = pendingEvidenceFocuses(state, r.incrementalChanges)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.plan_learning_agenda",
			"duration", time.Since(planStartedAt),
			"focuses_count", len(state.Agenda.Focuses),
			"pending_focuses_count", len(r.plannedFocuses),
			"candidate_count", len(focusRelPaths),
		)
		return nil
	}); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.plan_learning_agenda",
			"duration", time.Since(planStartedAt),
			"error", err,
		)
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToAnalyzeCodebase", map[string]interface{}{"Error": err.Error()}))
	}
	return nil
}
