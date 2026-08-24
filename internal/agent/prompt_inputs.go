package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/knowledge/maintained"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func promptLearningMode(mode config.LearningMode) config.LearningMode {
	return config.NormalizeLearningMode(string(mode))
}

type normalizePatternInput struct {
	ID              string                         `json:"id"`
	Name            string                         `json:"name"`
	Category        string                         `json:"category"`
	Description     string                         `json:"description,omitempty"`
	Rule            string                         `json:"rule,omitempty"`
	EvidencePaths   []string                       `json:"evidence_paths,omitempty"`
	CapabilityEntry *normalizeCapabilityEntryInput `json:"capability_entry,omitempty"`
	KnowledgeFlags  []string                       `json:"knowledge_flags,omitempty"`
}

type normalizeCapabilityEntryInput struct {
	Name               string `json:"name"`
	CurrentLocation    string `json:"current_location,omitempty"`
	HistoricalLocation string `json:"historical_location,omitempty"`
}

type maintainedGuidanceInput struct {
	Rules     []maintainedRuleInput     `json:"rules,omitempty"`
	Workflows []maintainedWorkflowInput `json:"workflows,omitempty"`
}

type maintainedRuleInput struct {
	ID               string   `json:"id"`
	Name             string   `json:"name,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	RouteTerms       []string `json:"route_terms,omitempty"`
	AffectedProjects []string `json:"affected_projects,omitempty"`
	Paths            []string `json:"paths,omitempty"`
	Content          string   `json:"content"`
}

type maintainedWorkflowInput struct {
	ID         string   `json:"id"`
	Name       string   `json:"name,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	RouteTerms []string `json:"route_terms,omitempty"`
	Content    string   `json:"content"`
}

