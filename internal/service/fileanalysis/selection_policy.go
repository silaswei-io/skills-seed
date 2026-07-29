package fileanalysis

import (
	"sort"
)

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
