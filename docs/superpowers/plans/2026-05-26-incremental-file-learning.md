# Incremental File Learning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `learn current` 增加基于文件 md5 的增量学习，未变化时跳过 patterns 与 profile 分析，并默认排除生成的 skills 文件。

**Architecture:** 增加文件分析记录领域模型和 BoltDB tracker，命令层在调用 analyzer 前计算当前可学习文件指纹并生成增量 focus paths。`learn current` 和 workspace 子项目学习复用同一增量准备逻辑，Analyzer 请求携带 known patterns 给 init-skills prompt 降低重复模式输出。

**Tech Stack:** Go, bbolt, Cobra command tests, embedded prompt templates, TOML i18n, existing config/filefilter utilities.

---

## File Structure

- Create: `internal/domain/file_analysis.go`
  - 定义 `FileAnalysisScope`、`FileAnalysisRecord`、md5/source 常量和 key/path 规范化方法；新增注释必须中文。
- Modify: `internal/domain/repository.go`
  - 增加 `FileAnalysisTracker` 接口。
- Modify: `internal/test/mocks/mocks.go`
  - 增加 `MockFileAnalysisTracker`，供 command tests 使用。
- Modify: `internal/infra/storage/boltdb/repository.go`
  - 在现有 project.db 内新增文件分析 bucket，并实现 `FileAnalysisTracker`。
- Modify: `internal/infra/storage/boltdb/repository_test.go`
  - 覆盖文件 tracker 的 save/list/delete/scope 隔离。
- Create: `internal/command/learn/incremental.go`
  - 收集 tracked files、应用 excludes、计算 md5、对比 tracker、生成增量 focus paths。
- Create: `internal/command/learn/incremental_test.go`
  - 覆盖 hash、排除 generated skills、focus paths、删除文件、workspace scope。
- Modify: `internal/container/container.go`
  - 为容器增加 `FileTracker domain.FileAnalysisTracker`，默认复用 `PatternRepository`。
- Modify: `internal/command/learn/command.go`
  - 单项目与 workspace `learn current` 接入增量检测、跳过逻辑、tracker 更新、known patterns。
- Modify: `internal/command/learn/command_test.go`
  - 覆盖无变化跳过、变更文件增量分析、删除文件只刷 profile、workspace scope 隔离。
- Modify: `internal/service/learner/service.go`
  - 暴露 known patterns snapshot 方法，复用现有 reduced JSON。
- Modify: `internal/agent/types.go`
  - `AnalyzeCurrentCodebaseRequest` 增加 `KnownPatternsJSON` 与 `KnownPatternsCount`。
- Modify: `internal/service/analyzer/service.go`
  - `AnalyzeCodebaseOptions` 与 `AnalyzeCurrentCodebaseRequest` 透传 known patterns。
- Modify: `internal/service/analyzer/service_test.go`
  - 覆盖 current-code analyzer 传递 known patterns。
- Modify: `embedfs/templates/prompts/common/init-skills.txt.tmpl`
- Modify: `embedfs/templates/prompts/common/init-skills.zh-CN.txt.tmpl`
  - 增加已有模式提示，要求避免换名重复输出。
- Modify: `internal/prompts/loader_test.go`
  - 覆盖 init-skills prompt 渲染 known patterns。
- Modify: `internal/i18n/locales/active.zh-CN.toml`
- Modify: `internal/i18n/locales/active.en-US.toml`
  - 增加增量检测、无变化跳过、排除 generated skills 等消息。
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `docs/project-generation-guide.md`
- Modify: `docs/project-generation-guide.en.md`
- Modify: `embedfs/templates/config/config.yaml.zh-CN.tmpl`
- Modify: `embedfs/templates/config/config.yaml.en-US.tmpl`
  - 文档说明文件 md5 增量、profile 同步跳过、generated skills 默认排除、workspace 子项目隔离。

## Task 1: Domain Model And Interfaces

**Files:**
- Create: `internal/domain/file_analysis.go`
- Modify: `internal/domain/repository.go`
- Modify: `internal/test/mocks/mocks.go`
- Test: `internal/domain/file_analysis_test.go`

- [ ] **Step 1: Write the failing domain test**

Create `internal/domain/file_analysis_test.go`:

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileAnalysisScopeKeyForPathNormalizesPath(t *testing.T) {
	scope := FileAnalysisScope{ProjectID: "backend", ScopePath: "services/backend"}

	require.Equal(t, "backend\x00services/backend\x00internal/app.go", scope.KeyForPath("./internal\\app.go"))
	require.Equal(t, "backend\x00services/backend\x00", scope.KeyPrefix())
}

func TestFileAnalysisScopeContainsPath(t *testing.T) {
	scope := FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}

	require.True(t, scope.ContainsPath("internal/app.go", []string{"internal"}))
	require.True(t, scope.ContainsPath("internal/app.go", []string{"internal/app.go"}))
	require.False(t, scope.ContainsPath("cmd/main.go", []string{"internal"}))
}
```

- [ ] **Step 2: Run the domain test to verify it fails**

Run: `go test ./internal/domain -run FileAnalysis -count=1`

Expected: FAIL with undefined identifiers such as `FileAnalysisScope`.

- [ ] **Step 3: Add the file analysis domain model**

Create `internal/domain/file_analysis.go`:

```go
package domain

import (
	"path/filepath"
	"strings"
)

const (
	// FileAnalysisHashMD5 表示当前文件增量学习使用 md5 指纹。
	FileAnalysisHashMD5 = "md5"
	// FileAnalysisSourceCurrentCode 表示记录来自 learn current。
	FileAnalysisSourceCurrentCode = "current_code"
)

// FileAnalysisScope 表示文件分析记录的隔离范围。
type FileAnalysisScope struct {
	ProjectID string `json:"project_id,omitempty"`
	ScopePath string `json:"scope_path,omitempty"`
}

// KeyForPath 返回当前 scope 下某个相对路径的稳定存储 key。
func (s FileAnalysisScope) KeyForPath(path string) string {
	return s.KeyPrefix() + normalizeAnalysisPath(path)
}

// KeyPrefix 返回当前 scope 的存储 key 前缀。
func (s FileAnalysisScope) KeyPrefix() string {
	return s.ProjectID + "\x00" + normalizeAnalysisPath(s.ScopePath) + "\x00"
}

// ContainsPath 判断 path 是否落在 focusPaths 指定的相对范围内。
func (s FileAnalysisScope) ContainsPath(path string, focusPaths []string) bool {
	if len(focusPaths) == 0 {
		return true
	}
	path = normalizeAnalysisPath(path)
	for _, focusPath := range focusPaths {
		focusPath = normalizeAnalysisPath(focusPath)
		if focusPath == "." || focusPath == "" || path == focusPath || strings.HasPrefix(path, focusPath+"/") {
			return true
		}
	}
	return false
}

