package analyzer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// learningAgendaInputSet 是议程规划本次输入文件的精确边界。
// 它只投影和校验路径，不解释任何项目语义。
type learningAgendaInputSet map[string]struct{}

func newLearningAgendaInputSet(paths []string) learningAgendaInputSet {
	inputs := make(learningAgendaInputSet, len(paths))
	for _, path := range paths {
		path = cleanLearningAgendaPath(path)
		if path != "" && path != "." {
			inputs[path] = struct{}{}
		}
	}
	return inputs
}

func (inputs learningAgendaInputSet) contains(path string) bool {
	_, ok := inputs[cleanLearningAgendaPath(path)]
	return ok
}

func (inputs learningAgendaInputSet) paths() []string {
	paths := make([]string, 0, len(inputs))
	for path := range inputs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

type reconciledLearningAgenda struct {
	Focuses []domain.EvidenceFocus
	Skipped []agent.LearningPathSkip
}

// reconcileLearningAgenda 将 Agent 的规划回执收敛为每个输入路径唯一归属的议程。
// 它只处理路径、重复和覆盖等结构性问题；焦点语义仍完全由 Agent 负责。
func reconcileLearningAgenda(inputPaths []string, rawFocuses []domain.EvidenceFocus, rawSkipped []agent.LearningPathSkip, fallbackPathsPerFocus int) (reconciledLearningAgenda, error) {
	reconciler := learningAgendaReconciler{inputs: newLearningAgendaInputSet(inputPaths)}
	focuses := reconciler.focuses(rawFocuses)
	if err := validateLearningAgendaFocuses(focuses); err != nil {
		return reconciledLearningAgenda{}, err
	}
	skipped := reconciler.skipped(rawSkipped, focuses)
	focuses = reconciler.completeCoverage(focuses, skipped, fallbackPathsPerFocus)
	if err := validateLearningAgendaCoverageForInputs(reconciler.inputs, focuses, skipped); err != nil {
		return reconciledLearningAgenda{}, err
	}
	return reconciledLearningAgenda{Focuses: focuses, Skipped: skipped}, nil
}

type learningAgendaReconciler struct {
	inputs learningAgendaInputSet
}

func (r learningAgendaReconciler) focuses(raw []domain.EvidenceFocus) []domain.EvidenceFocus {
	claimed := make(map[string]struct{}, len(r.inputs))
	focuses := make([]domain.EvidenceFocus, 0, len(raw))
	for _, focus := range raw {
		focus.ID = strings.TrimSpace(focus.ID)
		focus.Name = strings.TrimSpace(focus.Name)
		focus.EntryPaths = r.claimPaths(focus.EntryPaths, claimed)
		// 关联证据仅在焦点内部去重，不能抢占其他焦点的学习责任。
		local := make(map[string]struct{}, len(focus.EntryPaths))
		for _, path := range focus.EntryPaths {
			local[path] = struct{}{}
		}
		focus.RelatedPaths = r.claimPaths(focus.RelatedPaths, local)
		if len(focus.EntryPaths) == 0 {
			continue
		}
		focuses = append(focuses, focus)
	}
	return focuses
}

func (r learningAgendaReconciler) claimPaths(paths []string, claimed map[string]struct{}) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = cleanLearningAgendaPath(path)
		if !r.inputs.contains(path) {
			continue
		}
		if _, exists := claimed[path]; exists {
			continue
		}
		claimed[path] = struct{}{}
		result = append(result, path)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (r learningAgendaReconciler) skipped(raw []agent.LearningPathSkip, focuses []domain.EvidenceFocus) []agent.LearningPathSkip {
	claimed := focusPathSet(focuses)
	result := make([]agent.LearningPathSkip, 0, len(raw))
	for _, receipt := range raw {
		receipt.Path = cleanLearningAgendaPath(receipt.Path)
		receipt.Reason = strings.TrimSpace(receipt.Reason)
		if !r.inputs.contains(receipt.Path) {
			continue
		}
		if _, exists := claimed[receipt.Path]; exists {
			continue
		}
		claimed[receipt.Path] = struct{}{}
		result = append(result, receipt)
	}
	return result
}

func (r learningAgendaReconciler) completeCoverage(focuses []domain.EvidenceFocus, skipped []agent.LearningPathSkip, fallbackPathsPerFocus int) []domain.EvidenceFocus {
	claimed := focusPathSet(focuses)
	for _, receipt := range skipped {
		claimed[receipt.Path] = struct{}{}
	}
	missing := make([]string, 0)
	for _, path := range r.inputs.paths() {
		if _, exists := claimed[path]; !exists {
			missing = append(missing, path)
		}
	}
	if len(missing) == 0 {
		return focuses
	}
	if fallbackPathsPerFocus <= 0 {
		fallbackPathsPerFocus = len(missing)
	}
	for batchIndex, start := 0, 0; start < len(missing); batchIndex, start = batchIndex+1, start+fallbackPathsPerFocus {
		end := start + fallbackPathsPerFocus
		if end > len(missing) {
			end = len(missing)
		}
		batchNumber := batchIndex + 1
		nameKey := "LearnCurrentUnassignedEvidenceFocusName"
		reasonKey := "LearnCurrentUnassignedEvidenceFocusReason"
		params := map[string]interface{}{"Batch": batchNumber}
		if len(missing) > fallbackPathsPerFocus {
			nameKey += "WithBatch"
			reasonKey += "WithBatch"
		}
		focuses = append(focuses, domain.EvidenceFocus{
			ID:            nextUnassignedEvidenceFocusID(focuses),
			Name:          i18n.GetWithParams(nameKey, params),
			Purpose:       domain.EvidenceFocusPurposeCoverage,
			AnalysisDepth: domain.EvidenceFocusDepthStandard,
			EntryPaths:    append([]string(nil), missing[start:end]...),
			ScopeReason:   i18n.GetWithParams(reasonKey, params),
		})
	}
	return focuses
}

func focusPathSet(focuses []domain.EvidenceFocus) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, focus := range focuses {
		for _, path := range focus.EntryPaths {
			paths[cleanLearningAgendaPath(path)] = struct{}{}
		}
	}
	return paths
}

