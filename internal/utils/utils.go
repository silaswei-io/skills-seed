package utils

import (
	"fmt"
	"os"
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

	if seedPath, ok := findSeedPathFrom(currentDir, pathExists, filepath.Dir); ok {
		return seedPath, nil
	}

	return "", fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
}

func findSeedPathFrom(currentDir string, exists func(string) bool, parentOf func(string) string) (string, bool) {
	dir := currentDir
	for {
		seedPath := filepath.Join(dir, ".skills-seed")
		if exists(seedPath) {
			return seedPath, true
		}

		parentDir := parentOf(dir)
		if parentDir == dir {
			break
		}

		dir = parentDir
	}

	return "", false
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

// ResolvePath 按项目根目录解析输出/配置路径，并展开 "~"。
func ResolvePath(projectRoot, path string) (string, error) {
	if path == "" {
		return projectRoot, nil
	}

	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) || path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
		}
		if path == "~" {
			return homeDir, nil
		}
		return filepath.Join(homeDir, strings.TrimPrefix(strings.TrimPrefix(path, "~/"), `~\`)), nil
	}

	if filepath.IsAbs(path) {
		return path, nil
	}

	return filepath.Join(projectRoot, path), nil
}

// ResolveProjectOutputPath 解析项目输出路径，并确保输出目录不逃出项目根目录。
func ResolveProjectOutputPath(projectRoot, outputPath string) (string, error) {
	resolvedPath, err := ResolvePath(projectRoot, outputPath)
	if err != nil {
		return "", err
	}
	if projectRoot == "" {
		return filepath.Clean(resolvedPath), nil
	}

	rootAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", err
	}
	rootAbs = filepath.Clean(rootAbs)
	pathAbs = filepath.Clean(pathAbs)
	relPath, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%s", i18n.GetWithParams("GenerateOutputPathOutsideProjectRoot", map[string]interface{}{
			"OutputPath":  outputPath,
			"ProjectRoot": projectRoot,
		}))
	}
	return pathAbs, nil
}

// ConfiguredSkillOutputPath 根据 skills target 配置解析最终输出路径。
func ConfiguredSkillOutputPath(projectRoot string, configRepo config.Reader) (string, error) {
	target := ""
	outputPath := ""
	if configRepo != nil {
		target = configRepo.GetEffectiveSkillsTarget()
		outputPath = configRepo.GetEffectiveSkillsPath()
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = config.DefaultSkillsPathForTarget(target)
	}
	return ResolveProjectOutputPath(projectRoot, outputPath)
}

// RelativePaths 返回相对于 projectRoot 的斜杠分隔路径。
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
