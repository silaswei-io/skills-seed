package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type engineeringRuleValidationIssue struct {
	field string
	err   error
}

func (i engineeringRuleValidationIssue) Error() string {
	return fmt.Sprintf("%s: %v", i.field, i.err)
}

func validateEngineeringRules(root string, knowledge []string, hasUserContext bool, rules []domain.EngineeringRule) ([]domain.EngineeringRule, []error) {
	allowed := make(map[string]bool, len(knowledge))
	for _, path := range knowledge {
		if normalized, ok := projectRelativePath(path); ok {
			allowed[normalized] = true
		}
	}

	out := make([]domain.EngineeringRule, 0, len(rules))
	issues := make([]error, 0)
	seen := make(map[string]bool, len(rules))
	for i, rule := range rules {
		rule, err := validateEngineeringRule(root, allowed, hasUserContext, i, rule)
		if err != nil {
			issues = append(issues, err)
			continue
		}
		key := rule.Source + "\x00" + strings.ToLower(rule.Title) + "\x00" + rule.Rule
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, rule)
	}
	return out, issues
}

func validateEngineeringRule(root string, allowed map[string]bool, hasUserContext bool, index int, rule domain.EngineeringRule) (domain.EngineeringRule, error) {
	field := fmt.Sprintf("engineering_rules[%d]", index)
	rule.Title = strings.TrimSpace(rule.Title)
	rule.Rule = strings.TrimSpace(rule.Rule)
	rule.Source = strings.TrimSpace(rule.Source)
	if rule.Title == "" || rule.Rule == "" {
		return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: field, err: fmt.Errorf("title and rule are required")}
	}
	if rule.Source == "user_context" {
		if !hasUserContext {
			return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: field + ".source", err: fmt.Errorf("user_context was not provided")}
		}
	} else {
		source, ok := projectRelativePath(rule.Source)
		if !ok || !allowed[source] {
			return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: field + ".source", err: fmt.Errorf("%q is not an authoritative engineering knowledge file", rule.Source)}
		}
		if err := validateProjectFile(root, source); err != nil {
			return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: field + ".source", err: err}
		}
		rule.Source = source
		if len(rule.Evidence) == 0 {
			return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: field + ".evidence", err: fmt.Errorf("repository rule requires evidence paths")}
		}
	}
	evidence := make([]string, 0, len(rule.Evidence))
	for j, path := range rule.Evidence {
		normalized, ok := projectRelativePath(path)
		if !ok {
			return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: fmt.Sprintf("%s.evidence[%d]", field, j), err: fmt.Errorf("invalid project-relative path %q", path)}
		}
		if err := validateProjectFile(root, normalized); err != nil {
			return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: fmt.Sprintf("%s.evidence[%d]", field, j), err: err}
		}
		evidence = append(evidence, normalized)
	}
	rule.Evidence = evidence
	return rule, nil
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