func validateLearningAgendaFocuses(focuses []domain.EvidenceFocus) error {
	seenIDs := make(map[string]struct{}, len(focuses))
	for _, focus := range focuses {
		id := strings.TrimSpace(focus.ID)
		if id == "" {
			return fmt.Errorf("learning plan focus has no id")
		}
		if strings.TrimSpace(focus.Name) == "" {
			return fmt.Errorf("learning plan focus %q has no name", id)
		}
		if _, exists := seenIDs[id]; exists {
			return fmt.Errorf("learning plan repeats focus id %q", id)
		}
		seenIDs[id] = struct{}{}
	}
	return nil
}

func nextUnassignedEvidenceFocusID(focuses []domain.EvidenceFocus) string {
	used := make(map[string]struct{}, len(focuses))
	for _, focus := range focuses {
		used[focus.ID] = struct{}{}
	}
	base := "unassigned-evidence"
	if _, exists := used[base]; !exists {
		return base
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d", base, index)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func validateLearningAgendaCoverageForInputs(inputs learningAgendaInputSet, focuses []domain.EvidenceFocus, skipped []agent.LearningPathSkip) error {
	covered := make(map[string]struct{}, len(inputs))
	add := func(path, owner string) error {
		path = cleanLearningAgendaPath(path)
		if !inputs.contains(path) {
			return fmt.Errorf("learning plan %s references unknown path %q", owner, path)
		}
		if _, exists := covered[path]; exists {
			return fmt.Errorf("learning plan repeats ownership for path %q", path)
		}
		covered[path] = struct{}{}
		return nil
	}
	for _, focus := range focuses {
		for _, path := range focus.EntryPaths {
			if err := add(path, "focus"); err != nil {
				return err
			}
		}
		for _, path := range focus.RelatedPaths {
			if !inputs.contains(path) {
				return fmt.Errorf("learning plan evidence references unknown path %q", path)
			}
		}
	}
	seenSkipped := make(map[string]struct{}, len(skipped))
	for _, item := range skipped {
		path := cleanLearningAgendaPath(item.Path)
		if strings.TrimSpace(item.Reason) == "" {
			return fmt.Errorf("learning plan skipped path %q has no reason", path)
		}
		if _, exists := seenSkipped[path]; exists {
			return fmt.Errorf("learning plan repeats skipped path %q", path)
		}
		seenSkipped[path] = struct{}{}
		if _, exists := covered[path]; exists {
			return fmt.Errorf("learning plan path %q is both focused and skipped", path)
		}
		if err := add(path, "skip receipt"); err != nil {
			return err
		}
	}
	if len(covered) != len(inputs) {
		missing := make([]string, 0, len(inputs)-len(covered))
		for path := range inputs {
			if _, exists := covered[path]; !exists {
				missing = append(missing, path)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("learning plan has no decision for paths: %s", strings.Join(missing, ", "))
	}
	return nil
}

func cleanLearningAgendaPath(path string) string {
	return strings.TrimSpace(filepath.ToSlash(filepath.Clean(path)))
}
