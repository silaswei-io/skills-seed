package analyzer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/utils/filefilter"
)

// structuralContextRequest 定义结构化分析所需的输入参数。
type structuralContextRequest struct {
	ProjectName string
	Language    string
	Purpose     string
	FocusPaths  []string
	SeedPaths   []string
}

// structuralCollector 从源码文件中提取符号、import 和入口点。
type structuralCollector interface {
	Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error)
}

// treesitterCollector 基于内嵌 tree-sitter 实现 structuralCollector。
type treesitterCollector struct {
	maxSymbols  int
	maxFileSize int64
	exclude     []string
	excludeDirs map[string]bool
}

// fileResult 保存单个文件的结构化提取结果。
type fileResult struct {
	relPath  string
	langName string
	symbols  []symbolInfo
	imports  []importInfo
}

type seedRoot struct {
	path  string
	isDir bool
}

func newStructuralCollector(cfg config.StructuralConfig, exclude []string) structuralCollector {
	// maxFileSize 默认限制单文件 512KB，避免大文件拖慢 tree-sitter 解析。
	maxFileSize := int64(512 * 1024)
	if cfg.MaxFileSize > 0 {
		maxFileSize = int64(cfg.MaxFileSize) * 1024
	}

	// excludeDirs 合并固定元数据目录和配置中的简单目录排除规则。
	excludeDirs := map[string]bool{
		".git": true, ".skills-seed": true, ".idea": true, ".vscode": true,
		".cache": true,
	}
	for _, pattern := range exclude {
		clean := strings.TrimSpace(pattern)
		// 只把 vendor、node_modules、build 这类简单目录名加入快速跳过列表。
		if !strings.Contains(clean, "/") && !strings.Contains(clean, "*") && !strings.Contains(clean, ".") {
			excludeDirs[clean] = true
		}
	}

	return &treesitterCollector{
		maxSymbols:  cfg.MaxSymbols,
		maxFileSize: maxFileSize,
		exclude:     exclude,
		excludeDirs: excludeDirs,
	}
}

// Collect 遍历项目树、解析源码文件，并返回 Markdown 格式的结构化上下文。
func (c *treesitterCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	startedAt := time.Now()
	seedRoots := c.boundedSeedRoots(projectRoot, req.SeedPaths)
	if len(seedRoots) == 0 {
		return "", nil
	}

	var results []fileResult
	langCounts := map[string]int{}
	var stats grammars.WalkStats
	maxSymbols := c.maxSymbols
	if maxSymbols <= 0 {
		// maxSymbols 默认最多输出 30 个符号，控制提示词上下文体积。
		maxSymbols = 30
	}

	for _, seedRoot := range seedRoots {
		seedResults, seedLangCounts, seedStats := c.collectSeed(ctx, projectRoot, seedRoot)
		results = append(results, seedResults...)
		for lang, count := range seedLangCounts {
			langCounts[lang] += count
		}
		stats.FilesFound += seedStats.FilesFound
		stats.FilesParsed += seedStats.FilesParsed
		stats.FilesFailed += seedStats.FilesFailed
		stats.FilesFiltered += seedStats.FilesFiltered
		stats.LargeFiles += seedStats.LargeFiles
		stats.BinarySkipped += seedStats.BinarySkipped
		stats.BytesParsed += seedStats.BytesParsed
	}

	result := c.assembleMarkdown(results, langCounts, stats, maxSymbols)

	logger.Diagnostic("operation complete",
		"operation", "analyzer.treesitter_collect",
		"duration", time.Since(startedAt),
		"context_bytes", len(result),
		"project_root", projectRoot,
		"seed_roots", len(seedRoots),
		"files_parsed", stats.FilesParsed,
		"languages", len(langCounts),
	)

	return result, nil
}