// FileAnalysisRecord 保存单个文件最近一次成功分析时的指纹。
type FileAnalysisRecord struct {
	ProjectID      string `json:"project_id,omitempty"`
	ScopePath      string `json:"scope_path,omitempty"`
	Path           string `json:"path"`
	Hash           string `json:"hash"`
	HashAlgorithm  string `json:"hash_algorithm"`
	Size           int64  `json:"size"`
	ModTime        string `json:"mod_time"`
	Source         string `json:"source"`
	LastAnalyzedAt string `json:"last_analyzed_at"`
}

func normalizeAnalysisPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." {
		return ""
	}
	return strings.TrimPrefix(path, "./")
}
```

- [ ] **Step 4: Add the tracker interface**

Modify `internal/domain/repository.go` after `CommitAnalysisTracker`:

```go
// FileAnalysisTracker 文件分析追踪接口
type FileAnalysisTracker interface {
	// GetAnalyzedFile 获取指定文件最近一次成功分析记录
	GetAnalyzedFile(ctx context.Context, scope FileAnalysisScope, path string) (*FileAnalysisRecord, error)

	// ListAnalyzedFiles 获取指定范围内的全部文件分析记录
	ListAnalyzedFiles(ctx context.Context, scope FileAnalysisScope) ([]FileAnalysisRecord, error)

	// SaveAnalyzedFiles 保存一批文件分析记录
	SaveAnalyzedFiles(ctx context.Context, records []FileAnalysisRecord) error

	// DeleteAnalyzedFiles 删除指定范围内的文件分析记录
	DeleteAnalyzedFiles(ctx context.Context, scope FileAnalysisScope, paths []string) error
}
```

- [ ] **Step 5: Add the mock tracker**

Append to `internal/test/mocks/mocks.go`:

```go
// MockFileAnalysisTracker 模拟文件分析追踪器
type MockFileAnalysisTracker struct {
	GetAnalyzedFileFn    func(ctx context.Context, scope domain.FileAnalysisScope, path string) (*domain.FileAnalysisRecord, error)
	ListAnalyzedFilesFn  func(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error)
	SaveAnalyzedFilesFn  func(ctx context.Context, records []domain.FileAnalysisRecord) error
	DeleteAnalyzedFilesFn func(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error
}

func (m *MockFileAnalysisTracker) GetAnalyzedFile(ctx context.Context, scope domain.FileAnalysisScope, path string) (*domain.FileAnalysisRecord, error) {
	if m.GetAnalyzedFileFn != nil {
		return m.GetAnalyzedFileFn(ctx, scope, path)
	}
	return nil, nil
}

func (m *MockFileAnalysisTracker) ListAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
	if m.ListAnalyzedFilesFn != nil {
		return m.ListAnalyzedFilesFn(ctx, scope)
	}
	return []domain.FileAnalysisRecord{}, nil
}

func (m *MockFileAnalysisTracker) SaveAnalyzedFiles(ctx context.Context, records []domain.FileAnalysisRecord) error {
	if m.SaveAnalyzedFilesFn != nil {
		return m.SaveAnalyzedFilesFn(ctx, records)
	}
	return nil
}

func (m *MockFileAnalysisTracker) DeleteAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
	if m.DeleteAnalyzedFilesFn != nil {
		return m.DeleteAnalyzedFilesFn(ctx, scope, paths)
	}
	return nil
}
```

- [ ] **Step 6: Run the domain tests**

Run: `go test ./internal/domain ./internal/test/mocks -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/file_analysis.go internal/domain/file_analysis_test.go internal/domain/repository.go internal/test/mocks/mocks.go
git commit -m "feat: add file analysis tracker model"
```

## Task 2: BoltDB File Analysis Tracker

**Files:**
- Modify: `internal/infra/storage/boltdb/repository.go`
- Modify: `internal/infra/storage/boltdb/repository_test.go`

- [ ] **Step 1: Write failing BoltDB tracker tests**

Append to `internal/infra/storage/boltdb/repository_test.go`:

```go
func TestPatternRepository_FileAnalysisTracking(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	scope := domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}

	records := []domain.FileAnalysisRecord{
		{
			ProjectID:      "backend",
			ScopePath:      "backend",
			Path:           "internal/app.go",
			Hash:           "abc",
			HashAlgorithm:  domain.FileAnalysisHashMD5,
			Size:           12,
			ModTime:        "2026-05-26T00:00:00Z",
			Source:         domain.FileAnalysisSourceCurrentCode,
			LastAnalyzedAt: "2026-05-26T00:00:01Z",
		},
	}

	require.NoError(t, repo.SaveAnalyzedFiles(ctx, records))

	got, err := repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "abc", got.Hash)

	list, err := repo.ListAnalyzedFiles(ctx, scope)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, repo.DeleteAnalyzedFiles(ctx, scope, []string{"internal/app.go"}))
	got, err = repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestPatternRepository_FileAnalysisTrackingScopesRecords(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, repo.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{
		{ProjectID: "backend", ScopePath: "backend", Path: "main.go", Hash: "backend", HashAlgorithm: domain.FileAnalysisHashMD5},
		{ProjectID: "frontend", ScopePath: "frontend", Path: "main.go", Hash: "frontend", HashAlgorithm: domain.FileAnalysisHashMD5},
	}))

	backend, err := repo.GetAnalyzedFile(ctx, domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}, "main.go")
	require.NoError(t, err)
	require.NotNil(t, backend)
	require.Equal(t, "backend", backend.Hash)

	frontend, err := repo.GetAnalyzedFile(ctx, domain.FileAnalysisScope{ProjectID: "frontend", ScopePath: "frontend"}, "main.go")
	require.NoError(t, err)
	require.NotNil(t, frontend)
	require.Equal(t, "frontend", frontend.Hash)
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/infra/storage/boltdb -run FileAnalysisTracking -count=1`

Expected: FAIL because `PatternRepository` does not implement file tracking methods.

- [ ] **Step 3: Implement the BoltDB bucket and methods**

Modify `internal/infra/storage/boltdb/repository.go`:

```go
var (
	bucketPatterns      = []byte("patterns")
	bucketMetadata      = []byte("metadata")
	bucketAnalyzedFiles = []byte("analyzed_files")
	keyAnalyzedCommits  = []byte("analyzed_commits")
)
```

Add bucket creation inside `NewPatternRepository`:

```go
// 创建 analyzed_files bucket（用于保存 learn current 文件指纹）
if _, err := tx.CreateBucketIfNotExists(bucketAnalyzedFiles); err != nil {
	return fmt.Errorf("failed to create bucket %s: %w", bucketAnalyzedFiles, err)
}
```

Append methods before `Close`:

```go
// GetAnalyzedFile 获取指定文件最近一次成功分析记录
func (r *PatternRepository) GetAnalyzedFile(ctx context.Context, scope domain.FileAnalysisScope, path string) (*domain.FileAnalysisRecord, error) {
	var record *domain.FileAnalysisRecord
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		data := bucket.Get([]byte(scope.KeyForPath(path)))
		if data == nil {
			return nil
		}
		var found domain.FileAnalysisRecord
		if err := json.Unmarshal(data, &found); err != nil {
			return err
		}
		record = &found
		return nil
	})
	return record, err
}

