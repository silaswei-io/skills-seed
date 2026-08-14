package patternview

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

// RelationKind 表示两个模式之间可由源码位置验证的关联类型。
type RelationKind string

const (
	// RelationEvidencePath 表示模式证据位于相交的源码路径。
	RelationEvidencePath RelationKind = "evidence_path"
	// RelationScope 表示模式位于同一显式工作区作用域。
	RelationScope RelationKind = "scope"
)

// Relation 描述两个模式之间可解释的知识关联。
type Relation struct {
	Kinds []RelationKind
}

// Exists 报告关联是否成立。
func (r Relation) Exists() bool {
	return len(r.Kinds) > 0
}

// Relate 根据已验证的路径和显式作用域建立模式关联。
func Relate(left, right domain.Pattern) Relation {
	kinds := make([]RelationKind, 0, 2)
	if pathsOverlap(patternEvidencePaths(left), patternEvidencePaths(right)) {
		kinds = append(kinds, RelationEvidencePath)
	}
	if SameScope(left, right) {
		kinds = append(kinds, RelationScope)
	}
	return Relation{Kinds: kinds}
}

// RelatedToPaths 报告模式是否与给定的项目相对路径存在可解释关联。
func RelatedToPaths(pattern domain.Pattern, paths []string) bool {
	return pathsOverlap(patternPaths(pattern), paths)
}

func patternPaths(pattern domain.Pattern) []string {
	paths := patternEvidencePaths(pattern)
	paths = append(paths, pattern.ScopePath)
	return normalizedRelationPaths(paths)
}

func patternEvidencePaths(pattern domain.Pattern) []string {
	paths := make([]string, 0, len(pattern.EvidenceLocations)+1)
	for _, location := range pattern.EvidenceLocations {
		paths = append(paths, location.Path)
	}
	if pattern.BusinessMethod != nil {
		paths = append(paths, pattern.BusinessMethod.DisplayLocation())
	}
	return normalizedRelationPaths(paths)
}

func pathsOverlap(left, right []string) bool {
	for _, leftPath := range normalizedRelationPaths(left) {
		for _, rightPath := range normalizedRelationPaths(right) {
			if leftPath == rightPath ||
				strings.HasPrefix(leftPath, rightPath+"/") ||
				strings.HasPrefix(rightPath, leftPath+"/") {
				return true
			}
		}
	}
	return false
}

func normalizedRelationPaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = pathx.CleanEvidenceLocationPath(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
