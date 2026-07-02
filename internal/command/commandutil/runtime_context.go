package commandutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// ResolveRuntimeContext 合并 learn/generate 的一次性用户上下文参数。
// 行内上下文优先于 context-path，因为它是最明确的输入。
func ResolveRuntimeContext(inline string, paths ...string) (string, error) {
	inline = strings.TrimSpace(inline)
	if inline != "" {
		return inline, nil
	}
	contents, err := readRuntimeContextPaths(paths)
	if err != nil {
		return "", err
	}
	if len(contents) == 0 {
		return "", nil
	}
	if len(contents) == 1 {
		return strings.TrimSpace(contents[0].Content), nil
	}
	var builder strings.Builder
	for _, content := range contents {
		text := strings.TrimSpace(content.Content)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("# ")
		builder.WriteString(content.Path)
		builder.WriteString("\n\n")
		builder.WriteString(text)
	}
	return strings.TrimSpace(builder.String()), nil
}

type runtimeContextContent struct {
	Path    string
	Content string
}

func readRuntimeContextPaths(paths []string) ([]runtimeContextContent, error) {
	var files []string
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		expanded, err := expandRuntimeContextPath(path)
		if err != nil {
			return nil, err
		}
		files = append(files, expanded...)
	}
	sort.Strings(files)
	contents := make([]runtimeContextContent, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if isLikelyBinary(data) {
			continue
		}
		contents = append(contents, runtimeContextContent{
			Path:    filepath.ToSlash(file),
			Content: string(data),
		})
	}
	return contents, nil
}

func expandRuntimeContextPath(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("context path is not a regular file: %s", path)
		}
		return []string{path}, nil
	}
	var files []string
	err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			files = append(files, current)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func isLikelyBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return bytes.Contains(data, []byte{0}) || !utf8.Valid(data)
}
