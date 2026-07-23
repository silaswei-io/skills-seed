package sourcecode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

const codeGraphQueryLimit = 100

// Reference 描述需要由结构化索引确认的源码符号。
type Reference struct {
	Path string
	Line int
	Name string
	Kind string
}

// Catalog 是按项目相对路径组织的已解析符号目录。
type Catalog map[string][]Symbol

// Resolver 将一组源码引用解析为符号目录。
type Resolver interface {
	Resolve(ctx context.Context, projectRoot string, refs []Reference) (Catalog, error)
}

// NewResolver 根据结构化 provider 创建唯一的符号解析入口。
func NewResolver(cfg config.StructuralConfig) Resolver {
	if config.NormalizeStructuralProvider(string(cfg.Provider)) == config.StructuralProviderTreeSitter {
		return treeSitterResolver{}
	}
	return newCodeGraphResolver(codegraph.NewClient("codegraph"))
}

type treeSitterResolver struct{}

func (treeSitterResolver) Resolve(ctx context.Context, projectRoot string, refs []Reference) (Catalog, error) {
	paths := referencePaths(refs)
	catalog := make(Catalog, len(paths))
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resolved, ok := resolveProjectFile(projectRoot, path)
		if !ok {
			continue
		}
		src, err := os.ReadFile(resolved)
		if err != nil {
			continue
		}
		symbols, err := parseSymbols(path, src)
		if err != nil {
			continue
		}
		catalog[path] = symbols
	}
	return catalog, nil
}

type codeGraphClient interface {
	EnsureReady(ctx context.Context, projectRoot string) (*codegraph.Status, error)
	Run(ctx context.Context, projectRoot string, args ...string) (string, error)
}

type codeGraphResolver struct {
	client codeGraphClient
}

func newCodeGraphResolver(client codeGraphClient) *codeGraphResolver {
	return &codeGraphResolver{client: client}
}

func (r *codeGraphResolver) Resolve(ctx context.Context, projectRoot string, refs []Reference) (Catalog, error) {
	queries := referenceNames(refs)
	if len(queries) == 0 {
		return Catalog{}, nil
	}
	if _, err := r.client.EnsureReady(ctx, projectRoot); err != nil {
		return nil, err
	}

	wanted := wantedReferences(refs)
	catalog := Catalog{}
	var mu sync.Mutex
	var firstErr error
	jobs := make(chan string)
	var workers sync.WaitGroup
	workerCount := min(4, len(queries))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for name := range jobs {
				nodes, err := r.query(ctx, projectRoot, name)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					continue
				}
				mu.Lock()
				for _, node := range nodes {
					path := normalizeReferencePath(node.FilePath)
					if node.Name != name || !wanted[path][name] {
						continue
					}
					catalog[path] = append(catalog[path], Symbol{
						Name:      node.Name,
						Kind:      canonicalKind(node.Kind),
						Line:      node.StartLine,
						Signature: codeGraphSignature(node),
					})
				}
				mu.Unlock()
			}
		}()
	}
	for _, query := range queries {
		jobs <- query
	}
	close(jobs)
	workers.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return catalog, nil
}

type codeGraphNode struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualifiedName"`
	Kind          string `json:"kind"`
	FilePath      string `json:"filePath"`
	StartLine     int    `json:"startLine"`
	Signature     string `json:"signature"`
}

type codeGraphResult struct {
	Node codeGraphNode `json:"node"`
}

func (r *codeGraphResolver) query(ctx context.Context, projectRoot, name string) ([]codeGraphNode, error) {
	output, err := r.client.Run(ctx, projectRoot, "query", "-p", projectRoot, "-j", "-l", fmt.Sprintf("%d", codeGraphQueryLimit), name)
	if err != nil {
		return nil, fmt.Errorf("query CodeGraph symbol %q: %w: %s", name, err, strings.TrimSpace(output))
	}
	var results []codeGraphResult
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		return nil, fmt.Errorf("decode CodeGraph symbol %q: %w", name, err)
	}
	nodes := make([]codeGraphNode, 0, len(results))
	for _, result := range results {
		nodes = append(nodes, result.Node)
	}
	return nodes, nil
}

// EvidenceReferences 返回模式证据中的符号引用。
func EvidenceReferences(values []domain.PatternEvidenceLocation) []Reference {
	refs := make([]Reference, 0, len(values))
	for _, value := range values {
		refs = append(refs, Reference{Path: value.Path, Line: value.Line, Name: value.Symbol, Kind: value.Kind})
	}
	return refs
}

// UtilityReferences 返回工具函数中的符号引用。
func UtilityReferences(values []domain.UtilityFunction) []Reference {
	refs := make([]Reference, 0, len(values))
	for _, value := range values {
		path, line := splitLocation(value.File)
		refs = append(refs, Reference{Path: path, Line: line, Name: value.Name, Kind: "function"})
	}
	return refs
}

// BusinessMethodReferences 返回业务方法中的符号引用。
func BusinessMethodReferences(values []domain.BusinessMethod) []Reference {
	refs := make([]Reference, 0, len(values))
	for _, value := range values {
		path, line := splitLocation(value.DisplayLocation())
		refs = append(refs, Reference{Path: path, Line: line, Name: value.Name, Kind: "function"})
	}
	return refs
}

func referencePaths(refs []Reference) []string {
	seen := map[string]bool{}
	for _, ref := range refs {
		path := normalizeReferencePath(ref.Path)
		if path != "" && simpleSymbolName(ref.Name) != "" {
			seen[path] = true
		}
	}
	return sortedKeys(seen)
}

func referenceNames(refs []Reference) []string {
	seen := map[string]bool{}
	for _, ref := range refs {
		if name := simpleSymbolName(ref.Name); name != "" && normalizeReferencePath(ref.Path) != "" {
			seen[name] = true
		}
	}
	return sortedKeys(seen)
}

func wantedReferences(refs []Reference) map[string]map[string]bool {
	wanted := map[string]map[string]bool{}
	for _, ref := range refs {
		path := normalizeReferencePath(ref.Path)
		name := simpleSymbolName(ref.Name)
		if path == "" || name == "" {
			continue
		}
		if wanted[path] == nil {
			wanted[path] = map[string]bool{}
		}
		wanted[path][name] = true
	}
	return wanted
}

func normalizeReferencePath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" || filepath.IsAbs(path) {
		return ""
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(clean)
}

func resolveProjectFile(projectRoot, path string) (string, bool) {
	path = normalizeReferencePath(path)
	if path == "" {
		return "", false
	}
	resolved, err := utils.CanonicalPathWithinRoot(projectRoot, filepath.Join(projectRoot, filepath.FromSlash(path)))
	return resolved, err == nil
}

func codeGraphSignature(node codeGraphNode) string {
	signature := strings.TrimSpace(node.Signature)
	if signature == "" {
		return ""
	}
	name := strings.ReplaceAll(strings.TrimSpace(node.QualifiedName), "::", ".")
	if name == "" {
		name = node.Name
	}
	if strings.HasPrefix(signature, node.Name) || strings.HasPrefix(signature, name) {
		return signature
	}
	return name + signature
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}