func (c *treesitterCollector) collectSeed(ctx context.Context, projectRoot string, seed seedRoot) ([]fileResult, map[string]int, grammars.WalkStats) {
	if seed.isDir {
		return c.collectFromRoot(ctx, projectRoot, seed.path)
	}
	result, ok := c.collectFile(projectRoot, seed.path)
	stats := grammars.WalkStats{FilesFound: 1}
	if !ok {
		stats.FilesFiltered = 1
		return nil, nil, stats
	}
	stats.FilesParsed = 1
	stats.BytesParsed = fileSize(seed.path)
	return []fileResult{result}, map[string]int{result.langName: 1}, stats
}

func (c *treesitterCollector) collectFile(projectRoot, path string) (fileResult, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > c.maxFileSize {
		return fileResult{}, false
	}
	relPath, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return fileResult{}, false
	}
	relPath = filepath.ToSlash(relPath)
	if filefilter.MatchExcluded(relPath, c.exclude) {
		return fileResult{}, false
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return fileResult{}, false
	}
	tree, lang, err := parseTree(path, src)
	if err != nil || tree == nil || lang == nil {
		return fileResult{}, false
	}
	defer tree.Release()
	langEntry := grammars.DetectLanguage(filepath.Base(path))
	langName := ""
	if langEntry != nil {
		langName = langEntry.Name
	}
	if langName == "" {
		return fileResult{}, false
	}
	root := tree.RootNode()
	return fileResult{
		relPath:  relPath,
		langName: langName,
		symbols:  extractSymbols(root, lang, src, langName),
		imports:  extractImports(root, lang, src, langName),
	}, true
}

func (c *treesitterCollector) collectFromRoot(ctx context.Context, projectRoot, seedRoot string) ([]fileResult, map[string]int, grammars.WalkStats) {
	policy := c.parsePolicy(projectRoot)

	ch, statsFn := grammars.WalkAndParse(ctx, seedRoot, policy)

	var results []fileResult
	langCounts := map[string]int{}

	for pf := range ch {
		if pf.Err != nil || pf.Tree == nil {
			pf.Close()
			continue
		}

		relPath, _ := filepath.Rel(projectRoot, pf.Path)
		relPath = filepath.ToSlash(relPath)
		langName := pf.Lang.Name
		lang := pf.Lang.Language()
		src := pf.Source
		root := pf.Tree.RootNode()

		syms := extractSymbols(root, lang, src, langName)
		imps := extractImports(root, lang, src, langName)

		langCounts[langName]++
		results = append(results, fileResult{
			relPath:  relPath,
			langName: langName,
			symbols:  syms,
			imports:  imps,
		})

		pf.Close()
	}

	return results, langCounts, statsFn()
}

func (c *treesitterCollector) parsePolicy(projectRoot string) grammars.ParsePolicy {
	policy := grammars.DefaultPolicy()
	policy.SkipDirs = c.buildSkipDirs()
	policy.ShouldSkipDir = func(path string) bool {
		return c.excludeDirs[filepath.Base(path)]
	}
	policy.ShouldParse = func(path string, size int64, _ time.Time) bool {
		if size > c.maxFileSize {
			return false
		}
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return true
		}
		return !filefilter.MatchExcluded(filepath.ToSlash(relPath), c.exclude)
	}
	return policy
}

