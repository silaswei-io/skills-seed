package codegraph

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// Version 固定官方 SDK 和运行时版本，升级时必须验证公开接口兼容性。
const Version = "1.6.2"

// IndexDir 使用 CodeGraph 的标准本地索引目录。
const IndexDir = ".codegraph"

//go:embed assets/* bridge.cjs
var runtimeFiles embed.FS

// runtimeHome 返回官方平台包；源码开发可显式指定已安装的平台包目录。
func runtimeHome(ctx context.Context) (string, error) {
	if home := os.Getenv("SKILLS_SEED_CODEGRAPH_HOME"); home != "" {
		return filepath.Abs(home)
	}
	payload, err := runtimeFiles.ReadFile("assets/runtime.zip")
	if err != nil {
		return "", fmt.Errorf("%w: build with make bundle or set SKILLS_SEED_CODEGRAPH_HOME to the CodeGraph %s platform package", ErrUnavailable, Version)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	parent := filepath.Join(cache, "skills-seed", "codegraph", Version)
	target := filepath.Join(parent, runtime.GOOS+"-"+runtime.GOARCH+"-"+digest[:16])
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	lock := flock.New(target + ".lock")
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return "", err
	}
	if !locked {
		return "", ctx.Err()
	}
	defer lock.Unlock()
	if _, err := os.Stat(filepath.Join(target, ".complete")); err == nil {
		return target, nil
	}
	staging, err := os.MkdirTemp(parent, ".extract-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	if err := extractRuntime(ctx, payload, staging); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(staging, ".complete"), []byte(digest), 0600); err != nil {
		return "", err
	}
	if err := os.Rename(staging, target); err != nil {
		return "", err
	}
	return target, nil
}

func extractRuntime(ctx context.Context, payload []byte, target string) error {
	archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}
	for _, entry := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if name == "" {
			continue
		}
		if !filepath.IsLocal(name) || strings.Contains(name, "\\") || entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe CodeGraph archive entry: %s", entry.Name)
		}
		path := filepath.Join(target, filepath.FromSlash(name))
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := extractRuntimeFile(entry, path); err != nil {
			return err
		}
	}
	return nil
}

func extractRuntimeFile(entry *zip.File, path string) error {
	input, err := entry.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm()|0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// ErrUnavailable 表示构建产物缺少内嵌运行时。
var ErrUnavailable = errors.New("embedded CodeGraph runtime unavailable")
