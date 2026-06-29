package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// ParsePlanAnalysisUnitsResult 解析业务分析单元规划结果。
func ParsePlanAnalysisUnitsResult(output string) (*agent.PlanAnalysisUnitsResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var payload struct {
		Units []domain.AnalysisUnit `json:"units"`
	}
	if err := parseJSONPayload(jsonStr, &payload); err != nil {
		return nil, err
	}
	return &agent.PlanAnalysisUnitsResult{Units: payload.Units}, nil
}

// ParseWorkspaceProfile 解析工作区画像结果。
func ParseWorkspaceProfile(output string) (*domain.WorkspaceProfile, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentExtractJSONError"), err)
	}

	var profile domain.WorkspaceProfile
	if err := parseJSONPayload(jsonStr, &profile); err != nil {
		return nil, err
	}
	if profile.Projects == nil {
		profile.Projects = []domain.WorkspaceProject{}
	}
	if profile.Shared == nil {
		profile.Shared = []domain.WorkspacePath{}
	}
	if profile.Contracts == nil {
		profile.Contracts = []domain.WorkspacePath{}
	}
	if profile.Infra == nil {
		profile.Infra = []domain.WorkspacePath{}
	}
	if profile.Dependencies == nil {
		profile.Dependencies = []domain.WorkspaceDependency{}
	}
	if profile.ImpactRoutes == nil {
		profile.ImpactRoutes = []domain.WorkspaceRoute{}
	}
	return &profile, nil
}

// ParseWorkspaceSpec 解析工作区开发规范结果。
func ParseWorkspaceSpec(output string) (*domain.WorkspaceSpec, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentExtractJSONError"), err)
	}

	var payload workspaceSpecPayload
	if err := parseJSONPayload(jsonStr, &payload); err != nil {
		return nil, err
	}
	spec := payload.WorkspaceSpec
	spec.ChangeOrder = normalizeWorkspaceChangeOrder(payload.ChangeOrder)
	if spec.Projects == nil {
		spec.Projects = []domain.WorkspaceProject{}
	}
	if spec.Routing == nil {
		spec.Routing = []domain.WorkspaceRoute{}
	}
	if spec.Rules == nil {
		spec.Rules = []domain.WorkspaceRule{}
	}
	if spec.ChangeOrder == nil {
		spec.ChangeOrder = []string{}
	}
	if spec.ParallelAgentGuidance == nil {
		spec.ParallelAgentGuidance = []domain.WorkspaceParallelGuidance{}
	}
	if spec.LoadMultipleSkillsWhen == nil {
		spec.LoadMultipleSkillsWhen = []domain.WorkspaceLoadMultipleSkill{}
	}
	return &spec, nil
}

func normalizeWorkspaceChangeOrder(items []json.RawMessage) []string {
	if len(items) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := normalizeWorkspaceChangeOrderItem(item); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeWorkspaceChangeOrderItem(item json.RawMessage) string {
	var text string
	if err := json.Unmarshal(item, &text); err == nil {
		return strings.TrimSpace(text)
	}

	var object struct {
		Step    int    `json:"step"`
		Action  string `json:"action"`
		Details string `json:"details"`
	}
	if err := json.Unmarshal(item, &object); err != nil {
		return ""
	}
	action := strings.TrimSpace(object.Action)
	details := strings.TrimSpace(object.Details)
	if action == "" {
		return details
	}
	if object.Step > 0 {
		action = fmt.Sprintf("%d. %s", object.Step, action)
	}
	if details == "" {
		return action
	}
	return action + "：" + details
}
