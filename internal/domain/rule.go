package domain

import "time"

// Rule 描述用户明确维护、未来改动必须遵守的规则。
type Rule struct {
	ID               string
	Name             string
	Content          string
	AffectedProjects []string
	Paths            []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RuleRepository 保存和读取规则资源。
type RuleRepository interface {
	List() ([]Rule, error)
	Get(id string) (*Rule, error)
	Save(rule Rule) error
}
