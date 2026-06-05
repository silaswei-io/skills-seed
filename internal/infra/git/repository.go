package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// Repository Git 仓储实现
type Repository struct {
	projectRoot string
}

// NewRepository 创建 Git 仓储
func NewRepository(projectRoot string) *Repository {
	return &Repository{
		projectRoot: projectRoot,
	}
}

// GetCommits 获取提交历史
// 返回最近的 limit 个提交，按从旧到新的顺序排列
// since 参数支持时间过滤，格式如 "30d", "7d", "1m" 等
func (r *Repository) GetCommits(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
	// 1. 构建git log命令
	args := []string{
		"log",
		fmt.Sprintf("--max-count=%d", limit),
		"--pretty=format:%H|%an|%ad|%s",
		"--date=iso",
	}

	// 添加时间过滤
	if since != "" {
		args = append(args, fmt.Sprintf("--since=%s", since))
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("GitLogFailed"), err)
	}

	lines := strings.Split(string(output), "\n")
	commits := make([]domain.CommitInfo, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		date, err := time.Parse("2006-01-02 15:04:05 -0700", parts[2])
		if err != nil {
			continue
		}

		commits = append(commits, domain.NewCommitInfo(
			parts[0], // hash
			parts[1], // author
			parts[3], // message
			date,     // date
		))
	}

	// 2. 反转数组，使其按从旧到新的顺序
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	return commits, nil
}

// GetChangedFiles 获取指定提交涉及的文件路径
func (r *Repository) GetChangedFiles(ctx context.Context, hash string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "show", "--name-only", "--format=", hash)
	cmd.Dir = r.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("GitShowFailed"), err)
	}

	return nonEmptyLines(output), nil
}

// GetStagedFiles 获取暂存文件
func (r *Repository) GetStagedFiles(ctx context.Context) ([]domain.FileInfo, error) {
	// 获取暂存文件列表
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--name-status")
	cmd.Dir = r.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("GitDiffCachedFailed"), err)
	}

	lines := strings.Split(string(output), "\n")
	files := make([]domain.FileInfo, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		// 格式: M\tpath/to/file.go
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		status := r.parseStatus(parts[0])
		filePath := parts[1]

		files = append(files, domain.FileInfo{
			Path:     filePath,
			Language: domain.NewFileInfo(filePath, "").Language,
			Status:   status,
		})
	}

	return files, nil
}

// GetCurrentBranch 获取当前分支
func (r *Repository) GetCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", i18n.Get("GitRevParseFailed"), err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetProjectRoot 获取项目根目录
func (r *Repository) GetProjectRoot(ctx context.Context) (string, error) {
	return r.projectRoot, nil
}

// parseStatus 解析文件状态
func (r *Repository) parseStatus(status string) domain.Status {
	switch strings.TrimSpace(status) {
	case "A":
		return domain.StatusAdded
	case "D":
		return domain.StatusDeleted
	default:
		return domain.StatusModified
	}
}

func nonEmptyLines(output []byte) []string {
	lines := strings.Split(string(output), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// Stash 将当前修改保存到 stash
func (r *Repository) Stash(ctx context.Context, message string) error {
	cmd := exec.CommandContext(ctx, "git", "stash", "push", "-m", message)
	cmd.Dir = r.projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w, output: %s", i18n.Get("GitStashFailed"), err, string(output))
	}

	return nil
}

// CreateBranch 创建并切换到新分支
func (r *Repository) CreateBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName)
	cmd.Dir = r.projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w, output: %s", i18n.Get("GitCreateBranchFailed"), err, string(output))
	}

	return nil
}

// Checkout 切换到指定分支
func (r *Repository) Checkout(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", branchName)
	cmd.Dir = r.projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w, output: %s", i18n.Get("GitCheckoutFailed"), err, string(output))
	}

	return nil
}
