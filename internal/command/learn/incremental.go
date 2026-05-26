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
