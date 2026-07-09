package analyzer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
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

type structuralProvider interface {
	Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error)
}

type policyAwareStructuralProvider interface {
	withPolicy(policy fileanalysis.SelectionPolicy) structuralProvider
}

type renderedStructuralCollector struct {
	provider   structuralProvider
	renderer   structuralRenderer
	maxSymbols int
}

func (c *renderedStructuralCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	data, err := c.provider.Collect(ctx, projectRoot, req)
	if err != nil || data == nil {
		return "", err
	}
	return c.renderer.Render(data, c.maxSymbols), nil
}

func (c *renderedStructuralCollector) withPolicy(policy fileanalysis.SelectionPolicy) *renderedStructuralCollector {
	next := *c
	if aware, ok := next.provider.(policyAwareStructuralProvider); ok {
		next.provider = aware.withPolicy(policy)
	}
	return &next
}

// treesitterCollector 基于内嵌 tree-sitter 实现 structuralProvider。
type treesitterCollector struct {
	maxSymbols  int
	maxFileSize int64
	policy      fileanalysis.SelectionPolicy
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

func newStructuralCollector(cfg config.StructuralConfig) structuralCollector {
	return &renderedStructuralCollector{
		provider:   newStructuralProvider(cfg),
		renderer:   structuralRenderer{},
		maxSymbols: cfg.MaxSymbols,
	}
}

func newTreeSitterProvider(cfg config.StructuralConfig) *treesitterCollector {
	// maxFileSize 默认限制单文件 512KB，避免大文件拖慢 tree-sitter 解析。
	maxFileSize := int64(512 * 1024)
	if cfg.MaxFileSize > 0 {
		maxFileSize = int64(cfg.MaxFileSize) * 1024
	}

	return &treesitterCollector{
		maxSymbols:  cfg.MaxSymbols,
		maxFileSize: maxFileSize,
		policy:      fileanalysis.NewSelectionPolicy(nil),
	}
}

func (c *treesitterCollector) withPolicy(policy fileanalysis.SelectionPolicy) structuralProvider {
	next := *c
	next.policy = policy
	return &next
}

// Collect 遍历项目树、解析源码文件，并返回统一结构化上下文。
func (c *treesitterCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error) {
	startedAt := time.Now()
	seedRoots := c.boundedSeedRoots(projectRoot, req.SeedPaths)
	if len(seedRoots) == 0 {
		return nil, nil
	}

	var results []fileResult
	langCounts := map[string]int{}
	var stats grammars.WalkStats
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

	data := c.toStructuralContext(results, langCounts, stats)

	logger.Diagnostic("operation complete",
		"operation", "analyzer.treesitter_collect",
		"duration", time.Since(startedAt),
		"project_root", projectRoot,
		"seed_roots", len(seedRoots),
		"files_parsed", stats.FilesParsed,
		"languages", len(langCounts),
	)

	return data, nil
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
	if c.policy.IsExcluded(relPath) {
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
	policy.ShouldSkipDir = func(path string) bool {
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return false
		}
		return c.policy.IsExcluded(filepath.ToSlash(relPath))
	}
	policy.ShouldParse = func(path string, size int64, _ time.Time) bool {
		if size > c.maxFileSize {
			return false
		}
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return true
		}
		return !c.policy.IsExcluded(filepath.ToSlash(relPath))
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
			if err != nil || c.policy.IsExcluded(filepath.ToSlash(rel)) || info.Size() > c.maxFileSize {
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

func (c *treesitterCollector) toStructuralContext(
	results []fileResult,
	langCounts map[string]int,
	stats grammars.WalkStats,
) *structuralContextData {
	data := &structuralContextData{
		Source:      structuralProviderTreeSitter,
		FilesFound:  stats.FilesFound,
		FilesParsed: stats.FilesParsed,
		LangCounts:  langCounts,
	}
	for _, fr := range results {
		for _, sym := range fr.symbols {
			data.Symbols = append(data.Symbols, structuralSymbol{
				Path: fr.relPath,
				Lang: fr.langName,
				Name: sym.Name,
				Kind: sym.Kind,
				Line: sym.Line,
			})
			if isEntryPoint(sym.Name) {
				data.EntryPoints = append(data.EntryPoints, structuralEntryPoint{
					Path: fr.relPath,
					Kind: sym.Kind,
					Name: sym.Name,
					Line: sym.Line,
				})
			}
		}
		for _, imp := range fr.imports {
			data.Imports = append(data.Imports, structuralImport{
				Path:       fr.relPath,
				Lang:       fr.langName,
				ImportPath: imp.Path,
			})
		}
	}
	return data
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
