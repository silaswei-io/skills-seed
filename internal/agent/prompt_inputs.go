package agent

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/agent/promptfiles"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
)

// PromptInputSession owns temporary files used to keep large prompt inputs out
// of rendered prompt text.
type PromptInputSession = promptfiles.Session

// NewPromptInputSession creates a temporary prompt input file session.
func NewPromptInputSession(prefix string) (*PromptInputSession, error) {
	return promptfiles.New(prefix)
}

// NewPromptInputSessionForContext creates prompt input files under
// .skills-seed/memory/runtime when the current seed path is known.
func NewPromptInputSessionForContext(ctx context.Context, prefix string) (*PromptInputSession, error) {
	seedPath := runtimecontext.SeedPath(ctx)
	if seedPath == "" {
		return NewPromptInputSession(prefix)
	}
	return promptfiles.NewIn(filepath.Join(seedPath, "memory", "runtime"), prefix)
}

// BatchLearnPromptData returns prompt data for commit learning.
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
	}, nil
}

// GenerateSkillsPromptData returns prompt data for AI skill summary generation.
func GenerateSkillsPromptData(session *PromptInputSession, req *GenerateSkillsRequest) (map[string]interface{}, error) {
	patternsPath, err := session.UsePathOrWrite(req.PatternsPath, "patterns.json", req.PatternsJSON)
	if err != nil {
		return nil, fmt.Errorf("write patterns prompt input: %w", err)
	}
	userContextPath, err := session.UsePathOrWrite(req.UserContextPath, "user-context.md", req.UserContext)
	if err != nil {
		return nil, fmt.Errorf("write user context prompt input: %w", err)
	}
	return map[string]interface{}{
		"PROJECT_NAME":         req.ProjectName,
		"LANGUAGE":             req.Language,
		"PATTERNS_PATH":        patternsPath,
		"PATTERNS_COUNT":       req.PatternsCount,
		"EXISTING_SKILLS_PATH": req.ExistingSkillsPath,
		"USER_CONTEXT_PATH":    userContextPath,
	}, nil
}

// AnalyzeProjectPromptData returns prompt data for project profile analysis.
func AnalyzeProjectPromptData(session *PromptInputSession, req *AnalyzeProjectRequest) (map[string]interface{}, error) {
	structurePath, err := session.UsePathOrWrite(req.StructurePath, "project-structure.txt", req.Structure)
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

// AnalyzeCurrentCodebasePromptData returns prompt data for current codebase analysis.
func AnalyzeCurrentCodebasePromptData(session *PromptInputSession, req *AnalyzeCurrentCodebaseRequest) (map[string]interface{}, error) {
	structurePath, err := session.UsePathOrWrite(req.StructurePath, "project-structure.txt", req.Structure)
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
		"KnownPatternsPath":     knownPatternsPath,
		"KnownPatternsCount":    req.KnownPatternsCount,
		"FileCount":             req.FileCount,
		"DirCount":              req.DirCount,
		"UserContextPath":       userContextPath,
	}, nil
}
