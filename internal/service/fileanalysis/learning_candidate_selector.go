package fileanalysis

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
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

var highSignalTerms = []string{
	"entry", "main", "route", "router", "screen", "page", "view", "component",
	"workflow", "flow", "usecase", "logic", "model", "entity", "schema",
	"contract", "interface", "message", "event", "action", "command",
	"config", "setting", "policy", "permission", "auth", "audit", "validator",
	"adapter", "client", "provider", "integration", "protocol", "connector",
	"task", "job", "processor", "subscriber", "hook", "plugin", "extension",
	"store", "state", "cache", "migration", "release", "deploy", "script",
	"source", "template", "license", "key", "secret", "credential", "crypto", "certificate", "cert",
	"handler", "controller", "service", "repository", "repo", "middleware", "api",
}

var lowSignalTerms = []string{
	"test", "spec", "fixture", "mock", "stub", "readme", "doc", "docs",
	"example", "examples", "sample", "samples", "generated", "vendor",
}

var entryNames = []string{
	"main",
	"index",
	"app",
	"server",
	"bootstrap",
	"startup",
}

func PathsToFileInfos(paths []string) []domain.FileInfo {
	paths = normalizeCandidatePaths(paths)
	files := make([]domain.FileInfo, 0, len(paths))
	for _, path := range paths {
		files = append(files, domain.NewFileInfo(path, ""))
	}
	return files
}

// SelectLearningCandidates 返回保守的本地候选集。
//
// 本地阶段没有源码语义判断能力，不能因为路径词表或文件名看起来低信号就丢弃候选。
// 路径信号只用于 SelectLearningContextSeeds 挑结构化上下文入口；最终删减只能由 AI
// 候选收敛执行，且失败时由上层回退到全部候选。
func SelectLearningCandidates(opts LearningCandidateSelectionOptions) LearningCandidateSelectionResult {
	candidates := normalizeCandidatePaths(opts.Candidates)
	return LearningCandidateSelectionResult{
		SelectedPaths: candidates,
		SkippedPaths:  nil,
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
	for _, term := range highSignalTerms {
		if tokenSet[term] {
			score += 10
		}
	}
	for _, term := range lowSignalTerms {
		if tokenSet[term] {
			score -= 20
		}
	}
	name := learningCandidateBaseName(path)
	for _, entryName := range entryNames {
		if name == strings.ToLower(strings.TrimSpace(entryName)) {
			score += 8
			break
		}
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

func learningCandidateBaseName(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.ToLower(base)
}

func normalizeCandidatePaths(paths []string) []string {
	return pathx.CleanRelativeList(paths)
}

func candidatePathSet(candidates []string) map[string]bool {
	candidateSet := make(map[string]bool, len(candidates))
	for _, path := range candidates {
		candidateSet[path] = true
	}
	return candidateSet
}
