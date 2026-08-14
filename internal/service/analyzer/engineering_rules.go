package analyzer

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
)

var markdownHeadingPattern = regexp.MustCompile(`^#{1,6}[\t ]+(.+?)(?:[\t ]+#+[\t ]*)?$`)

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
	rule.Section = strings.TrimSpace(rule.Section)
	rule.AppliesTo = cleanRuleScopes(rule.AppliesTo)
	if !validCommandPolicy(rule.CommandPolicy) {
		return domain.EngineeringRule{}, engineeringRuleValidationIssue{field: field + ".command_policy", err: fmt.Errorf("invalid command policy %q", rule.CommandPolicy)}
	}
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

func cleanRuleScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	out := make([]string, 0, len(scopes))
	seen := make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	return out
}

func validCommandPolicy(policy string) bool {
	policy = strings.TrimSpace(policy)
	return policy == "" || policy == domain.CommandPolicyForbidden || policy == domain.CommandPolicyDescribeOnly || policy == domain.CommandPolicyRequiresAuthorization || policy == domain.CommandPolicyAllowed
}

func buildAuthoritySections(coverage []domain.AuthorityCoverage, hasUserContext bool) []agent.AuthoritySection {
	sections := make([]agent.AuthoritySection, 0, len(coverage))
	for _, item := range coverage {
		if len(item.Sections) == 0 {
			sections = append(sections, newAuthoritySection(item.Source, ""))
			continue
		}
		for _, section := range item.Sections {
			sections = append(sections, newAuthoritySection(item.Source, section))
		}
	}
	if hasUserContext {
		sections = append(sections, newAuthoritySection("user_context", ""))
	}
	return sections
}

func newAuthoritySection(source, section string) agent.AuthoritySection {
	source = strings.TrimSpace(source)
	section = strings.TrimSpace(section)
	sum := sha256.Sum256([]byte(source + "\x00" + section))
	return agent.AuthoritySection{ID: fmt.Sprintf("authority-%x", sum[:8]), Source: source, Section: section}
}

func expandAuthoritySections(catalog []agent.AuthoritySection, results []agent.AuthoritySectionResult) ([]domain.EngineeringRule, error) {
	byID := make(map[string]agent.AuthoritySection, len(catalog))
	for _, section := range catalog {
		byID[section.ID] = section
	}
	seen := make(map[string]bool, len(results))
	rules := make([]domain.EngineeringRule, 0)
	for _, result := range results {
		section, ok := byID[strings.TrimSpace(result.SectionID)]
		if !ok {
			return nil, fmt.Errorf("authority section result references unknown section_id %q", result.SectionID)
		}
		if seen[section.ID] {
			return nil, fmt.Errorf("authority section result %q is duplicated", section.ID)
		}
		seen[section.ID] = true
		if len(result.Rules) == 0 && !meaningfulNoRuleReason(result.NoRuleReason) {
			return nil, fmt.Errorf("authority section result %q must contain rules or a no-rule reason", section.ID)
		}
		for _, rule := range result.Rules {
			rule.Source = section.Source
			rule.Section = section.Section
			if section.Source != "user_context" {
				rule.Evidence = []string{section.Source}
			}
			rules = append(rules, rule)
		}
	}
	for _, section := range catalog {
		if !seen[section.ID] {
			return nil, fmt.Errorf("authority section %q was not returned", section.ID)
		}
	}
	return rules, nil
}

func meaningfulNoRuleReason(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "", "n/a", "na", "none", "null", "nil", "无", "无规则":
		return false
	default:
		return true
	}
}

// collectAuthorityCoverage 从本次实际输入确定性构造权威边界，不依赖 Agent 回显路径。
func collectAuthorityCoverage(root string, knowledge []string) ([]domain.AuthorityCoverage, error) {
	paths, err := cleanEngineeringKnowledgePaths(knowledge)
	if err != nil {
		return nil, err
	}
	coverage := make([]domain.AuthorityCoverage, 0, len(paths))
	for _, path := range paths {
		resolved, err := projectpath.CanonicalWithinRoot(root, filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, fmt.Errorf("authority path %q is outside the project root: %w", path, err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("stat authority path %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("authority path %q is not a regular file", path)
		}
		sections, err := authorityMarkdownSections(resolved)
		if err != nil {
			return nil, fmt.Errorf("read authority path %q: %w", path, err)
		}
		coverage = append(coverage, domain.AuthorityCoverage{Source: path, Sections: sections})
	}
	return coverage, nil
}

func authorityMarkdownSections(path string) ([]string, error) {
	if strings.ToLower(filepath.Ext(path)) != ".md" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sections []string
	inFence := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(raw, "    ") || strings.HasPrefix(raw, "\t") {
			continue
		}
		match := markdownHeadingPattern.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		heading := strings.TrimSpace(match[1])
		if heading != "" {
			sections = append(sections, heading)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cleanRuleScopes(sections), nil
}

func validateProjectFile(root, path string) error {
	resolved, err := projectpath.CanonicalWithinRoot(root, filepath.Join(root, filepath.FromSlash(path)))
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
