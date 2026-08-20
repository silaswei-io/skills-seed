package runtimeclean

import (
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

// Path 返回 .skills-seed 下的 runtime 根目录。
func Path(seedPath string) string {
	return layout.New(seedPath).Runtime()
}

// Exists 报告 runtime 根目录是否存在。
func Exists(seedPath string) (bool, error) {
	info, err := os.Stat(Path(seedPath))
	if err == nil {
		return info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Clear 删除整个 runtime 根目录。
func Clear(seedPath string) error {
	return os.RemoveAll(filepath.Clean(Path(seedPath)))
}
