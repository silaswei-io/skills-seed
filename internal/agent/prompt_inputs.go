package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func promptLearningMode(mode config.LearningMode) config.LearningMode {
	return config.NormalizeLearningMode(string(mode))
}

func promptLearningScope(scope config.LearningScope) config.LearningScope {
	return config.NormalizeLearningScope(string(scope))
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

// LearningSessionPromptData 返回当前代码学习阶段会话初始化所需的提示词数据。
func LearningSessionPromptData(session *PromptInputSession, req LearningSessionRequest) (map[string]interface{}, error) {
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	return map[string]interface{}{
		"ProjectName":       req.ProjectName,
		"RootPath":          req.RootPath,
		"Language":          req.Language,
		"Stage":             req.Stage,
		"LearningMode":      promptLearningMode(req.LearningMode),
		"LearningScope":     promptLearningScope(req.LearningScope),
		"ChangeProfile":     req.ChangeProfile,
		"UserContextPath":   userContextPath,
		"AllowedCategories": domain.AllowedPatternCategoriesText(),
	}, nil
}

// PlanLearningAgendaPromptData 返回业务学习议程规划所需的提示词数据。
func PlanLearningAgendaPromptData(session *PromptInputSession, req *PlanLearningAgendaRequest) (map[string]interface{}, error) {
	focusPathsPath, focusPathCount, err := writePathListInput(session, "analysis-files.txt", req.FocusPaths)
	if err != nil {
		return nil, promptInputWriteError("analysis-files.txt", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, promptInputWriteError("structural-context.md", err)
	}
	userContextPath, err := session.Write("user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	return map[string]interface{}{
		"ProjectName":           req.ProjectName,
		"RootPath":              req.RootPath,
		"Language":              req.Language,
		"FocusPathsPath":        focusPathsPath,
		"FocusPathCount":        focusPathCount,
		"StructuralContextPath": structuralContextPath,
		"UserContextPath":       userContextPath,
		"LearningMode":          promptLearningMode(req.LearningMode),
		"LearningScope":         promptLearningScope(req.LearningScope),
	}, nil
}

// SelectLearningCandidatesPromptData 返回当前代码学习候选文件 AI 收敛所需的提示词数据。
func SelectLearningCandidatesPromptData(session *PromptInputSession, req *SelectLearningCandidatesRequest) (map[string]interface{}, error) {
	candidatesPath, candidateCount, err := writePathListInput(session, "candidate-files.txt", req.CandidatePaths)
	if err != nil {
		return nil, promptInputWriteError("candidate-files.txt", err)
	}
	requiredPath, requiredCount, err := writePathListInput(session, "required-files.txt", req.RequiredPaths)
	if err != nil {
		return nil, promptInputWriteError("required-files.txt", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, promptInputWriteError("structural-context.md", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, promptInputWriteError("user-context.md", err)
	}
	return map[string]interface{}{
		"ProjectName":           req.ProjectName,
		"RootPath":              req.RootPath,
		"Language":              req.Language,
		"CandidatePathsPath":    candidatesPath,
		"CandidatePathCount":    candidateCount,
		"RequiredPathsPath":     requiredPath,
		"RequiredPathCount":     requiredCount,
		"StructuralContextPath": structuralContextPath,
		"UserContextPath":       userContextPath,
		"LearningMode":          promptLearningMode(req.LearningMode),
		"LearningScope":         promptLearningScope(req.LearningScope),
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
	engineeringKnowledgePath, engineeringKnowledgeCount, err := writePathListInput(session, "engineering-knowledge-paths.txt", req.EngineeringKnowledge)
	if err != nil {
		return nil, promptInputWriteError("engineering-knowledge-paths.txt", err)
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
	return map[string]interface{}{
		"ProjectName":               req.ProjectName,
		"RootPath":                  req.RootPath,
		"Language":                  req.Language,
		"StructurePath":             structurePath,
		"StructuralContextPath":     structuralContextPath,
		"ReadmePath":                req.ReadmePath,
		"MainFiles":                 req.MainFiles,
		"EngineeringKnowledgePath":  engineeringKnowledgePath,
		"EngineeringKnowledgeCount": engineeringKnowledgeCount,
		"ExistingProfilePath":       existingProfilePath,
		"FocusPathsPath":            focusPathsPath,
		"FocusPathCount":            focusPathCount,
		"UserContextPath":           userContextPath,
	}, nil
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
	return map[string]interface{}{
		"ProjectName":           req.ProjectName,
		"RootPath":              req.RootPath,
		"Language":              req.Language,
		"RuntimeLabel":          req.RuntimeLabel,
		"Focuses":               req.Focuses,
		"StructurePath":         structurePath,
		"StructuralContextPath": structuralContextPath,
		"MainFiles":             req.MainFiles,
		"UserContextPath":       userContextPath,
		"AllowedCategories":     domain.AllowedPatternCategoriesText(),
		"LearningMode":          promptLearningMode(req.LearningMode),
		"ChangeProfile":         req.ChangeProfile,
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
	return map[string]interface{}{
		"ProjectName":           req.ProjectName,
		"RootPath":              req.RootPath,
		"Language":              req.Language,
		"RuntimeLabel":          req.RuntimeLabel,
		"Focuses":               req.Focuses,
		"StructurePath":         structurePath,
		"StructuralContextPath": structuralContextPath,
		"UserContextPath":       userContextPath,
		"AllowedCategories":     domain.AllowedPatternCategoriesText(),
		"LearningMode":          promptLearningMode(req.LearningMode),
		"ChangeProfile":         req.ChangeProfile,
	}, nil
}
