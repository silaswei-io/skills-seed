package fileanalysis

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type LearningCandidateSelectionOptions struct {
	Candidates    []string
	Changes       *FileChanges
	RequiredPaths []string
}

type LearningCandidateSelectionResult struct {
	SelectedPaths []string
	SkippedPaths  []string
	Reason        string
}

func PathsToFileInfos(paths []string) []domain.FileInfo {
	paths = normalizeCandidatePaths(paths)
	files := make([]domain.FileInfo, 0, len(paths))
	for _, path := range paths {
		files = append(files, domain.NewFileInfo(path, ""))
	}
	return files
}

// SelectLearningCandidates 使用本地确定性信号收敛 learn current 的候选文件。
func SelectLearningCandidates(opts LearningCandidateSelectionOptions) LearningCandidateSelectionResult {
	candidates := normalizeCandidatePaths(opts.Candidates)
	selected := localLearningCandidateSelection(candidates, opts.Changes, opts.RequiredPaths)
	return LearningCandidateSelectionResult{
		SelectedPaths: selected,
		SkippedPaths:  subtractPaths(candidates, selected),
		Reason:        "local deterministic candidate selection",
	}
}

// SelectLearningContextSeeds 返回适合结构化上下文采集的高信号入口。
//
// 这里不做最终候选收敛；完整候选集仍交给 AI 判断。本函数只避免把整批候选文件
// 当作结构分析入口，导致 CodeGraph 或 tree-sitter 在大项目上执行无价值的大范围扫描。
func SelectLearningContextSeeds(opts LearningCandidateSelectionOptions) []string {
	candidates := normalizeCandidatePaths(opts.Candidates)
	candidateSet := candidatePathSet(candidates)
	selected := make(map[string]bool)
	add := func(paths []string) {
		for _, path := range normalizeCandidatePaths(paths) {
			if candidateSet[path] {
				selected[path] = true
			}
		}
	}

	add(opts.RequiredPaths)
	add(localChangedCandidatePaths(candidates, opts.Changes))
	add(highSignalCandidatePaths(candidates))
	return sortedPathMap(selected)
}

func localLearningCandidateSelection(candidates []string, changes *FileChanges, requiredPaths []string) []string {
	candidateSet := candidatePathSet(candidates)
	selected := make(map[string]bool)
	add := func(paths []string) {
		for _, path := range normalizeCandidatePaths(paths) {
			if candidateSet[path] {
				selected[path] = true
			}
		}
	}

	add(requiredPaths)
	add(localChangedCandidatePaths(candidates, changes))
	add(highSignalCandidatePaths(candidates))
	if len(selected) == 0 {
		add(candidates)
	}
	return sortedPathMap(selected)
}

func localChangedCandidatePaths(candidates []string, changes *FileChanges) []string {
	if changes == nil {
		return nil
	}
	changed := normalizeCandidatePaths(changes.AddedOrModified)
	if len(changed) >= len(candidates) {
		return nil
	}
	return changed
}

func highSignalCandidatePaths(paths []string) []string {
	selected := make([]string, 0)
	for _, path := range normalizeCandidatePaths(paths) {
		if learningCandidatePathScore(path) > 0 {
			selected = append(selected, path)
		}
	}
	sort.Strings(selected)
	return selected
}

func learningCandidatePathScore(path string) int {
	score := 0
	tokens := learningCandidatePathTokens(path)
	tokenSet := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		tokenSet[token] = true
	}
	highSignalTerms := []string{
		"route", "router", "handler", "controller", "workflow", "service", "usecase",
		"logic", "model", "entity", "schema", "config", "task", "job", "event",
		"subscriber", "adapter", "client", "provider", "middleware", "policy",
		"validator", "repository", "repo", "store", "migration", "proto", "api",
		"command", "action", "processor", "manager", "permission", "audit",
		"license", "certificate", "cert", "cluster", "snmp", "plugin", "hook",
	}
	for _, term := range highSignalTerms {
		if tokenSet[term] {
			score += 10
		}
	}
	lowSignalTerms := []string{
		"test", "spec", "fixture", "mock", "stub", "readme", "doc", "docs",
		"example", "examples", "sample", "samples", "generated", "vendor",
	}
	for _, term := range lowSignalTerms {
		if tokenSet[term] {
			score -= 20
		}
	}
	lower := strings.ToLower(filepath.ToSlash(path))
	if strings.HasSuffix(lower, "/main.go") || strings.HasSuffix(lower, "/main.ts") ||
		strings.HasSuffix(lower, "/index.ts") || strings.HasSuffix(lower, "/index.js") {
		score += 8
	}
	return score
}

func learningCandidatePathTokens(path string) []string {
	path = strings.ToLower(strings.TrimSuffix(filepath.ToSlash(path), filepath.Ext(path)))
	fields := strings.FieldsFunc(path, func(r rune) bool {
		switch r {
		case '/', '\\', '.', '-', '_', ' ', '\t':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func normalizeCandidatePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = cleanRelativePath(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func cleanRelativePath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "./")
	path = strings.Trim(path, "/")
	if path == "" || path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func candidatePathSet(candidates []string) map[string]bool {
	candidateSet := make(map[string]bool, len(candidates))
	for _, path := range candidates {
		candidateSet[path] = true
	}
	return candidateSet
}

func subtractPaths(all, selected []string) []string {
	selectedSet := pathSet(selected)
	out := make([]string, 0)
	for _, path := range normalizeCandidatePaths(all) {
		if !selectedSet[path] {
			out = append(out, path)
		}
	}
	return out
}
