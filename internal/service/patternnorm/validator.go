package patternnorm

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/patternview"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

func validateCandidates(candidates []domain.Pattern) []domain.Pattern {
	valid := make([]domain.Pattern, 0, len(candidates))
	for _, candidate := range candidates {
		if !domain.ValidKnowledgeFlags(candidate.KnowledgeFlags) {
			continue
		}
		pattern := patternview.Normalize(candidate)
		if pattern.IsValid() && !hasPlaceholderExample(pattern.GoodExample) {
			valid = append(valid, pattern)
		}
	}
	return valid
}

func (s *Service) validateCurrentCandidates(candidates []domain.Pattern) []domain.Pattern {
	valid := make([]domain.Pattern, 0, len(candidates))
	for _, candidate := range candidates {
		evidenceCount := domain.PatternEvidenceFileCount(candidate.EvidenceLocations)
		if evidenceCount == 0 || candidate.Confidence < s.admission.MinConfidence {
			continue
		}
		if evidenceCount == 1 && candidate.Confidence < s.admission.MinSingleEvidenceConfidence {
			continue
		}
		valid = append(valid, candidate)
	}
	return valid
}

// prepareCurrentCandidates 将本轮源码候选整理为独立的规范化输入。
// MergedFrom 是既有规范化结果的输出谱系，不能作为新一轮源码事实的输入身份。
func (s *Service) prepareCurrentCandidates(candidates []domain.Pattern) []domain.Pattern {
	candidates = s.validateCurrentCandidates(validateCandidates(candidates))
	for index := range candidates {
		candidates[index] = patternview.WithSources(candidates[index], []string{candidates[index].ID})
	}
	return coalesceCurrentCandidates(candidates)
}

func coalesceCurrentCandidates(candidates []domain.Pattern) []domain.Pattern {
	coalesced := make([]domain.Pattern, 0, len(candidates))
	indexByID := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		index, exists := indexByID[candidate.ID]
		if !exists {
			indexByID[candidate.ID] = len(coalesced)
			coalesced = append(coalesced, candidate)
			continue
		}

		previous := coalesced[index]
		merged := patternview.MergeKeepingBest(previous, candidate)
		merged.ProjectID = commonPairValue(previous.ProjectID, candidate.ProjectID)
		merged.ScopePath = commonPairValue(previous.ScopePath, candidate.ScopePath)
		merged.WorkspaceRole = commonPairValue(previous.WorkspaceRole, candidate.WorkspaceRole)
		coalesced[index] = merged
	}
	return coalesced
}

func commonPairValue(left, right string) string {
	left = strings.TrimSpace(left)
	if left == strings.TrimSpace(right) {
		return left
	}
	return ""
}

func validateNormalizeResultForOperation(operation Operation, result *proposal, candidates, existing []domain.Pattern) error {
	if operation == OperationLearnCurrent {
		if err := validateCapabilityEntryMerges(result, candidates, existing); err != nil {
			return err
		}
		if err := hydrateNormalizeResult(result, candidates, existing); err != nil {
			return err
		}
	}
	return validateNormalizeResult(result, candidates, existing)
}

func validateCapabilityEntryMerges(result *proposal, candidates, existing []domain.Pattern) error {
	if result == nil {
		return fmt.Errorf("normalization result is nil")
	}

	inputs := indexNormalizationSources(candidates, existing)
	for _, pattern := range result.Patterns {
		var entries []capabilityEntryIdentity
		for _, sourceID := range pattern.MergedFrom {
			if entry, ok := newCapabilityEntryIdentity(inputs[sourceID].BusinessMethod); ok {
				entries = append(entries, entry)
			}
		}
		for left := 0; left < len(entries); left++ {
			for right := left + 1; right < len(entries); right++ {
				if entries[left].distinctFrom(entries[right]) {
					return fmt.Errorf("normalized pattern %q merges distinct capability entries %q and %q", pattern.ID, entries[left].name, entries[right].name)
				}
			}
		}
	}
	return nil
}

type capabilityEntryIdentity struct {
	name      string
	locations map[string]struct{}
}

func newCapabilityEntryIdentity(method *domain.BusinessMethod) (capabilityEntryIdentity, bool) {
	if method == nil {
		return capabilityEntryIdentity{}, false
	}
	name := strings.ToLower(strings.TrimSpace(method.Name))
	if name == "" {
		return capabilityEntryIdentity{}, false
	}
	locations := make(map[string]struct{}, 2)
	for _, value := range []string{method.CodeLocation.CurrentLocation, method.CodeLocation.HistoricalLocation} {
		if location := strings.ToLower(pathx.CleanEvidenceLocationPath(value)); location != "" {
			locations[location] = struct{}{}
		}
	}
	return capabilityEntryIdentity{name: name, locations: locations}, true
}

func (left capabilityEntryIdentity) distinctFrom(right capabilityEntryIdentity) bool {
	if left.name != right.name {
		return true
	}
	if len(left.locations) == 0 || len(right.locations) == 0 {
		return false
	}
	for location := range left.locations {
		if _, ok := right.locations[location]; ok {
			return false
		}
	}
	return true
}

func validateNormalizeResult(result *proposal, candidates, existing []domain.Pattern) error {
	if result == nil {
		return fmt.Errorf("normalization result is nil")
	}

	state := newNormalizeValidationState(candidates, existing, len(result.Patterns))
	for i := range result.Patterns {
		if err := state.validatePattern(&result.Patterns[i]); err != nil {
			return err
		}
	}
	if err := state.validateDropped(result.Dropped); err != nil {
		return err
	}
	if err := state.validateCandidateCoverage(); err != nil {
		return err
	}
	return validateProposalOwnership(result)
}

