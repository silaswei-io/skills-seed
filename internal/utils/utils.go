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

	pathAbs, err := ResolvePathWithinRoot(projectRoot, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("%s", i18n.GetWithParams("GenerateOutputPathOutsideProjectRoot", map[string]interface{}{
			"OutputPath":  outputPath,
			"ProjectRoot": projectRoot,
		}))
	}
	rootAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	if filepath.Clean(pathAbs) == filepath.Clean(rootAbs) {
		return "", fmt.Errorf("%s", i18n.GetWithParams("GenerateOutputPathOutsideProjectRoot", map[string]interface{}{
			"OutputPath":  outputPath,
			"ProjectRoot": projectRoot,
		}))
	}
	return pathAbs, nil
}

// ResolvePathWithinRoot 解析现有符号链接，并确保目标的真实路径位于根目录内。
// 目标尚不存在时，从最近存在的父目录开始解析，避免中间符号链接绕过边界。
func ResolvePathWithinRoot(root, target string) (string, error) {
	_, err := CanonicalPathWithinRoot(root, target)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Clean(targetAbs), nil
}

// CanonicalPathWithinRoot 返回解析符号链接后的安全目标路径。
func CanonicalPathWithinRoot(root, target string) (string, error) {
	rootPath, err := resolvePathFromExistingAncestor(root)
	if err != nil {
		return "", err
	}
	targetPath, err := resolvePathFromExistingAncestor(target)
	if err != nil {
		return "", err
	}
	relPath, err := filepath.Rel(rootPath, targetPath)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("path %q resolves outside root %q", target, root)
	}
	return targetPath, nil
}

func resolvePathFromExistingAncestor(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absPath = filepath.Clean(absPath)
	current := absPath
	missingParts := make([]string, 0)
	for {
		if _, err := os.Lstat(current); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing ancestor for path %q", path)
		}
		missingParts = append(missingParts, filepath.Base(current))
		current = parent
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		return "", err
	}
	for i := len(missingParts) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missingParts[i])
	}
	return filepath.Clean(resolved), nil
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