// NormalizePatternsPromptData 返回当前模式合并优化所需的提示词数据。
func NormalizePatternsPromptData(session *PromptInputSession, req *NormalizePatternsRequest) (map[string]interface{}, error) {
	candidatesPath, err := writeNormalizePatternsInput(session, "candidate-patterns.json", req.Candidates)
	if err != nil {
		return nil, promptInputWriteError("candidate-patterns.json", err)
	}
	relatedPath, err := writeNormalizePatternsInput(session, "related-patterns.json", req.RelatedPatterns)
	if err != nil {
		return nil, promptInputWriteError("related-patterns.json", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	guidancePath, err := writeMaintainedGuidanceInput(session, req.MaintainedGuidance)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ProjectName":            req.ProjectName,
		"RootPath":               req.RootPath,
		"Language":               req.Language,
		"CandidatePatternsPath":  candidatesPath,
		"CandidatePatternCount":  len(req.Candidates),
		"RelatedPatternsPath":    relatedPath,
		"RelatedPatternCount":    len(req.RelatedPatterns),
		"UserContextPath":        userContextPath,
		"MaintainedGuidancePath": guidancePath,
		"AllowedCategories":      domain.AllowedPatternCategoriesText(),
	}, nil
}

// ReviewKnowledgePromptData 返回独立知识审查所需的提示词数据。
func ReviewKnowledgePromptData(session *PromptInputSession, req *ReviewKnowledgeRequest) (map[string]interface{}, error) {
	focusPath, err := writeJSONInput(session, "evidence-focus.json", req.EvidenceFocus)
	if err != nil {
		return nil, promptInputWriteError("evidence-focus.json", err)
	}
	candidatesPath, err := writeJSONInput(session, "knowledge-candidates.json", req.Candidates)
	if err != nil {
		return nil, promptInputWriteError("knowledge-candidates.json", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	guidancePath, err := writeMaintainedGuidanceInput(session, req.MaintainedGuidance)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ProjectName":            req.ProjectName,
		"RootPath":               req.RootPath,
		"Language":               req.Language,
		"EvidenceFocusPath":      focusPath,
		"CandidatesPath":         candidatesPath,
		"CandidateCount":         len(req.Candidates),
		"CandidateIDs":           ReviewKnowledgeCandidateIDs(req.Candidates),
		"UserContextPath":        userContextPath,
		"MaintainedGuidancePath": guidancePath,
		"AllowedCategories":      domain.AllowedPatternCategoriesText(),
	}, nil
}

// ReviewKnowledgeCandidateIDs 返回当前审查批次的规范候选 ID，供提示词和结构化契约共同使用。
func ReviewKnowledgeCandidateIDs(candidates []domain.Pattern) []string {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if id := strings.TrimSpace(candidate.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func writeNormalizePatternsInput(session *PromptInputSession, name string, patterns []domain.Pattern) (string, error) {
	data, err := json.MarshalIndent(compactNormalizePatterns(patterns), "", "  ")
	if err != nil {
		return "", err
	}
	return session.Write(name, string(data))
}

func compactNormalizePatterns(patterns []domain.Pattern) []normalizePatternInput {
	out := make([]normalizePatternInput, 0, len(patterns))
	for _, pattern := range patterns {
		item := normalizePatternInput{ID: strings.TrimSpace(pattern.ID), Name: strings.TrimSpace(pattern.Name), Category: string(domain.NormalizePatternCategory(pattern.Category)), Description: strings.TrimSpace(pattern.Description), Rule: strings.TrimSpace(pattern.Rule), EvidencePaths: compactEvidencePaths(pattern.EvidenceLocations), KnowledgeFlags: domain.CanonicalKnowledgeFlags(pattern.KnowledgeFlags)}
		if pattern.BusinessMethod != nil {
			item.CapabilityEntry = &normalizeCapabilityEntryInput{
				Name:               strings.TrimSpace(pattern.BusinessMethod.Name),
				CurrentLocation:    strings.TrimSpace(pattern.BusinessMethod.CodeLocation.CurrentLocation),
				HistoricalLocation: strings.TrimSpace(pattern.BusinessMethod.CodeLocation.HistoricalLocation),
			}
		}
		out = append(out, item)
	}
	return out
}

func compactEvidencePaths(locations []domain.PatternEvidenceLocation) []string {
	seen := make(map[string]struct{}, len(locations))
	paths := make([]string, 0, len(locations))
	for _, location := range locations {
		path := strings.TrimSpace(location.Path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// WorkspacePromptDataRequest 描述工作区画像和规范提示词共享的输入参数。
type WorkspacePromptDataRequest struct {
	WorkspaceName        string
	WorkspaceRoot        string
	WorkspaceInputPath   string
	WorkspaceProfilePath string
	UserContextPath      string
	ProjectIDs           []string
}

// WorkspacePromptData 返回工作区画像和规范提示词共享的路径参数。
func WorkspacePromptData(req WorkspacePromptDataRequest) map[string]interface{} {
	return map[string]interface{}{
		"WorkspaceName":        req.WorkspaceName,
		"WorkspaceRoot":        req.WorkspaceRoot,
		"WorkspaceInputPath":   req.WorkspaceInputPath,
		"WorkspaceProfilePath": req.WorkspaceProfilePath,
		"UserContextPath":      req.UserContextPath,
		"ProjectIDs":           append([]string(nil), req.ProjectIDs...),
		"ProjectIDList":        strings.Join(req.ProjectIDs, ", "),
	}
}

// NewPromptInputSessionForContext 在已知当前 seed 路径时，把提示词输入文件创建到 .skills-seed/runtime 下。
func NewPromptInputSessionForContext(ctx context.Context, prefix string) (*PromptInputSession, error) {
	seedPath := runtimecontext.SeedPath(ctx)
	if seedPath == "" {
		return NewPromptInputSession(prefix)
	}
	return newPromptInputSessionIn(layout.New(seedPath).Runtime(), prefix)
}

// UserDefinePatternPromptData 返回用户自定义模式所需的提示词数据。
func UserDefinePatternPromptData(session *PromptInputSession, req *UserDefinePatternRequest) (map[string]interface{}, error) {
	return map[string]interface{}{
		"Description":       req.Description,
		"Category":          req.Category,
		"UserContext":       req.UserContext,
		"Language":          req.Language,
		"AllowedCategories": domain.AllowedPatternCategoriesText(),
	}, nil
}

func promptInputWriteError(name string, err error) error {
	return fmt.Errorf("%s: %w", i18n.GetWithParams("AgentPromptInputWriteFailed", map[string]interface{}{"Name": name}), err)
}

func writePathListInput(session *PromptInputSession, name string, paths []string) (string, int, error) {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		normalized = append(normalized, path)
	}
	sort.Strings(normalized)
	path, err := session.Write(name, strings.Join(normalized, "\n"))
	if err != nil {
		return "", 0, err
	}
	return path, len(normalized), nil
}

func writeJSONInput(session *PromptInputSession, name string, value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return session.Write(name, string(data))
}

func writeMaintainedGuidanceInput(session *PromptInputSession, guidance maintained.Snapshot) (string, error) {
	if len(guidance.Rules) == 0 && len(guidance.Workflows) == 0 {
		return "", nil
	}
	input := maintainedGuidanceInput{
		Rules:     make([]maintainedRuleInput, 0, len(guidance.Rules)),
		Workflows: make([]maintainedWorkflowInput, 0, len(guidance.Workflows)),
	}
	for _, rule := range guidance.Rules {
		input.Rules = append(input.Rules, maintainedRuleInput{
			ID:               strings.TrimSpace(rule.ID),
			Name:             strings.TrimSpace(rule.Name),
			Summary:          strings.TrimSpace(rule.Summary),
			RouteTerms:       append([]string(nil), rule.RouteTerms...),
			AffectedProjects: append([]string(nil), rule.AffectedProjects...),
			Paths:            append([]string(nil), rule.Paths...),
			Content:          strings.TrimSpace(rule.Content),
		})
	}
	for _, workflow := range guidance.Workflows {
		input.Workflows = append(input.Workflows, maintainedWorkflowInput{
			ID:         strings.TrimSpace(workflow.ID),
			Name:       strings.TrimSpace(workflow.Name),
			Summary:    strings.TrimSpace(workflow.Summary),
			RouteTerms: append([]string(nil), workflow.RouteTerms...),
			Content:    strings.TrimSpace(workflow.Content),
		})
	}
	path, err := writeJSONInput(session, "maintained-guidance.json", input)
	if err != nil {
		return "", promptInputWriteError("maintained-guidance.json", err)
	}
	return path, nil
}

// PlanLearningAgendaPromptData 返回源码证据学习议程规划所需的提示词数据。
func PlanLearningAgendaPromptData(session *PromptInputSession, req *PlanLearningAgendaRequest) (map[string]interface{}, error) {
	focusPathsPath, focusPathCount, err := writePathListInput(session, "analysis-files.txt", req.FocusPaths)
	if err != nil {
		return nil, promptInputWriteError("analysis-files.txt", err)
	}
	sourceFactsPath, err := writeJSONInput(session, "planning-source-facts.json", req.SourceFacts)
	if err != nil {
		return nil, promptInputWriteError("planning-source-facts.json", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, promptInputWriteError("structural-context.md", err)
	}
	userContextPath, err := session.Write("user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	guidancePath, err := writeMaintainedGuidanceInput(session, req.MaintainedGuidance)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ProjectName":            req.ProjectName,
		"RootPath":               req.RootPath,
		"Language":               req.Language,
		"FocusPathsPath":         focusPathsPath,
		"FocusPathCount":         focusPathCount,
		"SourceFactsPath":        sourceFactsPath,
		"SourceFactCount":        len(req.SourceFacts),
		"StructuralContextPath":  structuralContextPath,
		"UserContextPath":        userContextPath,
		"MaintainedGuidancePath": guidancePath,
		"LearningMode":           promptLearningMode(req.LearningMode),
	}, nil
}

// AnalyzeProjectPromptData 返回项目画像分析所需的提示词数据。
func AnalyzeProjectPromptData(session *PromptInputSession, req *AnalyzeProjectRequest) (map[string]interface{}, error) {
	structurePath, err := session.UsePathOrWrite(req.StructurePath, "project-structure.txt", stringx.NormalizeStructureSummary(req.Structure))
	if err != nil {
		return nil, promptInputWriteError("project-structure.txt", err)
	}
	focusPathsPath, focusPathCount, err := writePathListInput(session, "focused-paths.txt", req.FocusPaths)
	if err != nil {
		return nil, promptInputWriteError("focused-paths.txt", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, promptInputWriteError("structural-context.md", err)
	}
	existingProfilePath, err := session.UsePathOrWrite(req.ExistingProfilePath, "existing-profile.json", req.ExistingProfileJSON)
	if err != nil {
		return nil, promptInputWriteError("existing-profile.json", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	guidancePath, err := writeMaintainedGuidanceInput(session, req.MaintainedGuidance)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ProjectName":            req.ProjectName,
		"RootPath":               req.RootPath,
		"Language":               req.Language,
		"StructurePath":          structurePath,
		"StructuralContextPath":  structuralContextPath,
		"ReadmePath":             req.ReadmePath,
		"MainFiles":              req.MainFiles,
		"ExistingProfilePath":    existingProfilePath,
		"FocusPathsPath":         focusPathsPath,
		"FocusPathCount":         focusPathCount,
		"UserContextPath":        userContextPath,
		"MaintainedGuidancePath": guidancePath,
	}, nil
}

// ExtractAuthorityPromptData 返回独立权威知识提取所需的提示词数据。
func ExtractAuthorityPromptData(session *PromptInputSession, req *ExtractAuthorityRequest) (map[string]interface{}, error) {
	knowledgePath, knowledgeCount, err := writePathListInput(session, "engineering-knowledge-paths.txt", req.EngineeringKnowledge)
	if err != nil {
		return nil, promptInputWriteError("engineering-knowledge-paths.txt", err)
	}
	sectionsPath, err := writeJSONInput(session, "authority-sections.json", req.AuthoritySections)
	if err != nil {
		return nil, promptInputWriteError("authority-sections.json", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	guidancePath, err := writeMaintainedGuidanceInput(session, req.MaintainedGuidance)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ProjectName":               req.ProjectName,
		"RootPath":                  req.RootPath,
		"EngineeringKnowledgePath":  knowledgePath,
		"EngineeringKnowledgeCount": knowledgeCount,
		"AuthoritySectionsPath":     sectionsPath,
		"AuthoritySectionCount":     len(req.AuthoritySections),
		"UserContextPath":           userContextPath,
		"MaintainedGuidancePath":    guidancePath,
	}, nil
}

// ReviewAuthorityPromptData 返回独立权威规则复核所需的提示词数据。
func ReviewAuthorityPromptData(session *PromptInputSession, req *ReviewAuthorityRequest) (map[string]interface{}, error) {
	data, err := ExtractAuthorityPromptData(session, &req.ExtractAuthorityRequest)
	if err != nil {
		return nil, err
	}
	candidatePath, err := writeJSONInput(session, "authority-candidate.json", req.Candidate)
	if err != nil {
		return nil, promptInputWriteError("authority-candidate.json", err)
	}
	data["CandidatePath"] = candidatePath
	return data, nil
}

// AnalyzeCurrentCodebaseBatchPromptData 返回批量当前代码库分析所需的提示词数据。
func AnalyzeCurrentCodebaseBatchPromptData(session *PromptInputSession, req *AnalyzeCurrentCodebaseBatchRequest) (map[string]interface{}, error) {
	structurePath, err := session.UsePathOrWrite(req.StructurePath, "project-structure.txt", stringx.NormalizeStructureSummary(req.Structure))
	if err != nil {
		return nil, promptInputWriteError("project-structure.txt", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, promptInputWriteError("structural-context.md", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	guidancePath, err := writeMaintainedGuidanceInput(session, req.MaintainedGuidance)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ProjectName":            req.ProjectName,
		"RootPath":               req.RootPath,
		"Language":               req.Language,
		"RuntimeLabel":           req.RuntimeLabel,
		"SharedContextPath":      strings.TrimSpace(req.SharedContextPath),
		"Focuses":                req.Focuses,
		"FocusIDs":               focusIDsFromEvidence(req.Focuses),
		"StructurePath":          structurePath,
		"StructuralContextPath":  structuralContextPath,
		"MainFiles":              req.MainFiles,
		"UserContextPath":        userContextPath,
		"MaintainedGuidancePath": guidancePath,
		"AllowedCategories":      domain.AllowedPatternCategoriesText(),
		"LearningMode":           promptLearningMode(req.LearningMode),
		"ChangeProfile":          req.ChangeProfile,
	}, nil
}

// AnalyzeCurrentDeltaBatchPromptData 返回 diff 锚定增量学习所需的提示词数据。
func AnalyzeCurrentDeltaBatchPromptData(session *PromptInputSession, req *AnalyzeCurrentDeltaBatchRequest) (map[string]interface{}, error) {
	structurePath, err := session.UsePathOrWrite(req.StructurePath, "focused-structure.txt", stringx.NormalizeStructureSummary(req.Structure))
	if err != nil {
		return nil, promptInputWriteError("focused-structure.txt", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, promptInputWriteError("structural-context.md", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	guidancePath, err := writeMaintainedGuidanceInput(session, req.MaintainedGuidance)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"ProjectName":            req.ProjectName,
		"RootPath":               req.RootPath,
		"Language":               req.Language,
		"RuntimeLabel":           req.RuntimeLabel,
		"SharedContextPath":      strings.TrimSpace(req.SharedContextPath),
		"Focuses":                req.Focuses,
		"FocusIDs":               focusIDs(req.Focuses),
		"StructurePath":          structurePath,
		"StructuralContextPath":  structuralContextPath,
		"UserContextPath":        userContextPath,
		"MaintainedGuidancePath": guidancePath,
		"AllowedCategories":      domain.AllowedPatternCategoriesText(),
		"LearningMode":           promptLearningMode(req.LearningMode),
		"ChangeProfile":          req.ChangeProfile,
	}, nil
}

func focusIDs(focuses []AnalyzeCurrentDeltaFocus) []string {
	ids := make([]string, 0, len(focuses))
	for _, focus := range focuses {
		if id := strings.TrimSpace(focus.EvidenceFocus.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func focusIDsFromEvidence(focuses []AnalyzeCurrentEvidenceFocus) []string {
	ids := make([]string, 0, len(focuses))
	for _, focus := range focuses {
		if id := strings.TrimSpace(focus.EvidenceFocus.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
