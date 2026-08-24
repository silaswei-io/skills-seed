// Package maintained 提供用户维护学习资源的读取能力。
package maintained

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// Snapshot 是一次学习运行可见的完整用户维护资源。
// Rule 约束未来改动；Workflow 负责用户定义的任务过程。
type Snapshot struct {
	Rules     []domain.Rule     `json:"rules,omitempty"`
	Workflows []domain.Workflow `json:"workflows,omitempty"`
}

// Provider 读取持久化的用户维护资源。
type Provider struct {
	rules     domain.RuleRepository
	workflows domain.WorkflowRepository
}

// New 创建用户维护资源提供者。
func New(rules domain.RuleRepository, workflows domain.WorkflowRepository) Provider {
	return Provider{rules: rules, workflows: workflows}
}

// Load 返回完整、稳定排序的资源快照。
func (p Provider) Load() (Snapshot, error) {
	var snapshot Snapshot
	if p.rules != nil {
		rules, err := p.rules.List()
		if err != nil {
			return Snapshot{}, fmt.Errorf("list user rules: %w", err)
		}
		snapshot.Rules = rules
	}
	if p.workflows != nil {
		workflows, err := p.workflows.List()
		if err != nil {
			return Snapshot{}, fmt.Errorf("list user workflows: %w", err)
		}
		snapshot.Workflows = workflows
	}
	return snapshot, nil
}
