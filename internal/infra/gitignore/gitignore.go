package gitignore

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Matcher 判断相对路径是否被 Git ignore 规则忽略。
type Matcher struct {
	ignored map[string]bool
}

// NewMatcher 从 Git 仓库读取当前被 ignore 的路径集合。
func NewMatcher(ctx context.Context, root string) (*Matcher, error) {
	if strings.TrimSpace(root) == "" {
		return &Matcher{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "git", "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return newMatcherFromGitOutput(output), nil
}

func newMatcherFromGitOutput(output []byte) *Matcher {
	ignored := make(map[string]bool)
	for _, raw := range bytes.Split(output, []byte{0}) {
		path := normalizePath(string(raw))
		if path == "" {
			continue
		}
		ignored[path] = true
	}
	return &Matcher{ignored: ignored}
}

// Match 返回 path 是否命中 Git ignore。
func (m *Matcher) Match(path string) bool {
	if m == nil || len(m.ignored) == 0 {
		return false
	}
	path = normalizePath(path)
	if path == "" {
		return false
	}
	if m.ignored[path] {
		return true
	}
	segments := strings.Split(path, "/")
	for i := 1; i <= len(segments); i++ {
		if m.ignored[strings.Join(segments[:i], "/")+"/"] {
			return true
		}
	}
	return false
}

// Paths 返回被 ignore 的路径列表，主要用于测试和诊断。
func (m *Matcher) Paths() []string {
	if m == nil {
		return nil
	}
	paths := make([]string, 0, len(m.ignored))
	for path := range m.ignored {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func normalizePath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return ""
	}
	if strings.HasSuffix(path, "/") {
		return strings.Trim(strings.TrimSuffix(path, "/"), "/") + "/"
	}
	return strings.Trim(path, "/")
}
