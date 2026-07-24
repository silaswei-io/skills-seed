package fileio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// ReplaceDirOptions 配置目录替换的权限和发布前边界复检。
type ReplaceDirOptions struct {
	Mode     os.FileMode
	Validate func() error
}

type directoryTransaction struct {
	Target    string `json:"target"`
	Staging   string `json:"staging"`
	Backup    string `json:"backup"`
	HadTarget bool   `json:"had_target"`
}

// ReplaceDir 在同级临时目录完成构建，再替换目标目录；发布失败时恢复旧目录。
func ReplaceDir(target string, build func(staging string) error) error {
	return ReplaceDirWithOptions(target, ReplaceDirOptions{Mode: 0o755}, build)
}

// ReplaceDirWithOptions 使用目标锁和持久化事务记录执行目录替换。
func ReplaceDirWithOptions(target string, opts ReplaceDirOptions, build func(staging string) error) error {
	if build == nil {
		return fmt.Errorf("%s", i18n.Get("DirectoryBuilderMissing"))
	}
	if opts.Mode.Perm() == 0 {
		return fmt.Errorf("%s", i18n.Get("DirectoryModeRequired"))
	}
	target, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return err
	}
	parent, transactionFile, unlock, err := prepareDirectoryReplacement(target, opts.Validate)
	if err != nil {
		return err
	}
	defer unlock()

	backup := backupPath(target)
	if err := ensureBackupPathAvailable(backup); err != nil {
		return err
	}
	targetExists, err := directoryExists(target)
	if err != nil {
		return err
	}

	staging, err := os.MkdirTemp(parent, "."+filepath.Base(target)+"-staging-*")
	if err != nil {
		return err
	}
	tx := directoryTransaction{
		Target:    target,
		Staging:   staging,
		Backup:    backup,
		HadTarget: targetExists,
	}
	if err := saveDirectoryTransaction(transactionFile, tx); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	cleanupBeforePublish := true
	defer func() {
		if cleanupBeforePublish {
			_ = os.RemoveAll(staging)
			_ = os.Remove(transactionFile)
		}
	}()

	if err := os.Chmod(staging, opts.Mode.Perm()); err != nil {
		return err
	}
	if err := build(staging); err != nil {
		return err
	}
	if err := validateDirectoryTarget(opts.Validate); err != nil {
		return err
	}

	cleanupBeforePublish = false
	if err := publishDirectoryTransaction(tx); err != nil {
		return err
	}
	if err := removeCommittedDirectoryTransaction(transactionFile); err != nil {
		return err
	}
	return nil
}

func prepareDirectoryReplacement(target string, validate func() error) (string, string, func(), error) {
	parent := filepath.Dir(target)
	if err := validateDirectoryTarget(validate); err != nil {
		return "", "", func() {}, err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", func() {}, err
	}
	lockFile, err := lockPath(target)
	if err != nil {
		return "", "", func() {}, err
	}
	if err := os.MkdirAll(filepath.Dir(lockFile), 0o700); err != nil {
		return "", "", func() {}, err
	}
	lock := flock.New(lockFile)
	if err := lock.Lock(); err != nil {
		return "", "", func() {}, fmt.Errorf("%s: %w", i18n.Get("DirectoryLockTargetFailed"), err)
	}
	unlock := func() { _ = lock.Unlock() }
	transactionFile := lockFile + ".transaction.json"
	if err := validateDirectoryTarget(validate); err != nil {
		unlock()
		return "", "", func() {}, err
	}
	if err := recoverDirectoryTransaction(target, transactionFile); err != nil {
		unlock()
		return "", "", func() {}, err
	}
	return parent, transactionFile, unlock, nil
}

func ensureBackupPathAvailable(backup string) error {
	if _, err := os.Lstat(backup); err == nil {
		return fmt.Errorf("%s", i18n.GetWithParams("DirectoryBackupPathExists", map[string]interface{}{"Path": backup}))
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func directoryExists(path string) (bool, error) {
	if _, err := os.Lstat(path); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	return false, nil
}

func publishDirectoryTransaction(tx directoryTransaction) error {
	if tx.HadTarget {
		if err := os.Rename(tx.Target, tx.Backup); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("DirectoryBackupFailed"), err)
		}
	}
	if err := os.Rename(tx.Staging, tx.Target); err != nil {
		return rollbackDirectoryPublish(tx, err)
	}
	if tx.HadTarget {
		if err := os.RemoveAll(tx.Backup); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("DirectoryRemoveBackupFailed"), err)
		}
	}
	return nil
}

