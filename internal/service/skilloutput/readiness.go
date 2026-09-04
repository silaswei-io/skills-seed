package skilloutput

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// markdownLinkTargets 提取 Markdown 内联链接目标。
// 不能用简单正则匹配：源码示例中的 map/index 表达式（例如 params["deadline"](RFC3339Nano)）
// 也包含方括号和圆括号，必须跳过行内代码、围栏代码及嵌入标识符的方括号。
func markdownLinkTargets(content string) []string {
	var targets []string
	inFence := false
	var fenceChar byte
	for _, rawLine := range strings.SplitAfter(content, "\n") {
		line := strings.TrimLeft(rawLine, " \t")
		if inFence {
			if isMarkdownFenceClose(line, fenceChar) {
				inFence = false
			}
			continue
		}
		if char, ok := markdownFenceStart(line); ok {
			inFence = true
			fenceChar = char
			continue
		}
		targets = append(targets, markdownInlineLinkTargets(rawLine)...)
	}
	return targets
}

func markdownFenceStart(line string) (byte, bool) {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') || line[1] != line[0] || line[2] != line[0] {
		return 0, false
	}
	return line[0], true
}

func isMarkdownFenceClose(line string, fenceChar byte) bool {
	if len(line) < 3 || line[0] != fenceChar || line[1] != fenceChar || line[2] != fenceChar {
		return false
	}
	return strings.TrimSpace(line[3:]) == ""
}

func markdownInlineLinkTargets(line string) []string {
	var targets []string
	inlineCodeDelimiter := 0
	for i := 0; i < len(line); {
		if line[i] == '`' {
			j := i
			for j < len(line) && line[j] == '`' {
				j++
			}
			delimiter := j - i
			if inlineCodeDelimiter == 0 {
				inlineCodeDelimiter = delimiter
			} else if delimiter == inlineCodeDelimiter {
				inlineCodeDelimiter = 0
			}
			i = j
			continue
		}
		if inlineCodeDelimiter == 0 && line[i] == '[' && (i == 0 || !isMarkdownLinkEmbeddedChar(line[i-1])) {
			if endLabel := strings.IndexByte(line[i+1:], ']'); endLabel >= 0 {
				close := i + 1 + endLabel
				if close+1 < len(line) && line[close+1] == '(' {
					if endTarget := strings.IndexByte(line[close+2:], ')'); endTarget >= 0 {
						targets = append(targets, line[close+2:close+2+endTarget])
						i = close + 3 + endTarget
						continue
					}
				}
			}
		}
		i++
	}
	return targets
}

func isMarkdownLinkEmbeddedChar(char byte) bool {
	return char == '_' || char == '-' || char == '"' || char == '\'' || char == ']' || char == ')' || char == '`' || (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
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