// ListAnalyzedFiles 获取指定范围内的全部文件分析记录
func (r *PatternRepository) ListAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
	records := make([]domain.FileAnalysisRecord, 0)
	prefix := []byte(scope.KeyPrefix())
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		return bucket.ForEach(func(k, v []byte) error {
			if !bytes.HasPrefix(k, prefix) {
				return nil
			}
			var record domain.FileAnalysisRecord
			if err := json.Unmarshal(v, &record); err != nil {
				return err
			}
			records = append(records, record)
			return nil
		})
	})
	return records, err
}

// SaveAnalyzedFiles 保存一批文件分析记录
func (r *PatternRepository) SaveAnalyzedFiles(ctx context.Context, records []domain.FileAnalysisRecord) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		for _, record := range records {
			scope := domain.FileAnalysisScope{ProjectID: record.ProjectID, ScopePath: record.ScopePath}
			record.Path = filepath.ToSlash(filepath.Clean(record.Path))
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			if err := bucket.Put([]byte(scope.KeyForPath(record.Path)), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteAnalyzedFiles 删除指定范围内的文件分析记录
func (r *PatternRepository) DeleteAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		for _, path := range paths {
			if err := bucket.Delete([]byte(scope.KeyForPath(path))); err != nil {
				return err
			}
		}
		return nil
	})
}
```

Add `bytes` to imports. Keep comments in Chinese.

- [ ] **Step 4: Run BoltDB tests**

Run: `go test ./internal/infra/storage/boltdb -run 'FileAnalysisTracking|CommitTracking' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infra/storage/boltdb/repository.go internal/infra/storage/boltdb/repository_test.go
git commit -m "feat: persist analyzed file fingerprints"
```

## Task 3: Incremental File Change Preparation

**Files:**
- Create: `internal/command/learn/incremental.go`
- Create: `internal/command/learn/incremental_test.go`

- [ ] **Step 1: Write failing tests for file changes and generated skills exclusion**

Create `internal/command/learn/incremental_test.go`:

```go
package learn

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/stretchr/testify/require"
)

func TestPrepareIncrementalFileChangesDetectsAddedModifiedAndDeleted(t *testing.T) {
	ctx := context.Background()
	projectRoot := initLearnGitRepo(t)
	writeLearnFile(t, projectRoot, "main.go", "package main\n")
	gitAddAll(t, projectRoot)

	repo := newLearnTracker(t, projectRoot)
	configRepo := newIncrementalConfig(t, projectRoot)
	scope := domain.FileAnalysisScope{}

first, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, scope, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"main.go"}, first.AddedOrModified)
	require.NoError(t, commitIncrementalFileChanges(ctx, repo, first))

	writeLearnFile(t, projectRoot, "main.go", "package main\nconst changed = true\n")
	writeLearnFile(t, projectRoot, "internal/app.go", "package internal\n")
	gitAddAll(t, projectRoot)

second, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, scope, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"main.go", "internal/app.go"}, second.AddedOrModified)
	require.Empty(t, second.Deleted)
	require.Equal(t, []string{"internal/app.go", "main.go"}, second.FocusPaths())

	require.NoError(t, commitIncrementalFileChanges(ctx, repo, second))
	require.NoError(t, os.Remove(filepath.Join(projectRoot, "internal", "app.go")))
	gitAddAll(t, projectRoot)

third, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, scope, nil)
	require.NoError(t, err)
	require.Empty(t, third.AddedOrModified)
	require.Equal(t, []string{"internal/app.go"}, third.Deleted)
	require.Equal(t, []string{"internal/app.go"}, third.FocusPaths())
}

func TestPrepareIncrementalFileChangesExcludesGeneratedSkills(t *testing.T) {
	ctx := context.Background()
	projectRoot := initLearnGitRepo(t)
	writeLearnFile(t, projectRoot, "main.go", "package main\n")
	writeLearnFile(t, projectRoot, ".agents/skills/skills-seed-skills/SKILL.md", "# generated\n")
	writeLearnFile(t, projectRoot, ".claude/skills/skills-seed-skills/SKILL.md", "# generated\n")
	gitAddAll(t, projectRoot)

	repo := newLearnTracker(t, projectRoot)
	configRepo := newIncrementalConfig(t, projectRoot)

changes, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"main.go"}, changes.AddedOrModified)
	require.ElementsMatch(t, []string{".agents/skills/skills-seed-skills", ".claude/skills/skills-seed-skills"}, changes.ExcludedGeneratedSkillDirs)
}
```

Add helper functions in the same test file:

```go
func initLearnGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", root, "init").Run())
	require.NoError(t, exec.Command("git", "-C", root, "config", "user.email", "test@example.com").Run())
	require.NoError(t, exec.Command("git", "-C", root, "config", "user.name", "Test User").Run())
	return root
}

func writeLearnFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func gitAddAll(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, exec.Command("git", "-C", root, "add", "-A").Run())
}