func (c *treesitterCollector) boundedSeedRoots(projectRoot string, seedPaths []string) []seedRoot {
	roots := make([]string, 0, len(seedPaths))
	seen := make(map[string]bool)
	for _, seed := range seedPaths {
		relPath := strings.TrimSpace(filepath.ToSlash(seed))
		if relPath == "" || filepath.IsAbs(relPath) {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(relPath))
		if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			continue
		}
		absPath := filepath.Join(projectRoot, clean)
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(projectRoot, absPath)
			if err != nil || filefilter.MatchExcluded(filepath.ToSlash(rel), c.exclude) || info.Size() > c.maxFileSize {
				continue
			}
		}
		if seen[absPath] {
			continue
		}
		seen[absPath] = true
		roots = append(roots, absPath)
	}
	out := make([]seedRoot, 0, len(roots))
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		out = append(out, seedRoot{path: root, isDir: info.IsDir()})
	}
	return out
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (c *treesitterCollector) buildSkipDirs() []string {
	dirs := []string{".git", ".skills-seed", ".hg", ".svn"}
	for d := range c.excludeDirs {
		found := false
		for _, existing := range dirs {
			if existing == d {
				found = true
				break
			}
		}
		if !found {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

func (c *treesitterCollector) assembleMarkdown(
	results []fileResult,
	langCounts map[string]int,
	stats grammars.WalkStats,
	maxSymbols int,
) string {
	var b strings.Builder
	b.WriteString("## Structural Context\n\n")

	// 状态区：输出扫描文件数、解析文件数和语言分布。
	b.WriteString("### Status\n\n")
	b.WriteString(fmt.Sprintf("Files scanned: %d | Files parsed: %d | Languages: %s\n\n",
		stats.FilesFound, stats.FilesParsed, formatLangCounts(langCounts)))

	// 符号区：按 maxSymbols 限制输出数量。
	b.WriteString("### Symbols\n\n")
	printed := 0
	var entryPoints []string

	// 按文件路径排序，保证输出稳定。
	sort.Slice(results, func(i, j int) bool {
		return results[i].relPath < results[j].relPath
	})

	for _, fr := range results {
		if len(fr.symbols) == 0 {
			continue
		}
		if printed >= maxSymbols {
			b.WriteString("... (truncated)\n\n")
			break
		}

		b.WriteString(fmt.Sprintf("#### %s: %s\n", fr.langName, fr.relPath))
		for _, sym := range fr.symbols {
			if printed >= maxSymbols {
				break
			}
			b.WriteString(fmt.Sprintf("- %s %s (line %d)\n", sym.Kind, sym.Name, sym.Line))
			if isEntryPoint(sym.Name) {
				entryPoints = append(entryPoints, fmt.Sprintf("- %s %s (%s:%d)", sym.Kind, sym.Name, fr.relPath, sym.Line))
			}
			printed++
		}
		b.WriteByte('\n')
	}

	// import 区：只输出简要依赖路径。
	b.WriteString("### Imports\n\n")
	importPrinted := 0
	for _, fr := range results {
		if len(fr.imports) == 0 {
			continue
		}
		if importPrinted >= maxSymbols {
			break
		}
		b.WriteString(fmt.Sprintf("#### %s: %s\n", fr.langName, fr.relPath))
		for _, imp := range fr.imports {
			if importPrinted >= maxSymbols {
				break
			}
			b.WriteString(fmt.Sprintf("- %s\n", imp.Path))
			importPrinted++
		}
		b.WriteByte('\n')
	}

	// 入口点区：集中列出 main 等入口符号。
	if len(entryPoints) > 0 {
		b.WriteString("### Entry Points\n\n")
		for _, ep := range entryPoints {
			b.WriteString(ep)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func formatLangCounts(counts map[string]int) string {
	type kv struct {
		lang  string
		count int
	}
	var kvs []kv
	for k, v := range counts {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].count > kvs[j].count
	})
	parts := make([]string, len(kvs))
	for i, kv := range kvs {
		parts[i] = fmt.Sprintf("%s(%d)", kv.lang, kv.count)
	}
	return strings.Join(parts, ", ")
}

// parseTree 是测试辅助方法：解析源码并返回 tree-sitter 语法树。
func parseTree(filename string, src []byte) (*gotreesitter.Tree, *gotreesitter.Language, error) {
	entry := grammars.DetectLanguage(filename)
	if entry == nil {
		return nil, nil, fmt.Errorf("unsupported file type: %s", filename)
	}
	lang := entry.Language()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, err
	}
	return tree, lang, nil
}
