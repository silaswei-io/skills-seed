package skilloutput

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// ReadinessRequirements 描述生成结果必须满足的确定性交付条件。
type ReadinessRequirements struct {
	ExpectedFiles   []string
	RequiredContent map[string][]string
}

// AuditReadiness 在 staging 发布前校验 Skill 入口、计划文件、本地链接和关键知识投影。
func AuditReadiness(root string, requirements ReadinessRequirements) error {
	if err := auditSkillFrontmatter(filepath.Join(root, "SKILL.md")); err != nil {
		return err
	}
	for _, path := range cleanAuditPaths(requirements.ExpectedFiles) {
		if err := requireRegularFile(root, path); err != nil {
			return err
		}
	}
	if err := auditRequiredContent(root, requirements.RequiredContent); err != nil {
		return err
	}
	return auditMarkdownLinks(root)
}

func auditSkillFrontmatter(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("skill readiness: read SKILL.md: %w", err)
	}
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return fmt.Errorf("skill readiness: SKILL.md must start with YAML frontmatter")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return fmt.Errorf("skill readiness: SKILL.md frontmatter is not closed")
	}
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(text[4:4+end]), &metadata); err != nil {
		return fmt.Errorf("skill readiness: invalid SKILL.md frontmatter: %w", err)
	}
	for _, field := range []string{"name", "description"} {
		value, ok := metadata[field].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("skill readiness: SKILL.md frontmatter requires non-empty %s", field)
		}
	}
	return nil
}

func auditRequiredContent(root string, required map[string][]string) error {
	paths := make([]string, 0, len(required))
	for path := range required {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("skill readiness: read required projection %q: %w", path, err)
		}
		for _, value := range required[path] {
			value = strings.TrimSpace(value)
			if value != "" && !strings.Contains(string(content), value) {
				return fmt.Errorf("skill readiness: %q is missing required projection %q", path, value)
			}
		}
	}
	return nil
}

func auditMarkdownLinks(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, rawTarget := range markdownLinkTargets(string(content)) {
			target := localMarkdownTarget(rawTarget)
			if target == "" {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)))
			rel, err := filepath.Rel(root, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("skill readiness: link %q in %q escapes skill root", rawTarget, filepath.ToSlash(path))
			}
			info, err := os.Stat(resolved)
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("skill readiness: broken link %q in %q", rawTarget, filepath.ToSlash(path))
			}
		}
		return nil
	})
}

// markdownLinkTargets 从 Markdown AST 提取链接目标。
// 标准解析器负责区分链接与代码、普通括号等语法，避免把源码调用参数误当成本地路径。
func markdownLinkTargets(content string) []string {
	source := []byte(content)
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	targets := make([]string, 0)
	collectMarkdownLinkTargets(document, source, &targets)
	return targets
}

func collectMarkdownLinkTargets(node ast.Node, source []byte, targets *[]string) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if target, ok := markdownNodeLinkTarget(child); ok && !isEmbeddedCodeIndexLink(child, source) {
			*targets = append(*targets, string(target))
		}
		collectMarkdownLinkTargets(child, source, targets)
	}
}

func markdownNodeLinkTarget(node ast.Node) ([]byte, bool) {
	switch link := node.(type) {
	case *ast.Link:
		return link.Destination, true
	case *ast.Image:
		return link.Destination, true
	default:
		return nil, false
	}
}

func isEmbeddedCodeIndexLink(node ast.Node, source []byte) bool {
	label, ok := node.FirstChild().(*ast.Text)
	if !ok || label.Segment.Start == 0 {
		return false
	}
	open := label.Segment.Start - 1
	if open == 0 || source[open] != '[' || !isMarkdownLinkEmbeddedChar(source[open-1]) {
		return false
	}
	return looksLikeCodeIndexLabel(string(label.Segment.Value(source)))
}

func isMarkdownLinkEmbeddedChar(char byte) bool {
	return char == '_' || char == ')' || (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

func looksLikeCodeIndexLabel(label string) bool {
	label = strings.TrimSpace(label)
	if len(label) >= 2 && ((label[0] == '"' && label[len(label)-1] == '"') || (label[0] == '\'' && label[len(label)-1] == '\'')) {
		return true
	}
	if label == "" {
		return false
	}
	for i := 0; i < len(label); i++ {
		if label[i] < '0' || label[i] > '9' {
			return false
		}
	}
	return true
}

func localMarkdownTarget(raw string) string {
	target := strings.TrimSpace(raw)
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") || strings.Contains(target, "://") {
		return ""
	}
	if index := strings.Index(target, "#"); index >= 0 {
		target = target[:index]
	}
	return strings.TrimSpace(target)
}

func requireRegularFile(root, path string) error {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return fmt.Errorf("skill readiness: expected file %q is missing: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("skill readiness: expected path %q is not a regular file", path)
	}
	return nil
}

func cleanAuditPaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if path == "" || path == "." || strings.HasPrefix(path, "../") || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
