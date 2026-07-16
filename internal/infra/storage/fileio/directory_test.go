package fileio

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/require"
)

func TestReplaceDirKeepsExistingTreeWhenBuildFails(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output")
	require.NoError(t, os.MkdirAll(target, 0o755))
	oldPath := filepath.Join(target, "old.txt")
	require.NoError(t, os.WriteFile(oldPath, []byte("old"), 0o644))

	err := ReplaceDir(target, func(staging string) error {
		require.NoError(t, os.WriteFile(filepath.Join(staging, "partial.txt"), []byte("partial"), 0o644))
		return errors.New("build failed")
	})

	require.Error(t, err)
	content, readErr := os.ReadFile(oldPath)
	require.NoError(t, readErr)
	require.Equal(t, "old", string(content))
	require.NoFileExists(t, filepath.Join(target, "partial.txt"))
}

func TestReplaceDirPublishesCompleteTree(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "retired.txt"), []byte("old"), 0o644))

	err := ReplaceDir(target, func(staging string) error {
		return os.WriteFile(filepath.Join(staging, "current.txt"), []byte("new"), 0o644)
	})

	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(target, "retired.txt"))
	require.FileExists(t, filepath.Join(target, "current.txt"))
	info, statErr := os.Stat(target)
	require.NoError(t, statErr)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestReplaceDirRecoversInterruptedBackup(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "output")
	backup := backupPath(target)
	staging := filepath.Join(parent, ".output-staging-123456789")
	require.NoError(t, os.MkdirAll(backup, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(backup, "old.txt"), []byte("old"), 0o644))
	require.NoError(t, os.MkdirAll(staging, 0o755))
	lockFile, err := lockPath(target)
	require.NoError(t, err)
	require.NoError(t, saveDirectoryTransaction(lockFile+".transaction.json", directoryTransaction{
		Target: target, Staging: staging, Backup: backup, HadTarget: true,
	}))

	err = ReplaceDir(target, func(staging string) error { return errors.New("stop after recovery") })

	require.Error(t, err)
	require.FileExists(t, filepath.Join(target, "old.txt"))
	require.NoDirExists(t, backup)
}

func TestReplaceDirFinishesInterruptedPublish(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "output")
	backup := backupPath(target)
	staging := filepath.Join(parent, ".output-staging-123456789")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "new.txt"), []byte("new"), 0o644))
	require.NoError(t, os.MkdirAll(backup, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(backup, "old.txt"), []byte("old"), 0o644))
	lockFile, err := lockPath(target)
	require.NoError(t, err)
	require.NoError(t, saveDirectoryTransaction(lockFile+".transaction.json", directoryTransaction{
		Target: target, Staging: staging, Backup: backup, HadTarget: true,
	}))

	err = ReplaceDir(target, func(staging string) error { return errors.New("stop after recovery") })

	require.Error(t, err)
	require.FileExists(t, filepath.Join(target, "new.txt"))
	require.NoDirExists(t, backup)
}

func TestReplaceDirOnlyRemovesRecordedStagingDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "output")
	stale := filepath.Join(parent, ".output-staging-123456789")
	unrelated := filepath.Join(parent, ".output-staging-987654321")
	require.NoError(t, os.MkdirAll(stale, 0o755))
	require.NoError(t, os.MkdirAll(unrelated, 0o755))
	lockFile, err := lockPath(target)
	require.NoError(t, err)
	require.NoError(t, saveDirectoryTransaction(lockFile+".transaction.json", directoryTransaction{
		Target: target, Staging: stale, Backup: backupPath(target),
	}))

	err = ReplaceDir(target, func(staging string) error { return errors.New("stop after cleanup") })

	require.Error(t, err)
	require.NoDirExists(t, stale)
	require.DirExists(t, unrelated)
}

func TestReplaceDirPreservesUnownedBackup(t *testing.T) {
	require.NoError(t, i18n.Init(i18n.LocaleEnglish))
	parent := t.TempDir()
	target := filepath.Join(parent, "output")
	backup := backupPath(target)
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.MkdirAll(backup, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(backup, "user.txt"), []byte("user data"), 0o644))

	err := ReplaceDir(target, func(staging string) error {
		return os.WriteFile(filepath.Join(staging, "new.txt"), []byte("new"), 0o644)
	})

	require.ErrorContains(t, err, "backup path already exists")
	require.FileExists(t, filepath.Join(backup, "user.txt"))
}

func TestReplaceDirSerializesSameTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output")
	var active int32
	var maxActive int32
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ReplaceDir(target, func(staging string) error {
				current := atomic.AddInt32(&active, 1)
				for {
					previous := atomic.LoadInt32(&maxActive)
					if current <= previous || atomic.CompareAndSwapInt32(&maxActive, previous, current) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return os.WriteFile(filepath.Join(staging, "complete.txt"), []byte("complete"), 0o644)
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, int32(1), maxActive)
	require.FileExists(t, filepath.Join(target, "complete.txt"))
}

func TestReplaceDirValidatesAgainBeforePublish(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output")
	var calls int
	err := ReplaceDirWithOptions(target, ReplaceDirOptions{
		Mode: 0o755,
		Validate: func() error {
			calls++
			if calls == 2 {
				return errors.New("boundary changed")
			}
			return nil
		},
	}, func(staging string) error {
		return os.WriteFile(filepath.Join(staging, "new.txt"), []byte("new"), 0o644)
	})

	require.Error(t, err)
	require.Equal(t, 2, calls)
	require.NoDirExists(t, target)
}
