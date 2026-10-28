package sourcecode

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

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

// BusinessMethodReferences 返回能力入口中的符号引用。
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

func normalizeReferencePath(path string) string {
	return pathx.CleanEvidenceLocationPath(path)
}

func resolveProjectFile(projectRoot, path string) (string, bool) {
	path = normalizeReferencePath(path)
	if path == "" {
		return "", false
	}
	resolved, err := projectpath.CanonicalWithinRoot(projectRoot, filepath.Join(projectRoot, filepath.FromSlash(path)))
	return resolved, err == nil
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

// NewResolver 使用本地语法树核验精确证据，结构索引统一由内嵌 CodeGraph 提供。
func NewResolver(_ config.StructuralConfig) Resolver { return treeSitterResolver{} }

// EvidenceDigest 验证路径归属后读取当前内容，供已审查证据失效判断使用。
func EvidenceDigest(root, path string) (string, error) {
	resolved, ok := resolveProjectFile(root, path)
	if !ok {
		return "", fmt.Errorf("evidence leaves project: %s", path)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
