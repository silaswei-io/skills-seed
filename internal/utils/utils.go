package utils

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"gopkg.in/yaml.v3"
)

// GetSeedPath 获取 .skills-seed 目录路径
func GetSeedPath() (string, error) {
	// 从当前目录开始向上查找
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}

	// 检查当前目录是否有 .skills-seed
	seedPath := filepath.Join(currentDir, ".skills-seed")
	if _, err := os.Stat(seedPath); err == nil {
		return seedPath, nil
	}

	// 向上查找父目录
	parentDir := filepath.Dir(currentDir)
	for {
		seedPath = filepath.Join(parentDir, ".skills-seed")
		if _, err := os.Stat(seedPath); err == nil {
			return seedPath, nil
		}

		// 检查是否到达根目录
		if parentDir == "/" || parentDir == currentDir {
			break
		}

		parentDir = filepath.Dir(parentDir)
	}

	return "", fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
}

// LoadConfig 加载配置文件（不创建 Container）
func LoadConfig(seedPath string) (*config.Config, error) {
	configPath := filepath.Join(seedPath, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("UtilsReadConfigFailed"), err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("UtilsParseConfigFailed"), err)
	}

	return &cfg, nil
}

// ResolvePath resolves output/config paths relative to project root and expands "~"
func ResolvePath(projectRoot, path string) (string, error) {
	if path == "" {
		return projectRoot, nil
	}

	if strings.HasPrefix(path, "~/") || path == "~" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
		}
		if path == "~" {
			return usr.HomeDir, nil
		}
		return filepath.Join(usr.HomeDir, strings.TrimPrefix(path, "~/")), nil
	}

	if filepath.IsAbs(path) {
		return path, nil
	}

	return filepath.Join(projectRoot, path), nil
}

// RelativePaths returns slash-separated paths relative to projectRoot.
func RelativePaths(projectRoot string, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	relPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			continue
		}
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}
	return relPaths
}
