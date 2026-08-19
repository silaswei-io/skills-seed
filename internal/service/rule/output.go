package rule

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
)

const ruleOutputDir = "rules"

// Reference 描述生成 Skill 中可引用的规则文档。
type Reference struct {
	ID               string
	Name             string
	Path             string
	Summary          string
	RouteTerms       []string
	AffectedProjects []string
	Paths            []string
}

// References 返回规则入口所需的简短路由信息。
func References(rules []domain.Rule) []Reference {
	refs := make([]Reference, 0, len(rules))
	for _, rule := range rules {
		if strings.TrimSpace(rule.ID) == "" {
			continue
		}
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			name = rule.ID
		}
		refs = append(refs, Reference{
			ID:               rule.ID,
			Name:             name,
			Path:             "./" + filepath.ToSlash(filepath.Join(ruleOutputDir, rule.ID+".md")),
			Summary:          strings.TrimSpace(rule.Summary),
			RouteTerms:       append([]string(nil), rule.RouteTerms...),
			AffectedProjects: append([]string(nil), rule.AffectedProjects...),
			Paths:            append([]string(nil), rule.Paths...),
		})
	}
	return refs
}

// Write 把规则原文确定性写入生成的 Skill。
func Write(rules []domain.Rule, outputPath string) error {
	dir := filepath.Join(outputPath, filepath.FromSlash(ruleOutputDir))
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, rule := range rules {
		if strings.TrimSpace(rule.ID) == "" {
			continue
		}
		content := strings.TrimSpace(rule.Content) + "\n"
		if err := fileio.WriteFileAtomic(filepath.Join(dir, rule.ID+".md"), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ApplicableToProject 返回明确适用于指定工作区子项目的根规则。
func ApplicableToProject(rules []domain.Rule, projectID, projectPath string) []domain.Rule {
	var out []domain.Rule
	for _, rule := range rules {
		if contains(rule.AffectedProjects, projectID) || pathTargetsProject(rule.Paths, projectPath) {
			out = append(out, rule)
		}
	}
	return out
}

func pathTargetsProject(paths []string, projectPath string) bool {
	projectPath = strings.Trim(filepath.ToSlash(strings.TrimSpace(projectPath)), "/")
	if projectPath == "" {
		return false
	}
	for _, path := range paths {
		path = strings.Trim(filepath.ToSlash(strings.TrimSpace(path)), "/")
		if path == projectPath || strings.HasPrefix(path, projectPath+"/") {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}