func newLearnTracker(t *testing.T, root string) *boltdb.PatternRepository {
	t.Helper()
	repo, err := boltdb.NewPatternRepository(filepath.Join(root, ".skills-seed", "memory", "project.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	return repo
}

func newIncrementalConfig(t *testing.T, root string) *config.Repository {
	t.Helper()
	repo, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := repo.Get()
	cfg.Project.RootPath = root
	cfg.Agent.Provider = "codex"
	cfg.Output.SkillsPaths = map[string]string{
		"claude": ".claude/skills/skills-seed-skills",
		"codex":  ".agents/skills/skills-seed-skills",
	}
	require.NoError(t, repo.Update(cfg))
	return repo
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/command/learn -run PrepareIncrementalFileChanges -count=1`

Expected: FAIL with undefined functions such as `prepareIncrementalFileChanges`.

- [ ] **Step 3: Implement incremental preparation**

Create `internal/command/learn/incremental.go` with Chinese comments:

```go
package learn

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/git"
	"github.com/silaswei-io/skills-seed/internal/utils"
	"github.com/silaswei-io/skills-seed/internal/utils/filefilter"
)

type incrementalFileChanges struct {
	Scope                      domain.FileAnalysisScope
	Records                    []domain.FileAnalysisRecord
	AddedOrModified            []string
	Deleted                    []string
	Unchanged                  []string
	Skipped                    []string
	ExcludedGeneratedSkillDirs []string
}

func (c incrementalFileChanges) HasChanges() bool {
	return len(c.AddedOrModified) > 0 || len(c.Deleted) > 0
}

func (c incrementalFileChanges) FocusPaths() []string {
	paths := append([]string{}, c.AddedOrModified...)
	paths = append(paths, c.Deleted...)
	sort.Strings(paths)
	return paths
}

// prepareIncrementalFileChanges 计算 learn current 本次需要增量分析的文件。
func prepareIncrementalFileChanges(ctx context.Context, tracker domain.FileAnalysisTracker, configRepo config.Reader, repoRoot string, scanRoot string, scope domain.FileAnalysisScope, focusAbsPaths []string) (*incrementalFileChanges, error) {
	if tracker == nil {
		return nil, fmt.Errorf("file analysis tracker is nil")
	}
	focusRelPaths := utils.RelativePaths(scanRoot, focusAbsPaths)
	files, err := git.NewRepository(repoRoot).GetAllFiles(ctx)
	if err != nil {
		return nil, err
	}

	records, err := tracker.ListAnalyzedFiles(ctx, scope)
	if err != nil {
		return nil, err
	}
	previous := make(map[string]domain.FileAnalysisRecord, len(records))
	for _, record := range records {
		if scope.ContainsPath(record.Path, focusRelPaths) {
			previous[filepath.ToSlash(record.Path)] = record
		}
	}

	excludePatterns := configuredLearnExcludes(configRepo, scanRoot)
	current := make(map[string]domain.FileAnalysisRecord)
	changes := &incrementalFileChanges{
		Scope:                      scope,
		ExcludedGeneratedSkillDirs: generatedSkillExcludeDirs(configRepo, scanRoot),
	}

	for _, file := range files {
		absPath := filepath.Join(repoRoot, filepath.FromSlash(file.Path))
		relPath, err := filepath.Rel(scanRoot, absPath)
		if err != nil || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
			continue
		}
		relPath = filepath.ToSlash(filepath.Clean(relPath))
		if !scope.ContainsPath(relPath, focusRelPaths) || filefilter.MatchExcluded(relPath, excludePatterns) {
			changes.Skipped = append(changes.Skipped, relPath)
			continue
		}
		record, err := fingerprintLearnFile(scanRoot, scope, relPath)
		if err != nil {
			changes.Skipped = append(changes.Skipped, relPath)
			continue
		}
		current[relPath] = record
		if prev, ok := previous[relPath]; !ok || prev.Hash != record.Hash {
			changes.AddedOrModified = append(changes.AddedOrModified, relPath)
			changes.Records = append(changes.Records, record)
		} else {
			changes.Unchanged = append(changes.Unchanged, relPath)
		}
	}

	for path := range previous {
		if _, ok := current[path]; !ok {
			changes.Deleted = append(changes.Deleted, path)
		}
	}

	sort.Strings(changes.AddedOrModified)
	sort.Strings(changes.Deleted)
	sort.Strings(changes.Unchanged)
	sort.Strings(changes.Skipped)
	sort.Slice(changes.Records, func(i, j int) bool { return changes.Records[i].Path < changes.Records[j].Path })
	return changes, nil
}
```

Add helpers:

```go
func fingerprintLearnFile(projectRoot string, scope domain.FileAnalysisScope, relPath string) (domain.FileAnalysisRecord, error) {
	path := filepath.Join(projectRoot, filepath.FromSlash(relPath))
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.FileAnalysisRecord{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.FileAnalysisRecord{}, err
	}
	sum := md5.Sum(data)
	now := time.Now().Format(time.RFC3339)
	return domain.FileAnalysisRecord{
		ProjectID:      scope.ProjectID,
		ScopePath:      scope.ScopePath,
		Path:           relPath,
		Hash:           hex.EncodeToString(sum[:]),
		HashAlgorithm:  domain.FileAnalysisHashMD5,
		Size:           info.Size(),
		ModTime:        info.ModTime().Format(time.RFC3339),
		Source:         domain.FileAnalysisSourceCurrentCode,
		LastAnalyzedAt: now,
	}, nil
}

func commitIncrementalFileChanges(ctx context.Context, tracker domain.FileAnalysisTracker, changes *incrementalFileChanges) error {
	if changes == nil {
		return nil
	}
	if err := tracker.SaveAnalyzedFiles(ctx, changes.Records); err != nil {
		return err
	}
	return tracker.DeleteAnalyzedFiles(ctx, changes.Scope, changes.Deleted)
}
```

Add configured/generated-skill exclude helpers:

```go
func configuredLearnExcludes(configRepo config.Reader, projectRoot string) []string {
	patterns := []string{".git/**", ".skills-seed/**", "vendor/**", "node_modules/**", ".claude/skills/**", ".agents/skills/**"}
	if configRepo != nil {
		patterns = append(patterns, configRepo.GetExclude()...)
	}
	for _, dir := range generatedSkillExcludeDirs(configRepo, projectRoot) {
		if dir != "" {
			patterns = append(patterns, filepath.ToSlash(dir)+"/**")
		}
	}
	return patterns
}

func generatedSkillExcludeDirs(configRepo config.Reader, projectRoot string) []string {
	dirs := make([]string, 0)
	readers := []config.Reader{}
	if configRepo != nil {
		readers = append(readers, configRepo)
	}
	childSeedPath := filepath.Join(projectRoot, ".skills-seed")
	if _, err := os.Stat(filepath.Join(childSeedPath, "config.yaml")); err == nil {
		if childRepo, err := config.NewRepository(childSeedPath, ""); err == nil {
			readers = append(readers, childRepo)
		}
	}
	for _, reader := range readers {
		for _, outputPath := range reader.GetOutputConfig().SkillsPaths {
			if outputPath == "" {
				continue
			}
			resolved, err := utils.ResolvePath(projectRoot, outputPath)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(projectRoot, resolved)
			if err == nil {
				dirs = append(dirs, filepath.ToSlash(rel))
			}
		}
	}
	sort.Strings(dirs)
	return uniqueStrings(dirs)
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
```

- [ ] **Step 4: Run incremental tests**

Run: `go test ./internal/command/learn -run PrepareIncrementalFileChanges -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/command/learn/incremental.go internal/command/learn/incremental_test.go
git commit -m "feat: detect incremental file changes"
```

## Task 4: Single-Project learn current Integration

**Files:**
- Modify: `internal/container/container.go`
- Modify: `internal/service/learner/service.go`
- Modify: `internal/command/learn/command.go`
- Modify: `internal/command/learn/command_test.go`
- Modify: `internal/i18n/locales/active.zh-CN.toml`
- Modify: `internal/i18n/locales/active.en-US.toml`

- [ ] **Step 1: Write failing command tests**

Add counters to `newLearnCurrentTestContainer` in `internal/command/learn/command_test.go` by changing the helper signature to return `*container.Container` only for existing tests, and add focused tests with local counters:

```go
func TestRunLearnCurrentSkipsAIWhenFilesUnchanged(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileAuto)
	defer restoreLearnFlags()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	require.NoError(t, runLearnCurrent(cont))

	analyzeCalls := 0
	profileCalls := 0
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		analyzeCalls++
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeProjectFn = func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
		profileCalls++
		return &agent.AnalyzeProjectResult{}, nil
	}

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.Zero(t, analyzeCalls)
	require.Zero(t, profileCalls)
	require.Contains(t, output, "未检测到可学习文件变化")
}

