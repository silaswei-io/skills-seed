package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

func validateEngineeringRules(root string, knowledge []string, hasUserContext bool, rules []domain.EngineeringRule) ([]domain.EngineeringRule, error) {
	allowed := make(map[string]bool, len(knowledge))
	for _, path := range knowledge {
		if normalized, ok := projectRelativePath(path); ok {
			allowed[normalized] = true
		}
	}

	out := make([]domain.EngineeringRule, 0, len(rules))
	seen := make(map[string]bool, len(rules))
	for i, rule := range rules {
		field := fmt.Sprintf("engineering_rules[%d]", i)
		rule.Title = strings.TrimSpace(rule.Title)
		rule.Rule = strings.TrimSpace(rule.Rule)
		rule.Source = strings.TrimSpace(rule.Source)
		if rule.Title == "" || rule.Rule == "" {
			return nil, fmt.Errorf("%s: title and rule are required", field)
		}
		if rule.Source == "user_context" {
			if !hasUserContext {
				return nil, fmt.Errorf("%s.source: user_context was not provided", field)
			}
		} else {
			source, ok := projectRelativePath(rule.Source)
			if !ok || !allowed[source] {
				return nil, fmt.Errorf("%s.source: %q is not an authoritative engineering knowledge file", field, rule.Source)
			}
			if err := validateProjectFile(root, source); err != nil {
				return nil, fmt.Errorf("%s.source: %w", field, err)
			}
			rule.Source = source
			if len(rule.Evidence) == 0 {
				return nil, fmt.Errorf("%s.evidence: repository rule requires evidence paths", field)
			}
		}
		evidence := make([]string, 0, len(rule.Evidence))
		for j, path := range rule.Evidence {
			normalized, ok := projectRelativePath(path)
			if !ok {
				return nil, fmt.Errorf("%s.evidence[%d]: invalid project-relative path %q", field, j, path)
			}
			if err := validateProjectFile(root, normalized); err != nil {
				return nil, fmt.Errorf("%s.evidence[%d]: %w", field, j, err)
			}
			evidence = append(evidence, normalized)
		}
		rule.Evidence = evidence
		key := rule.Source + "\x00" + strings.ToLower(rule.Title) + "\x00" + rule.Rule
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, rule)
	}
	return out, nil
}

func validateProjectFile(root, path string) error {
	resolved, err := utils.CanonicalPathWithinRoot(root, filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return fmt.Errorf("path %q is outside the project root: %w", path, err)
	}
	if _, err := os.Stat(resolved); err != nil {
		return fmt.Errorf("path %q does not exist: %w", path, err)
	}
	return nil
}

func projectRelativePath(path string) (string, bool) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" || filepath.IsAbs(filepath.FromSlash(path)) {
		return "", false
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." || path == ".." || strings.HasPrefix(path, "../") {
		return "", false
	}
	return path, true
}
