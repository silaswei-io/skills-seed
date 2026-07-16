package autofix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// Strategy 修复策略类型
type Strategy string

const (
	StrategyPatch  Strategy = "patch"  // 生成 patch 文件
	StrategyBackup Strategy = "backup" // 创建备份后修改
	StrategyStash  Strategy = "stash"  // 使用 git stash 保存修改
	StrategyBranch Strategy = "branch" // 创建新分支应用修复
)

// FixResult 修复结果
type FixResult struct {
	Strategy   Strategy
	Success    bool
	OutputPath string // patch 文件路径或备份目录
	Message    string
}

// AutofixService 自动修复服务
type AutofixService struct {
	strategy  Strategy
	backupDir string
	gitRepo   domain.GitRepository
}

// NewAutofixService 创建自动修复服务
func NewAutofixService(strategy, backupDir string, gitRepo domain.GitRepository) *AutofixService {
	return &AutofixService{
		strategy:  Strategy(strategy),
		backupDir: backupDir,
		gitRepo:   gitRepo,
	}
}

// FixIssues 修复问题
func (s *AutofixService) FixIssues(ctx context.Context, issues []domain.Issue, fixes map[string]string) (*FixResult, error) {
	switch s.strategy {
	case StrategyPatch:
		return s.fixWithPatch(ctx, issues, fixes)
	case StrategyBackup:
		return s.fixWithBackup(ctx, issues, fixes)
	case StrategyStash:
		return s.fixWithStash(ctx, issues, fixes)
	case StrategyBranch:
		return s.fixWithBranch(ctx, issues, fixes)
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", s.strategy)
	}
}

// fixWithPatch 使用 patch 策略修复
func (s *AutofixService) fixWithPatch(ctx context.Context, issues []domain.Issue, fixes map[string]string) (*FixResult, error) {
	// 创建 .skills-seed/patches 目录
	patchesDir := filepath.Join(s.backupDir, "patches")
	if err := os.MkdirAll(patchesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create patches directory: %w", err)
	}

	// 生成 patch 文件名
	timestamp := time.Now().Format("20060102-150405")
	patchFileName := fmt.Sprintf("fix-%s.patch", timestamp)
	patchPath := filepath.Join(patchesDir, patchFileName)

	// 构建 patch 内容
	var patchContent strings.Builder
	patchContent.WriteString(i18n.GetWithParams("AutoFixPatchHeader", map[string]interface{}{
		"GeneratedAt": time.Now().Format(time.RFC3339),
		"IssueCount":  len(issues),
	}))
	patchContent.WriteString("\n\n")

	// 为每个文件生成 diff
	for file, fix := range fixes {
		originalPath, err := s.resolveFilePath(ctx, file)
		if err != nil {
			return nil, err
		}
		original, err := os.ReadFile(originalPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}
		patchContent.WriteString(buildFullFilePatch(file, string(original), fix))
	}

	// 写入 patch 文件
	if err := os.WriteFile(patchPath, []byte(patchContent.String()), 0644); err != nil {
		return nil, fmt.Errorf("failed to write patch file: %w", err)
	}

	return &FixResult{
		Strategy:   StrategyPatch,
		Success:    true,
		OutputPath: patchPath,
		Message: i18n.GetWithParams("AutoFixPatchCreated", map[string]interface{}{
			"Path": patchPath,
		}),
	}, nil
}