func TestRunLearnCurrentUsesChangedFilesAsFocusPaths(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileAuto)
	defer restoreLearnFlags()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	require.NoError(t, runLearnCurrent(cont))

	writeLearnFile(t, cont.ConfigRepo.GetProjectConfig().RootPath, "main.go", "package main\nconst changed = true\n")
	gitAddAll(t, cont.ConfigRepo.GetProjectConfig().RootPath)

	var patternFocus []string
	var profileFocus []string
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		patternFocus = append([]string{}, req.FocusPaths...)
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeProjectFn = func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
		profileFocus = append([]string{}, req.FocusPaths...)
		return &agent.AnalyzeProjectResult{Language: "go", Summary: "profile"}, nil
	}

	require.NoError(t, runLearnCurrent(cont))

	require.Equal(t, []string{"main.go"}, patternFocus)
	require.Equal(t, []string{"main.go"}, profileFocus)
}
```

Update `newLearnCurrentTestContainer` so it initializes a git repo and stages tracked files:

```go
projectRoot := initLearnGitRepo(t)
writeLearnFile(t, projectRoot, "main.go", "package main\n")
gitAddAll(t, projectRoot)
```

Update existing workspace tests that write child project files after container creation, such as `TestRunLearnWorkspaceCurrentPrintsProjectTokenUsageAfterProjectLogs`, to call:

```go
gitAddAll(t, cont.ConfigRepo.GetProjectConfig().RootPath)
```

immediately after writing the child file. Incremental collection uses `git ls-files`, so test files must be tracked in the temporary git index.

- [ ] **Step 2: Run command tests to verify failure**

Run: `go test ./internal/command/learn -run 'RunLearnCurrent(SkipsAIWhenFilesUnchanged|UsesChangedFilesAsFocusPaths)' -count=1`

Expected: FAIL because `runLearnCurrent` does not prepare incremental changes.

- [ ] **Step 3: Wire the file tracker into the container**

Modify `internal/container/container.go`:

```go
FileTracker domain.FileAnalysisTracker
```

Set it in `NewContainer` return value:

```go
FileTracker: patternRepo,
```

Set it in tests:

```go
FileTracker: patternRepo,
```

- [ ] **Step 4: Expose known patterns snapshot**

Add to `internal/service/learner/service.go`:

```go
// KnownPatternsSnapshot 返回给当前代码学习使用的已知模式摘要。
func (s *LearnerService) KnownPatternsSnapshot(ctx context.Context) (string, int) {
	return s.marshalKnownPatterns(ctx)
}
```

- [ ] **Step 5: Add i18n messages**

Add to `internal/i18n/locales/active.zh-CN.toml` near current learning messages:

```toml
[ProgressLearnCurrentDetectChanges]
other = "检测增量文件变化"

[LearnCurrentIncrementalSummary]
other = "增量文件变化:\n  • 新增或修改: {{.Changed}}\n  • 删除: {{.Deleted}}\n  • 未变化跳过: {{.Unchanged}}\n  • 排除: {{.Skipped}}"

[LearnCurrentNoFileChanges]
other = "未检测到可学习文件变化，已跳过 patterns 学习和项目画像刷新"

[LearnCurrentGeneratedSkillsExcluded]
other = "已排除生成的 skills 目录: {{.Paths}}"
```

Add to `internal/i18n/locales/active.en-US.toml`:

```toml
[ProgressLearnCurrentDetectChanges]
other = "Detecting incremental file changes"

[LearnCurrentIncrementalSummary]
other = "Incremental file changes:\n  • added or modified: {{.Changed}}\n  • deleted: {{.Deleted}}\n  • unchanged skipped: {{.Unchanged}}\n  • excluded: {{.Skipped}}"

[LearnCurrentNoFileChanges]
other = "No learnable file changes detected; skipped pattern learning and project profile refresh"

