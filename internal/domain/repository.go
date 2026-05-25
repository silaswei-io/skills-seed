package domain

import "context"

// PatternRepository 模式仓储接口
type PatternRepository interface {
	// Get 根据ID获取模式
	Get(ctx context.Context, id string) (*Pattern, error)

	// GetAll 获取所有模式
	GetAll(ctx context.Context) ([]Pattern, error)

	// GetByCategory 根据分类获取模式
	GetByCategory(ctx context.Context, category Category) ([]Pattern, error)

	// GetHighConfidence 获取高置信度模式
	GetHighConfidence(ctx context.Context, threshold float64) ([]Pattern, error)

	// Save 保存模式
	Save(ctx context.Context, p *Pattern) error

	// FindSimilar 查找相似的模式（通过名称和分类）
	FindSimilar(ctx context.Context, pattern *Pattern) (*Pattern, error)

	// Delete 删除模式
	Delete(ctx context.Context, id string) error

	// Count 统计模式数量
	Count(ctx context.Context) (int, error)
}

// CommitAnalysisTracker 提交分析追踪接口
type CommitAnalysisTracker interface {
	// MarkCommitAnalyzed 标记commit已被分析
	MarkCommitAnalyzed(ctx context.Context, commitHash string) error

	// IsCommitAnalyzed 检查commit是否已被分析
	IsCommitAnalyzed(ctx context.Context, commitHash string) (bool, error)

	// GetAnalyzedCommits 获取所有已分析的commit列表
	GetAnalyzedCommits(ctx context.Context) ([]string, error)
}

// ProjectProfileRepository stores the durable project profile used for generated references
type ProjectProfileRepository interface {
	// Get returns the latest project profile
	Get(ctx context.Context) (*ProjectProfile, error)

	// Save stores the latest project profile
	Save(ctx context.Context, profile *ProjectProfile) error
}

// GitRepository Git 操作接口
type GitRepository interface {
	// GetCommits 获取提交历史
	// since 参数支持时间过滤，格式如 "30d", "7d", "1m" 等
	GetCommits(ctx context.Context, limit int, since string) ([]CommitInfo, error)

	// GetChangedFiles 获取指定提交涉及的文件路径
	GetChangedFiles(ctx context.Context, hash string) ([]string, error)

	// GetStagedFiles 获取暂存文件
	GetStagedFiles(ctx context.Context) ([]FileInfo, error)

	// GetAllFiles 获取所有文件
	GetAllFiles(ctx context.Context) ([]FileInfo, error)

	// GetCurrentBranch 获取当前分支
	GetCurrentBranch(ctx context.Context) (string, error)

	// GetProjectRoot 获取项目根目录
	GetProjectRoot(ctx context.Context) (string, error)

	// Stash 将当前修改保存到 stash
	Stash(ctx context.Context, message string) error

	// CreateBranch 创建并切换到新分支
	CreateBranch(ctx context.Context, branchName string) error

	// Checkout 切换到指定分支
	Checkout(ctx context.Context, branchName string) error
}
