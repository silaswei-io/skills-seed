package fileanalysis

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type FileChanges struct {
	Scope                      domain.FileAnalysisScope
	Records                    []domain.FileAnalysisRecord
	AddedOrModified            []string
	Deleted                    []string
	Unchanged                  []string
	Skipped                    []string
	SkippedFiles               []SkippedFile
	ExcludedGeneratedSkillDirs []string
}

type CurrentChangeOptions struct {
	Force bool
}

func PrepareCurrentChanges(ctx context.Context, tracker domain.FileAnalysisTracker, configRepo config.Reader, repoRoot string, scanRoot string, scope domain.FileAnalysisScope, focusAbsPaths []string) (*FileChanges, error) {
	return PrepareCurrentChangesWithOptions(ctx, tracker, configRepo, repoRoot, scanRoot, scope, focusAbsPaths, CurrentChangeOptions{})
}

func PrepareCurrentChangesWithOptions(ctx context.Context, tracker domain.FileAnalysisTracker, configRepo config.Reader, repoRoot string, scanRoot string, scope domain.FileAnalysisScope, focusAbsPaths []string, opts CurrentChangeOptions) (*FileChanges, error) {
	_ = repoRoot
	if tracker == nil {
		return nil, fmt.Errorf("file analysis tracker is nil")
	}
	focusRelPaths := utils.RelativePaths(scanRoot, focusAbsPaths)

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

	policy := NewConfiguredSelectionPolicy(configRepo, scanRoot)
	selection, err := SelectFiles(SelectOptions{
		Root:          scanRoot,
		Policy:        policy,
		FocusAbsPaths: focusAbsPaths,
	})
	if err != nil {
		return nil, err
	}
	current := make(map[string]domain.FileAnalysisRecord)
	changes := &FileChanges{
		Scope:                      scope,
		ExcludedGeneratedSkillDirs: GeneratedSkillExcludeDirs(configRepo, scanRoot),
		SkippedFiles:               append([]SkippedFile{}, selection.Skipped...),
	}
	for _, skipped := range selection.Skipped {
		changes.Skipped = append(changes.Skipped, skipped.Path)
	}
	skippedCurrent := make(map[string]bool, len(selection.Skipped))
	for _, skipped := range selection.Skipped {
		skippedCurrent[filepath.ToSlash(filepath.Clean(skipped.Path))] = true
	}

	for _, file := range selection.Files {
		relPath := filepath.ToSlash(filepath.Clean(file.Path))
		if !scope.ContainsPath(relPath, focusRelPaths) {
			changes.addSkipped(relPath, SkipReasonOutOfFocus)
			continue
		}
		record, err := fingerprintLearnFile(scanRoot, scope, relPath)
		if err != nil {
			changes.addSkipped(relPath, SkipReasonUnreadable)
			continue
		}
		current[relPath] = record
		if prev, ok := previous[relPath]; opts.Force || !ok || prev.Hash != record.Hash {
			changes.AddedOrModified = append(changes.AddedOrModified, relPath)
			changes.Records = append(changes.Records, record)
		} else {
			changes.Unchanged = append(changes.Unchanged, relPath)
		}
	}

	for path := range previous {
		if _, ok := current[path]; !ok {
			if skippedCurrent[path] || policy.IsExcluded(path) {
				continue
			}
			changes.Deleted = append(changes.Deleted, path)
		}
	}

	sort.Strings(changes.AddedOrModified)
	sort.Strings(changes.Deleted)
	sort.Strings(changes.Unchanged)
	sort.Strings(changes.Skipped)
	sort.Slice(changes.SkippedFiles, func(i, j int) bool { return changes.SkippedFiles[i].Path < changes.SkippedFiles[j].Path })
	sort.Slice(changes.Records, func(i, j int) bool { return changes.Records[i].Path < changes.Records[j].Path })
	return changes, nil
}

func CommitCurrentChanges(ctx context.Context, tracker domain.FileAnalysisTracker, changes *FileChanges) error {
	if changes == nil {
		return nil
	}
	if err := tracker.SaveAnalyzedFiles(ctx, changes.Records); err != nil {
		return err
	}
	return tracker.DeleteAnalyzedFiles(ctx, changes.Scope, changes.Deleted)
}

func (c FileChanges) HasChanges() bool {
	return len(c.AddedOrModified) > 0 || len(c.Deleted) > 0
}

func (c FileChanges) FocusPaths() []string {
	paths := append([]string{}, c.AddedOrModified...)
	paths = append(paths, c.Deleted...)
	sort.Strings(paths)
	return paths
}

func (c FileChanges) SkippedCount(reason SkipReason) int {
	count := 0
	for _, skipped := range c.SkippedFiles {
		if skipped.Reason == reason {
			count++
		}
	}
	return count
}

func (c *FileChanges) ApplyAISelection(selectedPaths []string, reason string) {
	if c == nil {
		return
	}
	selected := make(map[string]bool, len(selectedPaths))
	for _, path := range selectedPaths {
		selected[filepath.ToSlash(filepath.Clean(path))] = true
	}
	for i := range c.Records {
		path := filepath.ToSlash(filepath.Clean(c.Records[i].Path))
		if selected[path] {
			c.Records[i].AnalysisStatus = domain.FileAnalysisStatusAnalyzed
			c.Records[i].SelectionReason = ""
			continue
		}
		c.Records[i].AnalysisStatus = domain.FileAnalysisStatusAISkipped
		c.Records[i].SelectionReason = reason
	}
}

func (c *FileChanges) addSkipped(path string, reason SkipReason) {
	if c == nil {
		return
	}
	path = filepath.ToSlash(filepath.Clean(path))
	c.Skipped = append(c.Skipped, path)
	c.SkippedFiles = append(c.SkippedFiles, SkippedFile{Path: path, Reason: reason})
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
		AnalysisStatus: domain.FileAnalysisStatusAnalyzed,
		LastAnalyzedAt: now,
	}, nil
}

func ConfiguredLearnExcludes(configRepo config.Reader, projectRoot string) []string {
	configExcludes := []string{}
	if configRepo != nil {
		configExcludes = configRepo.GetExclude()
	}
	return config.LearnExcludePatterns(configExcludes, GeneratedSkillExcludeDirs(configRepo, projectRoot))
}

func GeneratedSkillExcludeDirs(configRepo config.Reader, projectRoot string) []string {
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
		for _, outputPath := range reader.GetSkillsConfig().Paths {
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
