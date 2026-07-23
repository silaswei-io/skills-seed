package sourcecode

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

// SymbolCollector 按文件批量读取源码符号事实。
type SymbolCollector interface {
	Collect(context.Context, string, []string) (Catalog, error)
}

// NewSymbolCollector 按结构化 provider 创建本地符号事实入口。
func NewSymbolCollector(cfg config.StructuralConfig) SymbolCollector {
	provider := config.NormalizeStructuralProvider(string(cfg.Provider))
	local := fileSymbolCollector{}
	if provider == config.StructuralProviderTreeSitter {
		return local
	}
	collector := &codeGraphSymbolCollector{client: codegraph.NewClient("codegraph"), readiness: map[string]error{}, attempted: map[string]bool{}}
	if provider == config.StructuralProviderAuto {
		return fallbackSymbolCollector{primary: collector, fallback: local}
	}
	return collector
}

type fallbackSymbolCollector struct {
	primary  SymbolCollector
	fallback SymbolCollector
}

func (c fallbackSymbolCollector) Collect(ctx context.Context, root string, paths []string) (Catalog, error) {
	catalog, err := c.primary.Collect(ctx, root, paths)
	if err == nil {
		return catalog, nil
	}
	return c.fallback.Collect(ctx, root, paths)
}

type fileSymbolCollector struct{}

func (fileSymbolCollector) Collect(ctx context.Context, root string, paths []string) (Catalog, error) {
	catalog := Catalog{}
	for _, path := range referencePathsFromValues(paths) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		absolute, err := utils.CanonicalPathWithinRoot(root, filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			continue
		}
		symbols, err := ReadSymbols(absolute)
		if err == nil {
			catalog[path] = symbols
		}
	}
	return catalog, nil
}

type codeGraphSymbolCollector struct {
	client    codeGraphClient
	mu        sync.Mutex
	readiness map[string]error
	attempted map[string]bool
}

type codeGraphContext struct {
	Nodes []codeGraphNode `json:"nodes"`
}

func (c *codeGraphSymbolCollector) Collect(ctx context.Context, root string, paths []string) (Catalog, error) {
	paths = referencePathsFromValues(paths)
	if len(paths) == 0 {
		return Catalog{}, nil
	}
	if err := c.ensureReady(ctx, root); err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}
	catalog := Catalog{}
	for start := 0; start < len(paths); start += 25 {
		end := min(start+25, len(paths))
		batch := paths[start:end]
		query := "Return source symbols defined in exactly these files: " + strings.Join(batch, ", ")
		limit := max(100, len(batch)*80)
		output, err := c.client.Run(ctx, root, "context", "-p", root, "-f", "json", "-n", fmt.Sprint(limit), "-c", "0", query)
		if err != nil {
			return nil, fmt.Errorf("collect CodeGraph symbols: %w: %s", err, strings.TrimSpace(output))
		}
		var result codeGraphContext
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			return nil, fmt.Errorf("decode CodeGraph symbols: %w", err)
		}
		for _, node := range result.Nodes {
			path := normalizeReferencePath(node.FilePath)
			if !wanted[path] || node.Name == "" {
				continue
			}
			catalog[path] = append(catalog[path], Symbol{Name: node.Name, Kind: canonicalKind(node.Kind), Line: node.StartLine, Signature: codeGraphSignature(node)})
		}
	}
	return catalog, nil
}

func (c *codeGraphSymbolCollector) ensureReady(ctx context.Context, root string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attempted[root] {
		return c.readiness[root]
	}
	_, err := c.client.EnsureReady(ctx, root)
	c.attempted[root] = true
	c.readiness[root] = err
	return err
}

func referencePathsFromValues(paths []string) []string {
	seen := map[string]bool{}
	for _, path := range paths {
		if path = normalizeReferencePath(path); path != "" {
			seen[path] = true
		}
	}
	return sortedKeys(seen)
}
