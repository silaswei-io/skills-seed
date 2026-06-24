package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	textutil "github.com/silaswei-io/skills-seed/internal/utils/text"
)

// NewPromptInputSessionForContext 在已知当前 seed 路径时，把提示词输入文件创建到 .skills-seed/memory/runtime 下。
func NewPromptInputSessionForContext(ctx context.Context, prefix string) (*PromptInputSession, error) {
	seedPath := runtimecontext.SeedPath(ctx)
	if seedPath == "" {
		return NewPromptInputSession(prefix)
	}
	return newPromptInputSessionIn(filepath.Join(seedPath, "memory", "runtime"), prefix)
}

// BatchLearnPromptData 返回提交学习所需的提示词数据。
func BatchLearnPromptData(session *PromptInputSession, commits []domain.CommitInfo, commitFiles []CommitFileChange, knownPatternsJSON, knownPatternsPath string, knownPatternsCount int) (map[string]interface{}, error) {
	path, err := session.UsePathOrWrite(knownPatternsPath, "known-patterns.json", knownPatternsJSON)
	if err != nil {
		return nil, fmt.Errorf("write known patterns prompt input: %w", err)
	}
	return map[string]interface{}{
		"Commits":            commits,
		"CommitFiles":        commitFiles,
		"KnownPatternsPath":  path,
		"KnownPatternsCount": knownPatternsCount,
		"AllowedCategories":  domain.AllowedPatternCategoriesText(),
	}, nil
}

// UserDefinePatternPromptData 返回用户自定义模式所需的提示词数据。
func UserDefinePatternPromptData(session *PromptInputSession, req *UserDefinePatternRequest) (map[string]interface{}, error) {
	return map[string]interface{}{
		"Description":       req.Description,
		"Category":          req.Category,
		"Files":             req.Files,
		"UserContext":       req.UserContext,
		"Language":          req.Language,
		"AllowedCategories": domain.AllowedPatternCategoriesText(),
	}, nil
}

// SelectFilesPromptData 返回 AI 文件选择器所需的提示词数据。
func SelectFilesPromptData(session *PromptInputSession, req *SelectFilesRequest) (map[string]interface{}, error) {
	candidatesPath, err := session.Write("candidates.json", mustJSON(req.Candidates))
	if err != nil {
		return nil, fmt.Errorf("write file selection candidates: %w", err)
	}
	userContextPath, err := session.Write("user-context.md", req.UserContext)
	if err != nil {
		return nil, fmt.Errorf("write file selection user context: %w", err)
	}
	return map[string]interface{}{
		"FileTree":        req.FileTree,
		"CandidatesPath":  candidatesPath,
		"UserContextPath": userContextPath,
		"CandidateNum":    req.CandidateNum,
	}, nil
}

func mustJSON(value interface{}) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}

// CheckPromptData 返回检查场景所需的提示词数据。
func CheckPromptData(session *PromptInputSession, req *AnalyzeRequest) (map[string]interface{}, error) {
	return map[string]interface{}{
		"Files":         req.Files,
		"DiffFiles":     req.DiffFiles,
		"Context":       req.Context,
		"Patterns":      req.Patterns,
		"RecentCommits": req.RecentCommits,
	}, nil
}

// AnalyzeProjectPromptData 返回项目画像分析所需的提示词数据。
func AnalyzeProjectPromptData(session *PromptInputSession, req *AnalyzeProjectRequest) (map[string]interface{}, error) {
	structurePath, err := session.UsePathOrWrite(req.StructurePath, "project-structure.txt", textutil.NormalizeStructureSummary(req.Structure))
	if err != nil {
		return nil, fmt.Errorf("write project structure prompt input: %w", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, fmt.Errorf("write structural context prompt input: %w", err)
	}
	existingProfilePath, err := session.UsePathOrWrite(req.ExistingProfilePath, "existing-profile.json", req.ExistingProfileJSON)
	if err != nil {
		return nil, fmt.Errorf("write existing profile prompt input: %w", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, fmt.Errorf("write user context prompt input: %w", err)
	}
	return map[string]interface{}{
		"ProjectName":           req.ProjectName,
		"RootPath":              req.RootPath,
		"Language":              req.Language,
		"StructurePath":         structurePath,
		"StructuralContextPath": structuralContextPath,
		"ReadmePath":            req.ReadmePath,
		"MainFiles":             req.MainFiles,
		"ExistingProfilePath":   existingProfilePath,
		"FocusPaths":            req.FocusPaths,
		"UserContextPath":       userContextPath,
	}, nil
}

// AnalyzeCurrentCodebasePromptData 返回当前代码库分析所需的提示词数据。
func AnalyzeCurrentCodebasePromptData(session *PromptInputSession, req *AnalyzeCurrentCodebaseRequest) (map[string]interface{}, error) {
	structurePath, err := session.UsePathOrWrite(req.StructurePath, "project-structure.txt", textutil.NormalizeStructureSummary(req.Structure))
	if err != nil {
		return nil, fmt.Errorf("write project structure prompt input: %w", err)
	}
	structuralContextPath, err := session.UsePathOrWrite(req.StructuralContextPath, "structural-context.md", req.StructuralContext)
	if err != nil {
		return nil, fmt.Errorf("write structural context prompt input: %w", err)
	}
	knownPatternsPath, err := session.UsePathOrWrite(req.KnownPatternsPath, "known-patterns.json", req.KnownPatternsJSON)
	if err != nil {
		return nil, fmt.Errorf("write known patterns prompt input: %w", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, fmt.Errorf("write user context prompt input: %w", err)
	}
	return map[string]interface{}{
		"ProjectName":           req.ProjectName,
		"RootPath":              req.RootPath,
		"Language":              req.Language,
		"FocusPaths":            req.FocusPaths,
		"StructurePath":         structurePath,
		"StructuralContextPath": structuralContextPath,
		"MainFiles":             req.MainFiles,
		"SampleFiles":           req.SampleFiles,
		"DiffFiles":             req.DiffFiles,
		"KnownPatternsPath":     knownPatternsPath,
		"KnownPatternsCount":    req.KnownPatternsCount,
		"FileCount":             req.FileCount,
		"DirCount":              req.DirCount,
		"UserContextPath":       userContextPath,
		"AllowedCategories":     domain.AllowedPatternCategoriesText(),
	}, nil
}
