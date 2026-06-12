package fileanalysis

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/filefilter"
)

// AISelectorOptions 描述 AI 文件选择器的本地输入。
type AISelectorOptions struct {
	ProjectRoot string
	Candidates  []string
	Changes     *FileChanges
	UserContext string
}

// AISelectorResult 描述 AI 文件选择器应用后的本地结果。
type AISelectorResult struct {
	SelectedPaths []string
	SkippedPaths  []string
	Reason        string
}

// ApplyAIFileSelector 让 AI 基于候选文件树收敛本次 learn current 的分析范围。
func ApplyAIFileSelector(ctx context.Context, selector agent.FileSelector, opts AISelectorOptions) (*AISelectorResult, error) {
	candidates := normalizeCandidatePaths(opts.Candidates)
	if selector == nil || opts.ProjectRoot == "" || len(candidates) == 0 {
		return &AISelectorResult{SelectedPaths: candidates}, nil
	}

	req := &agent.SelectFilesRequest{
		FileTree:     buildCandidateTree(candidates),
		Candidates:   buildCandidateMetadata(opts.ProjectRoot, candidates, opts.Changes),
		UserContext:  opts.UserContext,
		CandidateNum: len(candidates),
	}
	aiResult, err := selector.SelectFiles(ctx, req)
	if err != nil {
		return nil, err
	}
	if aiResult == nil {
		aiResult = &agent.SelectFilesResult{}
	}
	selected := applyAISelection(candidates, aiResult)
	if len(selected) == 0 {
		selected = candidates
	}
	return &AISelectorResult{
		SelectedPaths: selected,
		SkippedPaths:  subtractPaths(candidates, selected),
		Reason:        strings.TrimSpace(aiResult.Reason),
	}, nil
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

func buildCandidateTree(paths []string) string {
	if len(paths) == 0 {
		return ".\n"
	}
	root := &candidateTreeNode{children: map[string]*candidateTreeNode{}}
	for _, path := range paths {
		current := root
		for _, part := range strings.Split(path, "/") {
			if part == "" {
				continue
			}
			child := current.children[part]
			if child == nil {
				child = &candidateTreeNode{children: map[string]*candidateTreeNode{}}
				current.children[part] = child
			}
			current = child
		}
		current.file = true
	}
	var b strings.Builder
	b.WriteString(".\n")
	writeCandidateTree(&b, root, 1)
	return b.String()
}

type candidateTreeNode struct {
	children map[string]*candidateTreeNode
	file     bool
}

func writeCandidateTree(b *strings.Builder, node *candidateTreeNode, depth int) {
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		child := node.children[name]
		b.WriteString(strings.Repeat("  ", depth))
		b.WriteString("- ")
		b.WriteString(name)
		if len(child.children) > 0 && !child.file {
			b.WriteByte('/')
		}
		b.WriteByte('\n')
		writeCandidateTree(b, child, depth+1)
	}
}

func buildCandidateMetadata(projectRoot string, paths []string, changes *FileChanges) []agent.FileSelectionCandidate {
	statusByPath := make(map[string]string)
	if changes != nil {
		for _, path := range changes.AddedOrModified {
			statusByPath[filepath.ToSlash(path)] = "changed"
		}
		for _, path := range changes.Deleted {
			statusByPath[filepath.ToSlash(path)] = "deleted"
		}
	}
	candidates := make([]agent.FileSelectionCandidate, 0, len(paths))
	for _, path := range paths {
		item := agent.FileSelectionCandidate{
			Path:    path,
			Status:  statusByPath[path],
			Kind:    candidateKind(path),
			Changed: statusByPath[path] != "",
		}
		if item.Status == "" {
			item.Status = "candidate"
		}
		if item.Status != "deleted" {
			if info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(path))); err == nil {
				item.Size = info.Size()
			}
		}
		candidates = append(candidates, item)
	}
	return candidates
}

func candidateKind(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".yaml", ".yml", ".toml", ".xml":
		return "config-or-data"
	case ".sql", ".graphql", ".gql", ".proto", ".api":
		return "schema-or-contract"
	default:
		if ext == "" {
			return "unknown"
		}
		return "source"
	}
}

func applyAISelection(candidates []string, result *agent.SelectFilesResult) []string {
	if result == nil {
		return candidates
	}
	candidateSet := make(map[string]bool, len(candidates))
	for _, path := range candidates {
		candidateSet[path] = true
	}
	selected := make(map[string]bool)
	addPath := func(path string) {
		path = cleanRelativePath(path)
		if path == "" || !candidateSet[path] {
			return
		}
		selected[path] = true
	}
	for _, path := range result.SelectedPaths {
		addPath(path)
	}
	for _, include := range result.Include {
		include = cleanPattern(include)
		if include == "" {
			continue
		}
		for _, candidate := range candidates {
			if candidateMatchesPattern(candidate, include) {
				selected[candidate] = true
			}
		}
	}
	if len(selected) == 0 && len(result.Include) == 0 && len(result.SelectedPaths) == 0 {
		for _, candidate := range candidates {
			selected[candidate] = true
		}
	}
	for _, exclude := range result.Exclude {
		exclude = cleanPattern(exclude)
		if exclude == "" {
			continue
		}
		for path := range selected {
			if candidateMatchesPattern(path, exclude) {
				delete(selected, path)
			}
		}
	}
	out := make([]string, 0, len(selected))
	for path := range selected {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func cleanPattern(pattern string) string {
	pattern = strings.TrimSpace(filepath.ToSlash(pattern))
	pattern = strings.TrimPrefix(pattern, "./")
	pattern = strings.Trim(pattern, "/")
	if pattern == "" || pattern == "." || filepath.IsAbs(pattern) || strings.HasPrefix(pattern, "../") || strings.Contains(pattern, "/../") {
		return ""
	}
	return pattern
}

func candidateMatchesPattern(path, pattern string) bool {
	if path == pattern {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if !strings.HasPrefix(path, prefix+"/") {
			return false
		}
		return !strings.Contains(strings.TrimPrefix(path, prefix+"/"), "/")
	}
	return filefilter.MatchExcluded(path, []string{pattern})
}

func subtractPaths(all, selected []string) []string {
	selectedSet := make(map[string]bool, len(selected))
	for _, path := range selected {
		selectedSet[path] = true
	}
	out := make([]string, 0)
	for _, path := range all {
		if !selectedSet[path] {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

// PathsToFileInfos 把相对路径转换成用于快照分析的文件信息。
func PathsToFileInfos(paths []string) []domain.FileInfo {
	normalized := normalizeCandidatePaths(paths)
	files := make([]domain.FileInfo, 0, len(normalized))
	for _, path := range normalized {
		files = append(files, domain.NewFileInfo(path, ""))
	}
	return files
}
