package curator

import "fmt"

func validateProposalOwnership(result *proposal) error {
	owners := make(map[string]string)
	outputIDs := make(map[string]struct{}, len(result.Patterns))
	for _, pattern := range result.Patterns {
		outputIDs[pattern.ID] = struct{}{}
		seen := make(map[string]struct{}, len(pattern.MergedFrom))
		for _, sourceID := range pattern.MergedFrom {
			if _, duplicate := seen[sourceID]; duplicate {
				return fmt.Errorf("curated pattern %q references source %q more than once", pattern.ID, sourceID)
			}
			seen[sourceID] = struct{}{}
			if owner, consumed := owners[sourceID]; consumed {
				return fmt.Errorf("source pattern %q is consumed by both %q and %q", sourceID, owner, pattern.ID)
			}
			owners[sourceID] = pattern.ID
		}
	}
	for _, dropped := range result.Dropped {
		if _, isOutput := outputIDs[dropped.ID]; isOutput {
			return fmt.Errorf("pattern %q is both an output and dropped", dropped.ID)
		}
	}
	return nil
}
