package generator

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	ruleoutput "github.com/silaswei-io/skills-seed/internal/service/rule"
)

func (s *GeneratorService) loadRules(projected []domain.Rule) ([]domain.Rule, error) {
	var rules []domain.Rule
	if s.ruleRepo != nil {
		stored, err := s.ruleRepo.List()
		if err != nil {
			return nil, err
		}
		rules = append(rules, stored...)
	}
	rules = append(rules, projected...)
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		if seen[rule.ID] {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("GenerateRuleIDConflict", map[string]interface{}{"ID": rule.ID}))
		}
		seen[rule.ID] = true
	}
	return rules, nil
}

func ruleReferences(rules []domain.Rule) []RuleReference {
	refs := ruleoutput.References(rules)
	out := make([]RuleReference, 0, len(refs))
	for _, ref := range refs {
		out = append(out, RuleReference(ref))
	}
	return out
}

// RuleReferences 把规则转换为工作区模板可用的路由引用。
func RuleReferences(rules []domain.Rule) []RuleReference { return ruleReferences(rules) }

func writeRuleOutputs(rules []domain.Rule, outputPath string) error {
	return ruleoutput.Write(rules, outputPath)
}
