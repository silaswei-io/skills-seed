package snapshotflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	snapshotstore "github.com/silaswei-io/skills-seed/internal/infra/storage/snapshot"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	snapshotdiff "github.com/silaswei-io/skills-seed/internal/snapshot"
)

// Result contains the request inputs derived from comparing current files to snapshots.
type Result struct {
	AddedFiles   []domain.FileInfo
	DiffFiles    []agent.DiffFileRef
	CurrentFiles map[string]string
	MergedFiles  map[string]string
	Repository   *snapshotstore.Repository
}

// Build reads current file contents, compares them with stored snapshots, and
// returns the added files and modified-file diff references for an AI request.
func Build(ctx context.Context, projectRoot string, files []domain.FileInfo) (*Result, error) {
	return BuildScoped(ctx, projectRoot, files, nil)
}

// BuildScoped is like Build, but MergedFiles replaces only snapshots inside
// scopePaths and preserves snapshots outside the scope.
func BuildScoped(ctx context.Context, projectRoot string, files []domain.FileInfo, scopePaths []string) (*Result, error) {
	seedPath := seedPathFor(ctx, projectRoot)
	repo := snapshotstore.NewRepository(seedPath)
	oldSnapshots, err := repo.Load()
	if err != nil {
		return nil, err
	}

	currentFiles, err := readCurrentFiles(projectRoot, files)
	if err != nil {
		return nil, err
	}

	runtimeDir := filepath.Join(seedPath, "memory", "runtime")
	changes, err := snapshotdiff.CompareScoped(currentFiles, oldSnapshots, runtimeDir, scopePaths)
	if err != nil {
		return nil, err
	}

	byPath := make(map[string]domain.FileInfo, len(files))
	for _, file := range files {
		file.Content = ""
		byPath[file.Path] = file
	}

	result := &Result{
		AddedFiles:   []domain.FileInfo{},
		DiffFiles:    []agent.DiffFileRef{},
		CurrentFiles: currentFiles,
		MergedFiles:  mergeSnapshots(oldSnapshots, currentFiles, scopePaths),
		Repository:   repo,
	}
	for _, change := range changes {
		switch change.Status {
		case snapshotdiff.ChangeAdded:
			result.AddedFiles = append(result.AddedFiles, byPath[change.Path])
		case snapshotdiff.ChangeModified, snapshotdiff.ChangeDeleted:
			result.DiffFiles = append(result.DiffFiles, agent.DiffFileRef{
				Path:     change.Path,
				DiffPath: change.DiffPath,
			})
		}
	}
	return result, nil
}

func mergeSnapshots(oldSnapshots, currentFiles map[string]string, scopePaths []string) map[string]string {
	if len(scopePaths) == 0 {
		return currentFiles
	}
	merged := make(map[string]string, len(oldSnapshots)+len(currentFiles))
	for path, content := range oldSnapshots {
		if pathInScope(path, scopePaths) {
			continue
		}
		merged[path] = content
	}
	for path, content := range currentFiles {
		merged[path] = content
	}
	return merged
}

func pathInScope(path string, scopePaths []string) bool {
	path = strings.Trim(filepath.ToSlash(path), "/")
	for _, scope := range scopePaths {
		scope = strings.Trim(filepath.ToSlash(scope), "/")
		if scope == "" {
			continue
		}
		if path == scope || strings.HasPrefix(path, scope+"/") {
			return true
		}
	}
	return false
}

func seedPathFor(ctx context.Context, projectRoot string) string {
	if seedPath := runtimecontext.SeedPath(ctx); seedPath != "" {
		return seedPath
	}
	return filepath.Join(projectRoot, ".skills-seed")
}

func readCurrentFiles(projectRoot string, files []domain.FileInfo) (map[string]string, error) {
	current := make(map[string]string, len(files))
	for _, file := range files {
		path := filepath.Join(projectRoot, filepath.FromSlash(file.Path))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read current file %s: %w", file.Path, err)
		}
		current[file.Path] = string(data)
	}
	return current, nil
}