type normalizeValidationState struct {
	candidateIDs      map[string]struct{}
	allIDs            map[string]struct{}
	coveredCandidates map[string]struct{}
	mergedCandidates  map[string]struct{}
	outputIDs         map[string]struct{}
}

func newNormalizeValidationState(candidates, existing []domain.Pattern, outputCount int) normalizeValidationState {
	candidateIDs := patternIDSet(candidates)
	existingIDs := patternIDSet(existing)
	allIDs := make(map[string]struct{}, len(candidateIDs)+len(existingIDs))
	for id := range candidateIDs {
		allIDs[id] = struct{}{}
	}
	for id := range existingIDs {
		allIDs[id] = struct{}{}
	}
	addPatternSourceIDs(allIDs, candidates)
	addPatternSourceIDs(allIDs, existing)
	return normalizeValidationState{
		candidateIDs:      candidateIDs,
		allIDs:            allIDs,
		coveredCandidates: make(map[string]struct{}, len(candidateIDs)),
		mergedCandidates:  make(map[string]struct{}, len(candidateIDs)),
		outputIDs:         make(map[string]struct{}, outputCount),
	}
}

func addPatternSourceIDs(ids map[string]struct{}, patterns []domain.Pattern) {
	for _, pattern := range patterns {
		for _, sourceID := range pattern.MergedFrom {
			sourceID = strings.TrimSpace(sourceID)
			if sourceID != "" {
				ids[sourceID] = struct{}{}
			}
		}
	}
}

func (s normalizeValidationState) validatePattern(pattern *domain.Pattern) error {
	pattern.Category = domain.NormalizePatternCategory(pattern.Category)
	if strings.TrimSpace(pattern.ID) == "" {
		return fmt.Errorf("normalized pattern has empty id")
	}
	if _, exists := s.outputIDs[pattern.ID]; exists {
		return fmt.Errorf("duplicate normalized pattern id %q", pattern.ID)
	}
	s.outputIDs[pattern.ID] = struct{}{}
	if !domain.IsValidPatternCategory(pattern.Category) {
		return fmt.Errorf("normalized pattern %q has invalid category %q", pattern.ID, pattern.Category)
	}
	if !domain.ValidKnowledgeFlags(pattern.KnowledgeFlags) {
		return fmt.Errorf("normalized pattern %q has invalid knowledge flags", pattern.ID)
	}
	if strings.TrimSpace(pattern.Name) == "" {
		return fmt.Errorf("normalized pattern %q has empty name", pattern.ID)
	}
	if strings.TrimSpace(pattern.Rule) == "" {
		return fmt.Errorf("normalized pattern %q has empty rule", pattern.ID)
	}
	if pattern.Confidence < 0 || pattern.Confidence > 1 {
		return fmt.Errorf("normalized pattern %q has confidence outside [0,1]", pattern.ID)
	}
	if hasPlaceholderExample(pattern.GoodExample) {
		return fmt.Errorf("normalized pattern %q has placeholder good example", pattern.ID)
	}
	return s.validateMergedFrom(pattern)
}

func (s normalizeValidationState) validateMergedFrom(pattern *domain.Pattern) error {
	for _, id := range pattern.MergedFrom {
		if _, ok := s.allIDs[id]; !ok {
			return fmt.Errorf("normalized pattern %q references unknown merged_from id %q", pattern.ID, id)
		}
		if _, ok := s.candidateIDs[id]; ok {
			s.coveredCandidates[id] = struct{}{}
			s.mergedCandidates[id] = struct{}{}
		}
	}
	return nil
}

func (s normalizeValidationState) validateDropped(droppedPatterns []Drop) error {
	droppedIDs := make(map[string]struct{}, len(droppedPatterns))
	for _, dropped := range droppedPatterns {
		if _, ok := s.candidateIDs[dropped.ID]; !ok {
			return fmt.Errorf("dropped pattern id %q is not a current candidate id; dropped may only reference current candidates", dropped.ID)
		}
		if _, exists := droppedIDs[dropped.ID]; exists {
			return fmt.Errorf("duplicate dropped candidate id %q", dropped.ID)
		}
		if _, merged := s.mergedCandidates[dropped.ID]; merged {
			return fmt.Errorf("candidate pattern %q is both merged and dropped", dropped.ID)
		}
		if !dropped.ReasonCode.Valid() {
			return fmt.Errorf("dropped pattern %q has invalid reason_code %q", dropped.ID, dropped.ReasonCode)
		}
		if strings.TrimSpace(dropped.Reason) == "" {
			return fmt.Errorf("dropped pattern %q has empty reason", dropped.ID)
		}
		droppedIDs[dropped.ID] = struct{}{}
		s.coveredCandidates[dropped.ID] = struct{}{}
	}
	return nil
}

func (s normalizeValidationState) validateCandidateCoverage() error {
	for id := range s.candidateIDs {
		if _, ok := s.coveredCandidates[id]; !ok {
			return fmt.Errorf("candidate pattern %q is not covered by normalized result", id)
		}
	}
	return nil
}

func patternIDSet(patterns []domain.Pattern) map[string]struct{} {
	result := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		if pattern.ID == "" {
			continue
		}
		result[pattern.ID] = struct{}{}
	}
	return result
}

func hasPlaceholderExample(example string) bool {
	trimmed := strings.TrimSpace(example)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "/* ... */") || strings.Contains(trimmed, "{ ... }") {
		return true
	}
	lines := strings.Split(trimmed, "\n")
	placeholderLines := 0
	for _, line := range lines {
		normalized := strings.TrimSpace(line)
		if normalized == "..." || normalized == "// ..." || normalized == "# ..." {
			placeholderLines++
		}
	}
	return placeholderLines > 0 && placeholderLines == len(lines)
}