// fixWithBackup 使用 backup 策略修复
func (s *AutofixService) fixWithBackup(ctx context.Context, issues []domain.Issue, fixes map[string]string) (*FixResult, error) {
	// 创建备份目录
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(s.backupDir, "backups", timestamp)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 备份并修改每个文件
	for file, fix := range fixes {
		// 读取原文件
		originalPath, err := s.resolveFilePath(ctx, file)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(originalPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		// 保存备份
		backupFile := filepath.Join(backupPath, safeBackupPath(file))
		if err := os.MkdirAll(filepath.Dir(backupFile), 0755); err != nil {
			return nil, fmt.Errorf("failed to create backup directory for %s: %w", file, err)
		}
		if err := os.WriteFile(backupFile, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to backup file %s: %w", file, err)
		}

		// 应用修复
		if err := os.WriteFile(originalPath, []byte(fix), 0644); err != nil {
			return nil, fmt.Errorf("failed to write fix to %s: %w", file, err)
		}
	}

	return &FixResult{
		Strategy:   StrategyBackup,
		Success:    true,
		OutputPath: backupPath,
		Message: i18n.GetWithParams("AutoFixBackupCreated", map[string]interface{}{
			"Path": backupPath,
		}),
	}, nil
}

// fixWithStash 使用 git stash 策略修复
func (s *AutofixService) fixWithStash(ctx context.Context, issues []domain.Issue, fixes map[string]string) (*FixResult, error) {
	// 1. 创建带消息的 stash
	timestamp := time.Now().Format("20060102-150405")
	stashMessage := fmt.Sprintf("skills-seed-auto-fix-%s", timestamp)

	// 2. 先应用修复
	for file, fix := range fixes {
		path, err := s.resolveFilePath(ctx, file)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(fix), 0644); err != nil {
			return nil, fmt.Errorf("failed to write fix to %s: %w", file, err)
		}
	}

	// 3. 将修复的内容 stash
	if err := s.gitRepo.Stash(ctx, stashMessage); err != nil {
		return nil, fmt.Errorf("failed to stash changes: %w", err)
	}

	return &FixResult{
		Strategy: StrategyStash,
		Success:  true,
		Message: i18n.GetWithParams("AutoFixStashCreated", map[string]interface{}{
			"Message": stashMessage,
		}),
	}, nil
}

// fixWithBranch 创建新分支应用修复
func (s *AutofixService) fixWithBranch(ctx context.Context, issues []domain.Issue, fixes map[string]string) (*FixResult, error) {
	// 1. 生成新分支名
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("skills-seed/auto-fix-%s", timestamp)

	// 2. 创建并切换到新分支
	if err := s.gitRepo.CreateBranch(ctx, branchName); err != nil {
		return nil, fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	// 3. 在新分支上应用修复
	for file, fix := range fixes {
		path, err := s.resolveFilePath(ctx, file)
		if err != nil {
			_ = s.gitRepo.Checkout(ctx, "-")
			return nil, err
		}
		if err := os.WriteFile(path, []byte(fix), 0644); err != nil {
			// 如果失败，切回原分支
			_ = s.gitRepo.Checkout(ctx, "-")
			return nil, fmt.Errorf("failed to write fix to %s: %w", file, err)
		}
	}

	return &FixResult{
		Strategy:   StrategyBranch,
		Success:    true,
		OutputPath: branchName,
		Message: i18n.GetWithParams("AutoFixBranchCreated", map[string]interface{}{
			"BranchName": branchName,
		}),
	}, nil
}

func (s *AutofixService) resolveFilePath(ctx context.Context, file string) (string, error) {
	clean := filepath.Clean(file)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to fix path outside project root: %s", file)
	}
	if s.gitRepo != nil {
		if root, err := s.gitRepo.GetProjectRoot(ctx); err == nil && root != "" {
			path := clean
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, clean)
			}
			rel, err := filepath.Rel(root, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("refusing to fix path outside project root: %s", file)
			}
			return path, nil
		}
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("refusing to fix absolute path without project root: %s", file)
	}
	return clean, nil
}

func safeBackupPath(file string) string {
	if filepath.IsAbs(file) {
		return filepath.Base(file)
	}
	clean := filepath.Clean(file)
	for strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		clean = strings.TrimPrefix(clean, ".."+string(filepath.Separator))
	}
	if clean == ".." || clean == "." {
		return filepath.Base(file)
	}
	return clean
}

func buildFullFilePatch(file, original, fixed string) string {
	oldLines := splitPatchLines(original)
	newLines := splitPatchLines(fixed)
	oldStart := 1
	newStart := 1
	if len(oldLines) == 0 {
		oldStart = 0
	}
	if len(newLines) == 0 {
		newStart = 0
	}

	var patch strings.Builder
	patch.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", file, file))
	patch.WriteString(fmt.Sprintf("--- a/%s\n", file))
	patch.WriteString(fmt.Sprintf("+++ b/%s\n", file))
	patch.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, len(oldLines), newStart, len(newLines)))
	for _, line := range oldLines {
		patch.WriteString("-")
		patch.WriteString(ensureTrailingNewline(line))
	}
	for _, line := range newLines {
		patch.WriteString("+")
		patch.WriteString(ensureTrailingNewline(line))
	}
	return patch.String()
}

func splitPatchLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func ensureTrailingNewline(line string) string {
	if strings.HasSuffix(line, "\n") {
		return line
	}
	return line + "\n"
}
