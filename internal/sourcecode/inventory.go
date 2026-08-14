package sourcecode

import (
	"context"
	"os"
	"sort"
	"strings"
)

// FileFact 是规划阶段可安全消费的轻量源码事实，不包含源码正文或语义判断。
type FileFact struct {
	Path          string
	SizeBytes     int64
	LineCount     int
	NonBlankLines int
	Symbols       []Symbol
}

// InspectFiles 为给定项目相对路径生成确定性的源码事实清单。
func InspectFiles(ctx context.Context, projectRoot string, paths []string) []FileFact {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = normalizeReferencePath(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		normalized = append(normalized, path)
	}
	sort.Strings(normalized)

	facts := make([]FileFact, 0, len(normalized))
	for _, path := range normalized {
		if ctx.Err() != nil {
			break
		}
		resolved, ok := resolveProjectFile(projectRoot, path)
		if !ok {
			continue
		}
		src, err := os.ReadFile(resolved)
		if err != nil {
			continue
		}
		lineCount, nonBlankLines := sourceLineCounts(src)
		fact := FileFact{
			Path:          path,
			SizeBytes:     int64(len(src)),
			LineCount:     lineCount,
			NonBlankLines: nonBlankLines,
		}
		if symbols, err := parseSymbols(path, src); err == nil {
			fact.Symbols = symbols
		}
		facts = append(facts, fact)
	}
	return facts
}

func sourceLineCounts(src []byte) (int, int) {
	if len(src) == 0 {
		return 0, 0
	}
	lines := strings.Split(string(src), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	nonBlank := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonBlank++
		}
	}
	return len(lines), nonBlank
}