[LearnCurrentGeneratedSkillsExcluded]
other = "Excluded generated skills directories: {{.Paths}}"
```

- [ ] **Step 6: Integrate incremental flow in `runLearnCurrent`**

In `internal/command/learn/command.go`, increase progress count:

```go
tracker := progress.New(5)
```

After project preparation and before `AnalyzeCodebaseFullWithOptions`, add:

```go
var incrementalChanges *incrementalFileChanges
var effectiveFocusPaths []string
detectStartedAt := time.Now()
if err := tracker.RunStep(i18n.Get("ProgressLearnCurrentDetectChanges"), func() error {
	var err error
	incrementalChanges, err = prepareIncrementalFileChanges(ctx, cont.FileTracker, cont.ConfigRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, resolvedFocusPaths)
	if err != nil {
		return err
	}
	effectiveFocusPaths = resolveIncrementalFocusPaths(projectRoot, incrementalChanges.FocusPaths())
	return nil
}); err != nil {
	return err
}
logger.Info(i18n.GetWithParams("LearnCurrentIncrementalSummary", map[string]interface{}{
	"Changed":   len(incrementalChanges.AddedOrModified),
	"Deleted":   len(incrementalChanges.Deleted),
	"Unchanged": len(incrementalChanges.Unchanged),
	"Skipped":   len(incrementalChanges.Skipped),
}))
if len(incrementalChanges.ExcludedGeneratedSkillDirs) > 0 {
	logger.Info(i18n.GetWithParams("LearnCurrentGeneratedSkillsExcluded", map[string]interface{}{
		"Paths": strings.Join(incrementalChanges.ExcludedGeneratedSkillDirs, ", "),
	}))
}
logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
	"operation", "command.learn_current.detect_changes",
	"duration", time.Since(detectStartedAt),
	"changed_count", len(incrementalChanges.AddedOrModified),
	"deleted_count", len(incrementalChanges.Deleted),
	"unchanged_count", len(incrementalChanges.Unchanged),
	"skipped_count", len(incrementalChanges.Skipped),
)
```

Add helper:

```go
func resolveIncrementalFocusPaths(projectRoot string, relPaths []string) []string {
	paths := make([]string, 0, len(relPaths))
	for _, relPath := range relPaths {
		paths = append(paths, filepath.Join(projectRoot, filepath.FromSlash(relPath)))
	}
	return paths
}
```

If no changes, skip AI steps but still run progress placeholders and final state update:

```go
if !incrementalChanges.HasChanges() {
	logger.Info(i18n.Get("LearnCurrentNoFileChanges"))
	_ = tracker.RunStep(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error { return nil })
	_ = tracker.RunStep(i18n.Get("ProgressLearnCurrentSavePatterns"), func() error { return nil })
	_ = tracker.RunStep(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil })
	logger.Info(i18n.Get("LearnCurrentComplete"))
	logLearnCurrentNextSteps()
	agent.FlushTokenUsageScope(ctx)
	return nil
}
```

Use `effectiveFocusPaths` in analyzer calls:

```go
analyzeResult, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(ctx, projectRoot, projectName, currentLanguage, analyzer.AnalyzeCodebaseOptions{
	FocusPaths: effectiveFocusPaths,
})
```

For profile refresh, force auto refresh when incremental changes exist unless the user explicitly uses `--profile skip`:

```go
if learnCurrentProfileOpt != learnCurrentProfileSkip && incrementalChanges.HasChanges() {
	refreshProfile = true
}
```

After successful scheduled pattern/profile persistence, commit file records:

```go
if err := commitIncrementalFileChanges(ctx, cont.FileTracker, incrementalChanges); err != nil {
	return err
}
```

- [ ] **Step 7: Run single-project command tests**

Run: `go test ./internal/command/learn -run 'RunLearnCurrent|ResolveFocusPaths|ShouldRefreshProfile' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/container/container.go internal/service/learner/service.go internal/command/learn/command.go internal/command/learn/command_test.go internal/i18n/locales/active.zh-CN.toml internal/i18n/locales/active.en-US.toml
git commit -m "feat: make current learning incremental"
```

## Task 5: Workspace learn current Integration

**Files:**
- Modify: `internal/command/learn/command.go`
- Modify: `internal/command/learn/command_test.go`
- Modify: `internal/i18n/locales/active.zh-CN.toml`
- Modify: `internal/i18n/locales/active.en-US.toml`

- [ ] **Step 1: Write failing workspace tests**

Add to `internal/command/learn/command_test.go`:

```go
func TestRunLearnWorkspaceCurrentSkipsUnchangedChildProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	writeLearnFile(t, cont.ConfigRepo.GetProjectConfig().RootPath, "backend/main.go", "package main\n")
	gitAddAll(t, cont.ConfigRepo.GetProjectConfig().RootPath)

	require.NoError(t, runLearnCurrent(cont))

	analyzeCalls := 0
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		analyzeCalls++
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.Zero(t, analyzeCalls)
	require.Contains(t, output, "子项目 backend 未检测到可学习文件变化")
}

