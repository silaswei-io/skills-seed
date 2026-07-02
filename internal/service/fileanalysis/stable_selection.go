package fileanalysis

import (
	"path/filepath"
	"sort"
	"strings"
)

const (
	// 大候选集下启用最小预算，避免 AI 一次建议过窄导致学习覆盖面大幅波动。
	stableSelectionBudgetCandidateThreshold = 100
	// 大候选集下至少保留的候选比例。
	stableSelectionMinRatio = 0.10
	// 大候选集下最少补足到的文件数。
	stableSelectionMinCount = 50
	// 最小预算上限，避免本地补足把分析范围扩大到不可控。
	stableSelectionMaxBudget = 200
)

type stableSelectionOptions struct {
	Candidates    []string
	AISelected    []string
	RequiredPaths []string
}

func applyStableSelectionPolicy(opts stableSelectionOptions) ([]string, []string) {
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

	forced := make(map[string]bool)
	for _, path := range normalizeCandidatePaths(opts.RequiredPaths) {
		if candidateSet[path] {
			selected[path] = true
			forced[path] = true
		}
	}

	if len(selected) == 0 {
		for _, path := range candidates {
			selected[path] = true
			forced[path] = true
		}
	} else {
		fillStableSelectionBudget(candidates, selected)
	}

	return sortedPathMap(selected), sortedPathMap(forced)
}

func fillStableSelectionBudget(candidates []string, selected map[string]bool) {
	budget := stableSelectionBudget(len(candidates))
	if budget <= 0 || len(selected) >= budget {
		return
	}
	fillers := append([]string(nil), candidates...)
	sort.SliceStable(fillers, func(i, j int) bool {
		leftRank, rightRank := stableSelectionFillRank(fillers[i]), stableSelectionFillRank(fillers[j])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftDepth, rightDepth := strings.Count(fillers[i], "/"), strings.Count(fillers[j], "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return fillers[i] < fillers[j]
	})
	for _, path := range fillers {
		selected[path] = true
		if len(selected) >= budget {
			return
		}
	}
}

func stableSelectionBudget(candidateCount int) int {
	if candidateCount < stableSelectionBudgetCandidateThreshold {
		return 0
	}
	budget := int(float64(candidateCount) * stableSelectionMinRatio)
	if budget < stableSelectionMinCount {
		budget = stableSelectionMinCount
	}
	if budget > stableSelectionMaxBudget {
		budget = stableSelectionMaxBudget
	}
	if budget > candidateCount {
		return candidateCount
	}
	return budget
}

func stableSelectionFillRank(path string) int {
	switch candidateKind(path) {
	case "source":
		return 0
	case "schema-or-contract":
		return 1
	case "config-or-data":
		return 2
	default:
		if filepath.Ext(path) == "" {
			return 4
		}
		return 3
	}
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
