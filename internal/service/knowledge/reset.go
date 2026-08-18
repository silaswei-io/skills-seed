// Package knowledge 重置可重新学习的项目知识，保留初始化配置和用户 Context。
package knowledge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

var (
	// ErrScopeRequired 表示重置必须显式选择至少一种知识范围。
	ErrScopeRequired = errors.New("knowledge reset scope is required")
	// ErrSeedNotFound 表示当前目录尚未初始化 skills-seed。
	ErrSeedNotFound = errors.New("skills-seed state is not initialized")
)

// Scope 描述一次知识重置需要移出的资源类别。
type Scope struct {
	Patterns  bool
	Rules     bool
	Workflows bool
	Profiles  bool
	Cache     bool
	History   bool
}

// AllScope 返回除配置、Context 和生成输出外的全部可重置知识范围。
func AllScope() Scope {
	return Scope{
		Patterns:  true,
		Rules:     true,
		Workflows: true,
		Profiles:  true,
		Cache:     true,
		History:   true,
	}
}

// Empty 表示没有选择任何重置范围。
func (s Scope) Empty() bool {
	return !s.Patterns && !s.Rules && !s.Workflows && !s.Profiles && !s.Cache && !s.History
}

// Result 描述一次重置移动的资源和对应备份目录。
type Result struct {
	BackupPath string
	Resources  []string
}

type resource struct {
	id       string
	path     string
	relative string
}

// Reset 将选中的活跃知识移入时间戳备份，使后续学习从干净状态开始。
func Reset(seedPath string, scope Scope) (Result, error) {
	if scope.Empty() {
		return Result{}, ErrScopeRequired
	}
	info, err := os.Stat(seedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, ErrSeedNotFound
		}
		return Result{}, fmt.Errorf("inspect skills-seed state: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("%w: %s is not a directory", ErrSeedNotFound, seedPath)
	}
	seedLayout := layout.New(seedPath)
	if _, err := os.Stat(seedLayout.Config()); err != nil {
		if os.IsNotExist(err) {
			return Result{}, ErrSeedNotFound
		}
		return Result{}, fmt.Errorf("inspect skills-seed config: %w", err)
	}

	resources := selectedResources(seedLayout, scope)
	backupPath := knowledgeBackupPath(seedPath)
	moved := make([]resource, 0, len(resources))
	for _, item := range resources {
		if _, err := os.Lstat(item.path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			rollbackMovedResources(moved, backupPath)
			return Result{}, fmt.Errorf("inspect %s: %w", item.id, err)
		}
		target := filepath.Join(backupPath, item.relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			rollbackMovedResources(moved, backupPath)
			return Result{}, fmt.Errorf("create knowledge backup: %w", err)
		}
		if err := os.Rename(item.path, target); err != nil {
			rollbackMovedResources(moved, backupPath)
			return Result{}, fmt.Errorf("back up %s: %w", item.id, err)
		}
		moved = append(moved, item)
	}

	result := Result{Resources: make([]string, 0, len(moved))}
	if len(moved) == 0 {
		return result, nil
	}
	result.BackupPath = backupPath
	for _, item := range moved {
		result.Resources = append(result.Resources, item.id)
	}
	return result, nil
}

func selectedResources(seedLayout layout.Layout, scope Scope) []resource {
	resources := make([]resource, 0, 9)
	add := func(id, relative string, path string) {
		resources = append(resources, resource{id: id, relative: relative, path: path})
	}
	if scope.Patterns {
		add("patterns", filepath.Join("store", "project.db"), seedLayout.ProjectDB())
	}
	if scope.Rules {
		add("rules", "rules", seedLayout.Rules())
	}
	if scope.Workflows {
		add("workflows", "workflows", seedLayout.Workflows())
	}
	if scope.Profiles {
		add("project-profile", filepath.Join("store", "documents", "project-profile.json"), seedLayout.ProjectProfile())
		add("workspace-profile", filepath.Join("store", "documents", "workspace-profile.json"), seedLayout.WorkspaceProfile())
		add("workspace-spec", filepath.Join("store", "documents", "workspace-spec.json"), seedLayout.WorkspaceSpec())
		add("child-project-profiles", filepath.Join("store", "documents", "projects"), seedLayout.StoreDocuments("projects"))
	}
	if scope.Cache {
		add("file-snapshots", filepath.Join("cache", "snapshots"), seedLayout.Snapshots())
		add("command-checkpoints", filepath.Join("cache", "commands"), seedLayout.CommandStates())
	}
	if scope.History {
		add("learning-history", filepath.Join("store", "documents", "change-log.json"), seedLayout.ChangeLog())
	}
	return resources
}

func knowledgeBackupPath(seedPath string) string {
	return filepath.Join(
		filepath.Dir(seedPath),
		filepath.Base(seedPath)+".backup",
		time.Now().Format("20060102-150405.000000000"),
		"knowledge",
	)
}

func rollbackMovedResources(resources []resource, backupPath string) {
	for index := len(resources) - 1; index >= 0; index-- {
		item := resources[index]
		target := filepath.Join(backupPath, item.relative)
		if err := os.MkdirAll(filepath.Dir(item.path), 0o755); err != nil {
			continue
		}
		_ = os.Rename(target, item.path)
	}
}