func TestRunLearnWorkspaceCurrentScopesEqualRelativePaths(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "worker", Path: "worker", Type: "backend", Language: "go"},
	}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, projects)
	writeLearnFile(t, cont.ConfigRepo.GetProjectConfig().RootPath, "backend/main.go", "package main\n")
	writeLearnFile(t, cont.ConfigRepo.GetProjectConfig().RootPath, "worker/main.go", "package main\n")
	gitAddAll(t, cont.ConfigRepo.GetProjectConfig().RootPath)

	require.NoError(t, runLearnCurrent(cont))

	recordsBackend, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"})
	require.NoError(t, err)
	require.Len(t, recordsBackend, 1)

	recordsWorker, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{ProjectID: "worker", ScopePath: "worker"})
	require.NoError(t, err)
	require.Len(t, recordsWorker, 1)
}
```

- [ ] **Step 2: Run workspace tests to verify failure**

Run: `go test ./internal/command/learn -run 'RunLearnWorkspaceCurrent(SkipsUnchangedChildProject|ScopesEqualRelativePaths)' -count=1`

Expected: FAIL because workspace flow still analyzes every child project.

- [ ] **Step 3: Add workspace messages**

Add to `internal/i18n/locales/active.zh-CN.toml`:

```toml
[LearnWorkspaceProjectNoFileChanges]
other = "子项目 {{.ProjectName}} 未检测到可学习文件变化，已跳过"
```

Add to `internal/i18n/locales/active.en-US.toml`:

```toml
[LearnWorkspaceProjectNoFileChanges]
other = "Child project {{.ProjectName}} has no learnable file changes; skipped"
```

- [ ] **Step 4: Integrate workspace incremental flow**

In `runLearnWorkspaceCurrent`, before `AnalyzeCodebaseFullWithOptions` inside each project task:

```go
scope := domain.FileAnalysisScope{ProjectID: project.ID, ScopePath: project.Path}
incrementalChanges, err := prepareIncrementalFileChanges(projectCtx, cont.FileTracker, cont.ConfigRepo, projectRoot, projectRootPath, scope, nil)
if err != nil {
	finishProjectLog(projectCtx, "LearnWorkspaceProjectFailed", map[string]interface{}{
		"ProjectName": project.ID,
		"Error":       err.Error(),
	})
	return err
}
if !incrementalChanges.HasChanges() {
	finishProjectLog(projectCtx, "LearnWorkspaceProjectNoFileChanges", map[string]interface{}{"ProjectName": project.ID})
	return nil
}
effectiveFocusPaths := resolveIncrementalFocusPaths(projectRootPath, incrementalChanges.FocusPaths())
```

Pass known patterns and focus paths:

```go
result, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(projectCtx, projectRootPath, project.ID, project.Language, analyzer.AnalyzeCodebaseOptions{
	FocusPaths: effectiveFocusPaths,
})
```

For profile:

```go
projectOptions := analyzer.AnalyzeProjectOptions{FocusPaths: effectiveFocusPaths}
if cont.ProfileRepo != nil {
	if existing, getErr := cont.ProfileRepo.GetForProject(projectCtx, project.ID); getErr == nil {
		projectOptions.ExistingProfile = existing
	}
}
projectAnalysis, err := cont.AnalyzerSvc.AnalyzeProjectFullWithOptions(projectCtx, projectRootPath, project.ID, project.Language, projectOptions)
```

After child profile save succeeds:

```go
if err := commitIncrementalFileChanges(projectCtx, cont.FileTracker, incrementalChanges); err != nil {
	return err
}
```

- [ ] **Step 5: Run workspace command tests**

Run: `go test ./internal/command/learn -run 'RunLearnWorkspaceCurrent' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/command/learn/command.go internal/command/learn/command_test.go internal/i18n/locales/active.zh-CN.toml internal/i18n/locales/active.en-US.toml
git commit -m "feat: apply incremental learning to workspaces"
```

## Task 6: Known Patterns In Current-Code Analysis

**Files:**
- Modify: `internal/agent/types.go`
- Modify: `internal/service/analyzer/service.go`
- Modify: `internal/service/analyzer/service_test.go`
- Modify: `embedfs/templates/prompts/common/init-skills.txt.tmpl`
- Modify: `embedfs/templates/prompts/common/init-skills.zh-CN.txt.tmpl`
- Modify: `internal/prompts/loader_test.go`

- [ ] **Step 1: Write failing prompt and analyzer tests**

Add to `internal/prompts/loader_test.go`:

```go
func TestRenderInitSkillsIncludesKnownPatterns(t *testing.T) {
	tests := []struct {
		locale string
		label  string
	}{
		{locale: "zh-CN", label: "已有模式"},
		{locale: "en-US", label: "Existing Patterns"},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := NewLoader("codex", tt.locale, "")
			req := sampleAnalyzeCurrentCodebaseRequest()
			req.KnownPatternsJSON = `[{"id":"known","name":"Known Pattern","category":"api"}]`
			req.KnownPatternsCount = 1

			prompt, err := loader.Render("init-skills", req)

			require.NoError(t, err)
			require.Contains(t, prompt, tt.label)
			require.Contains(t, prompt, `"name":"Known Pattern"`)
		})
	}
}
```

Add to `internal/service/analyzer/service_test.go`:

```go
func TestAnalyzeCodebaseFullPassesKnownPatterns(t *testing.T) {
	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal:      "mock",
		AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644))

	_, _, err := svc.AnalyzeCodebaseFullWithOptions(context.Background(), tmpDir, "demo", "go", AnalyzeCodebaseOptions{
		KnownPatternsJSON:  `[{"id":"known"}]`,
		KnownPatternsCount: 1,
	})

	require.NoError(t, err)
	require.Equal(t, `[{"id":"known"}]`, received.KnownPatternsJSON)
	require.Equal(t, 1, received.KnownPatternsCount)
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/prompts ./internal/service/analyzer -run 'KnownPatterns|AnalyzeCodebaseFullPassesKnownPatterns' -count=1`

Expected: FAIL because request structs/templates do not include known pattern fields.

- [ ] **Step 3: Add known pattern fields and command wiring**

Modify `internal/agent/types.go`:

```go
KnownPatternsJSON  string       // 已知模式 JSON（不包含代码示例）
KnownPatternsCount int          // 已知模式数量
```

Place these fields in `AnalyzeCurrentCodebaseRequest` after `SampleFiles`.

Modify `internal/service/analyzer/service.go`:

```go
type AnalyzeCurrentCodebaseRequest struct {
	ProjectName        string
	RootPath           string
	Language           string
	FocusPaths         []string
	Structure          string
	StructuralContext  string
	MainFiles          []string
	SampleFiles        []agent.SampleFile
	KnownPatternsJSON  string
	KnownPatternsCount int
}
```

Add the same fields to `AnalyzeCodebaseOptions`, pass them into the request, and copy them into `agent.AnalyzeCurrentCodebaseRequest`.

In `internal/command/learn/command.go`, update the single-project analyzer call:

```go
knownPatternsJSON, knownPatternsCount := cont.LearnerSvc.KnownPatternsSnapshot(ctx)
analyzeResult, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(ctx, projectRoot, projectName, currentLanguage, analyzer.AnalyzeCodebaseOptions{
	FocusPaths:         effectiveFocusPaths,
	KnownPatternsJSON:  knownPatternsJSON,
	KnownPatternsCount: knownPatternsCount,
})
```

In the workspace analyzer call:

```go
knownPatternsJSON, knownPatternsCount := cont.LearnerSvc.KnownPatternsSnapshot(projectCtx)
result, learnedPatterns, err := cont.AnalyzerSvc.AnalyzeCodebaseFullWithOptions(projectCtx, projectRootPath, project.ID, project.Language, analyzer.AnalyzeCodebaseOptions{
	FocusPaths:         effectiveFocusPaths,
	KnownPatternsJSON:  knownPatternsJSON,
	KnownPatternsCount: knownPatternsCount,
})
```

- [ ] **Step 4: Update init-skills templates**

In `embedfs/templates/prompts/common/init-skills.zh-CN.txt.tmpl`, after sample files section:

```gotemplate
{{- if .KnownPatternsJSON}}

# 已有模式 ({{.KnownPatternsCount}})

以下模式已经存在于项目记忆中。分析变更文件时不要把同一规则换名重复输出；如果变更代码补充了已有模式的证据，请复用已有语义并只输出有实质新增的信息。

{{.KnownPatternsJSON}}
{{- end}}
```

In `embedfs/templates/prompts/common/init-skills.txt.tmpl`:

```gotemplate
{{- if .KnownPatternsJSON}}

# Existing Patterns ({{.KnownPatternsCount}})

The following patterns already exist in project memory. Do not re-emit the same rule under a new name. If changed code adds evidence to an existing pattern, reuse the existing concept and output only materially new information.

{{.KnownPatternsJSON}}
{{- end}}
```

- [ ] **Step 5: Run prompt and analyzer tests**

Run: `go test ./internal/prompts ./internal/service/analyzer -run 'KnownPatterns|AnalyzeCodebaseFullPassesKnownPatterns|RenderAllBuiltInPrompts' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/types.go internal/service/analyzer/service.go internal/service/analyzer/service_test.go embedfs/templates/prompts/common/init-skills.txt.tmpl embedfs/templates/prompts/common/init-skills.zh-CN.txt.tmpl internal/prompts/loader_test.go
git commit -m "feat: pass known patterns to current learning"
```

## Task 7: Documentation And Config Template Updates

**Files:**
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `docs/project-generation-guide.md`
- Modify: `docs/project-generation-guide.en.md`
- Modify: `embedfs/templates/config/config.yaml.zh-CN.tmpl`
- Modify: `embedfs/templates/config/config.yaml.en-US.tmpl`

- [ ] **Step 1: Update Chinese README learning section**

In `README.md`, under `### 学习项目知识`, add:

