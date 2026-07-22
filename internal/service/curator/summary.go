package curator

import "github.com/silaswei-io/skills-seed/internal/domain"

func summarizeCuration(candidateCount, existingCount int, written []domain.Pattern, dropped []Drop) Summary {
	mergeCount := 0
	for _, pattern := range written {
		if sources := len(uniqueStrings(pattern.MergedFrom)); sources > 1 {
			mergeCount += sources - 1
		}
	}
	return Summary{
		TotalCandidates: candidateCount,
		TotalExisting:   existingCount,
		TotalWritten:    len(written),
		TotalDropped:    len(dropped),
		MergeCount:      mergeCount,
	}
}
