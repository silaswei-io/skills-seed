package curator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func summarizeCuration(candidateCount, existingCount int, written []domain.Pattern, dropped []Drop) Summary {
	mergeCount := 0
	for _, pattern := range written {
		if sources := len(stringx.UniqueNonEmpty(pattern.MergedFrom)); sources > 1 {
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