func rollbackDirectoryPublish(tx directoryTransaction, publishErr error) error {
	if !tx.HadTarget {
		return fmt.Errorf("%s: %w", i18n.Get("DirectoryPublishFailed"), publishErr)
	}
	if rollbackErr := os.Rename(tx.Backup, tx.Target); rollbackErr != nil {
		return errors.Join(
			fmt.Errorf("%s: %w", i18n.Get("DirectoryPublishFailed"), publishErr),
			fmt.Errorf("%s: %w", i18n.Get("DirectoryRestoreFailed"), rollbackErr),
		)
	}
	return fmt.Errorf("%s: %w", i18n.Get("DirectoryPublishFailed"), publishErr)
}

func removeCommittedDirectoryTransaction(transactionFile string) error {
	if err := os.Remove(transactionFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", i18n.Get("DirectoryRemoveTransactionFailed"), err)
	}
	return nil
}

func recoverDirectoryTransaction(target, transactionFile string) error {
	tx, err := loadDirectoryTransaction(transactionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := validateDirectoryTransaction(target, tx); err != nil {
		return err
	}
	_, targetErr := os.Lstat(target)
	_, backupErr := os.Lstat(tx.Backup)
	if targetErr != nil && !os.IsNotExist(targetErr) {
		return targetErr
	}
	if backupErr != nil && !os.IsNotExist(backupErr) {
		return backupErr
	}
	if tx.HadTarget {
		switch {
		case os.IsNotExist(targetErr) && backupErr == nil:
			if err := os.Rename(tx.Backup, target); err != nil {
				return fmt.Errorf("%s: %w", i18n.Get("DirectoryRestoreInterruptedFailed"), err)
			}
		case targetErr == nil && backupErr == nil:
			if err := os.RemoveAll(tx.Backup); err != nil {
				return fmt.Errorf("%s: %w", i18n.Get("DirectoryRemoveCommittedBackupFailed"), err)
			}
		case os.IsNotExist(targetErr) && os.IsNotExist(backupErr):
			return fmt.Errorf("%s", i18n.GetWithParams("DirectoryTransactionLostTarget", map[string]interface{}{"Path": target}))
		}
	}
	if err := os.RemoveAll(tx.Staging); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("DirectoryRemoveStagingFailed"), err)
	}
	if err := os.Remove(transactionFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", i18n.Get("DirectoryRemoveRecoveredTransactionFailed"), err)
	}
	return nil
}

func saveDirectoryTransaction(path string, tx directoryTransaction) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data, 0o600)
}

func loadDirectoryTransaction(path string) (directoryTransaction, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return directoryTransaction{}, err
	}
	var tx directoryTransaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return directoryTransaction{}, fmt.Errorf("%s: %w", i18n.Get("DirectoryParseTransactionFailed"), err)
	}
	return tx, nil
}

func validateDirectoryTransaction(target string, tx directoryTransaction) error {
	if tx.Target != target || tx.Backup != backupPath(target) {
		return fmt.Errorf("%s", i18n.GetWithParams("DirectoryTransactionTargetMismatch", map[string]interface{}{"Path": target}))
	}
	stagingParent := filepath.Dir(tx.Staging)
	stagingPrefix := "." + filepath.Base(target) + "-staging-"
	if stagingParent != filepath.Dir(target) || !strings.HasPrefix(filepath.Base(tx.Staging), stagingPrefix) {
		return fmt.Errorf("%s", i18n.GetWithParams("DirectoryInvalidStagingPath", map[string]interface{}{"Path": tx.Staging}))
	}
	return nil
}

func validateDirectoryTarget(validate func() error) error {
	if validate == nil {
		return nil
	}
	if err := validate(); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("DirectoryValidateTargetFailed"), err)
	}
	return nil
}

func lockPath(target string) (string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if resolvedParent, resolveErr := filepath.EvalSymlinks(filepath.Dir(absTarget)); resolveErr == nil {
		absTarget = filepath.Join(resolvedParent, filepath.Base(absTarget))
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(filepath.Clean(absTarget)))
	return filepath.Join(cacheDir, "skills-seed", "locks", hex.EncodeToString(sum[:])+".lock"), nil
}

func backupPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".backup")
}
