package learn

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

func (r *learnCurrentProjectRun) ensureSharedLearningContext() error {
	if r.sharedLearningContextPath != "" || r.cont == nil || strings.TrimSpace(r.cont.SeedPath) == "" {
		return nil
	}
	path := layout.New(r.cont.SeedPath).Runtime("learn-current", "shared-context.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(r.sharedLearningContextText()), 0600); err != nil {
		return err
	}
	r.sharedLearningContextPath = path
	return nil
}

func (r *learnCurrentProjectRun) sharedLearningContextText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", i18n.Get("LearnCurrentSharedContextTitle"))
	fmt.Fprintf(&b, "%s\n\n", i18n.Get("LearnCurrentSharedContextDescription"))
	fmt.Fprintf(&b, "%s:\n", i18n.Get("LearnCurrentSharedContextProject"))
	fmt.Fprintf(&b, "- %s: %s\n", i18n.Get("LearnCurrentSharedContextProjectName"), r.projectName)
	fmt.Fprintf(&b, "- %s: %s\n", i18n.Get("LearnCurrentSharedContextProjectPath"), r.projectRoot)
	fmt.Fprintf(&b, "- %s: %s\n", i18n.Get("LearnCurrentSharedContextProjectLanguage"), r.currentLanguage)
	fmt.Fprintf(&b, "- %s: %s\n", i18n.Get("LearnCurrentSharedContextLearningMode"), r.learningMode)
	fmt.Fprintf(&b, "- %s: %s\n", i18n.Get("LearnCurrentSharedContextLearningScope"), r.learningScope)
	fmt.Fprintf(&b, "- %s: %s\n\n", i18n.Get("LearnCurrentSharedContextChangeProfile"), r.changeProfile)

	fmt.Fprintf(&b, "%s:\n", i18n.Get("LearnCurrentSharedContextCandidateSelection"))
	fmt.Fprintf(&b, "- %s: %d\n", i18n.Get("LearnCurrentSharedContextCandidateCount"), len(r.selectionPlan.Candidates))
	fmt.Fprintf(&b, "- %s: %d\n", i18n.Get("LearnCurrentSharedContextSelectedCount"), len(r.selectedFiles))
	if r.selectionSummary.Status != "" {
		fmt.Fprintf(&b, "- %s: %s\n", i18n.Get("LearnCurrentSharedContextSelectionStatus"), r.selectionSummary.Status)
	}
	fmt.Fprintf(&b, "\n%s:\n", i18n.Get("LearnCurrentSharedContextAgenda"))
	for i, focus := range sharedContextFocuses(r.analysisStateAgendaFocuses(), r.plannedFocuses) {
		fmt.Fprintf(&b, "%d. %s", i+1, learnCurrentProgressSubject(focus))
		if focus.ID != "" {
			fmt.Fprintf(&b, " (`%s`)", focus.ID)
		}
		b.WriteByte('\n')
		writeSharedContextList(&b, i18n.Get("LearnCurrentSharedContextEntryPaths"), focus.EntryPaths)
		writeSharedContextList(&b, i18n.Get("LearnCurrentSharedContextRelatedPaths"), focus.RelatedPaths)
	}
	return b.String()
}

func (r *learnCurrentProjectRun) analysisStateAgendaFocuses() []domain.EvidenceFocus {
	if r.analysisState == nil {
		return nil
	}
	return r.analysisState.Agenda.Focuses
}

func sharedContextFocuses(all, planned []domain.EvidenceFocus) []domain.EvidenceFocus {
	if len(all) > 0 {
		return all
	}
	return planned
}

func writeSharedContextList(b *strings.Builder, label string, paths []string) {
	if len(paths) == 0 {
		return
	}
	fmt.Fprintf(b, "   - %s:\n", label)
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			fmt.Fprintf(b, "     - %s\n", path)
		}
	}
}
