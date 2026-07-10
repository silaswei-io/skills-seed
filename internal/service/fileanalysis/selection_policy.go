package fileanalysis

import (
	"sort"
)

type validatedSelectionOptions struct {
	Candidates    []string
	AISelected    []string
	AIExcluded    []string
	RequiredPaths []string
	AllowFallback bool
}

func applyValidatedSelectionPolicy(opts validatedSelectionOptions) ([]string, []string) {
	candidates := normalizeCandidatePaths(opts.Candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	candidateSet := pathSet(candidates)
	selected := make(map[string]bool, len(candidates))
	for _, path := range normalizeCandidatePaths(opts.AISelected) {
		if candidateSet[path] {
			selected[path] = true
		}
	}
	excluded := pathSet(opts.AIExcluded)

	forced := make(map[string]bool)
	for _, path := range normalizeCandidatePaths(opts.RequiredPaths) {
		if candidateSet[path] {
			selected[path] = true
			forced[path] = true
		}
	}

	if len(selected) == 0 && opts.AllowFallback {
		for _, path := range candidates {
			if !excluded[path] {
				selected[path] = true
				forced[path] = true
			}
		}
	}

	return sortedPathMap(selected), sortedPathMap(forced)
}

func pathSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range normalizeCandidatePaths(paths) {
		set[path] = true
	}
	return set
}

func sortedPathMap(paths map[string]bool) []string {
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