```markdown
`learn current` 首次成功后会记录已分析文件的 md5。后续执行会先比较文件指纹：没有可学习文件变化时会同时跳过 patterns 学习和项目画像刷新；有变化时只围绕新增、修改或删除的文件做增量学习。workspace 模式按子项目隔离记录，一个子项目的变更不会触发其他子项目重新学习。

生成的 skills 目录默认不会参与学习，包括配置中的 `output.skills_paths`，以及 `.claude/skills/**`、`.agents/skills/**`。这可以避免 `SKILL.md` 和 `references/` 被下一轮学习当作普通项目文件。
```

- [ ] **Step 2: Update English README learning section**

In `README.en.md`, under `### Learn Project Knowledge`, add:

```markdown
After the first successful `learn current`, Skills Seed records md5 fingerprints for analyzed files. Later runs compare those fingerprints first: when no learnable files changed, both pattern learning and project profile refresh are skipped; when files changed, only added, modified, or deleted paths drive incremental learning. Workspace mode scopes records per child project, so one child project's change does not re-learn the others.

Generated skills directories are excluded from learning by default, including configured `output.skills_paths`, `.claude/skills/**`, and `.agents/skills/**`. This prevents generated `SKILL.md` and `references/` files from feeding back into future learning.
```

- [ ] **Step 3: Update generation guides**

In `docs/project-generation-guide.md`, after the `learn current` flow diagram, add:

```markdown
`learn current` 会在成功分析后把普通项目文件的 md5 写入 `.skills-seed/memory/project.db`。下一次执行会先比较文件指纹：

- 没有新增、修改或删除的可学习文件：跳过 patterns 学习和项目画像刷新
- 有新增或修改文件：只把这些文件作为增量 focus paths
- 只有删除文件：跳过 patterns 学习，并在已有画像基础上刷新项目画像

生成的 skills 输出目录默认排除，不需要手动写入 `exclude`。
```

In `docs/project-generation-guide.en.md`, add the equivalent:

```markdown
After a successful run, `learn current` stores md5 fingerprints for normal project files in `.skills-seed/memory/project.db`. The next run compares fingerprints first:

- no added, modified, or deleted learnable files: skip pattern learning and profile refresh
- added or modified files: use only those files as incremental focus paths
- deleted files only: skip pattern learning and refresh the profile from the existing profile

Generated skills output directories are excluded by default and do not need to be listed manually in `exclude`.
```

- [ ] **Step 4: Update config templates**

In `embedfs/templates/config/config.yaml.zh-CN.tmpl`, under `exclude:`, add this comment before the list:

```yaml
  # 生成的 skills 输出目录会自动排除，包括 output.skills_paths、.claude/skills/** 和 .agents/skills/**
```

In `embedfs/templates/config/config.yaml.en-US.tmpl`, add:

```yaml
  # Generated skills output directories are excluded automatically, including output.skills_paths, .claude/skills/**, and .agents/skills/**
```

- [ ] **Step 5: Run config and docs-adjacent tests**

Run: `go test ./internal/infra/config ./internal/prompts -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add README.md README.en.md docs/project-generation-guide.md docs/project-generation-guide.en.md embedfs/templates/config/config.yaml.zh-CN.tmpl embedfs/templates/config/config.yaml.en-US.tmpl
git commit -m "docs: document incremental current learning"
```

## Task 8: Final Verification

**Files:**
- Verify all files changed by Tasks 1-7.

- [ ] **Step 1: Format Go code**

Run:

```bash
gofmt -w internal/domain/file_analysis.go internal/domain/file_analysis_test.go internal/test/mocks/mocks.go internal/infra/storage/boltdb/repository.go internal/infra/storage/boltdb/repository_test.go internal/command/learn/incremental.go internal/command/learn/incremental_test.go internal/command/learn/command.go internal/command/learn/command_test.go internal/service/learner/service.go internal/agent/types.go internal/service/analyzer/service.go internal/service/analyzer/service_test.go internal/prompts/loader_test.go internal/container/container.go
```

Expected: command exits 0.

- [ ] **Step 2: Run focused tests**

Run:

```bash
go test ./internal/domain ./internal/infra/storage/boltdb ./internal/command/learn ./internal/service/analyzer ./internal/prompts ./internal/infra/config -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Run vet**

Run:

```bash
go vet ./...
```

Expected: PASS.

- [ ] **Step 5: Build**

Run:

```bash
go build ./cmd/skills-seed
```

Expected: PASS.

- [ ] **Step 6: Check changed comments**

Run:

```bash
rg -n "//|/\\*" internal/domain/file_analysis.go internal/command/learn/incremental.go internal/command/learn/command.go internal/service/analyzer/service.go internal/service/learner/service.go internal/infra/storage/boltdb/repository.go internal/agent/types.go
```

Expected: any new explanatory comments are Chinese. Public identifiers remain English.

- [ ] **Step 7: Inspect status**

Run:

```bash
git status --short
```

Expected: only intentional files from the feature are modified, with no unrelated workspace files staged.
Pre-existing unrelated dirty files may still appear in the worktree; do not stage or revert them.

- [ ] **Step 8: Commit final verification fixes if any**

If formatting or verification required small fixes, commit only the files touched by those fixes. Example:

```bash
git add internal/command/learn/command.go internal/service/analyzer/service.go
git commit -m "test: verify incremental file learning"
```

Expected: commit created only when fixes were needed.

## Self-Review

- Spec coverage:
  - md5 tracking: Tasks 1-3.
  - patterns and profile incremental skip: Tasks 4-5.
  - workspace scope isolation: Tasks 3 and 5.
  - generated skills exclusion: Tasks 3 and 7.
  - known-pattern duplicate reduction: Task 6.
  - documentation updates: Task 7.
  - Chinese comments: Tasks 1-8 include explicit checks.
- Placeholder scan: this plan avoids unresolved markers and gives concrete file paths, commands, tests, and code snippets.
- Type consistency:
  - `FileAnalysisScope`, `FileAnalysisRecord`, and `FileAnalysisTracker` are introduced before use.
  - `incrementalFileChanges` is created before command integration.
  - `KnownPatternsJSON` and `KnownPatternsCount` are added to agent and analyzer request paths before prompt assertions.
