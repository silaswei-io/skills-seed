package skilloutput

import (
	"fmt"
	"os"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

// Replace 在 staging 目录完整生成 Skill，并事务性替换现有输出。
func Replace(outputPath string, build func(staging string) error) error {
	return fileio.ReplaceDir(outputPath, build)
}

// ReplaceWithinRoot 在构建前和发布前重新校验项目输出边界。
func ReplaceWithinRoot(projectRoot, outputPath string, build func(staging string) error) error {
	if strings.TrimSpace(projectRoot) == "" {
		return Replace(outputPath, build)
	}
	if _, err := utils.ResolveProjectOutputPath(projectRoot, outputPath); err != nil {
		return err
	}
	canonicalTarget, err := utils.CanonicalPathWithinRoot(projectRoot, outputPath)
	if err != nil {
		return err
	}
	validate := func() error {
		current, err := utils.CanonicalPathWithinRoot(projectRoot, outputPath)
		if err != nil {
			return err
		}
		if current != canonicalTarget {
			return fmt.Errorf("%s", i18n.Get("SkillOutputPathChanged"))
		}
		return nil
	}
	return fileio.ReplaceDirWithOptions(canonicalTarget, fileio.ReplaceDirOptions{Mode: 0o755, Validate: validate}, build)
}

// Remove 删除由配置指定的 skills-seed 输出目录。
func Remove(outputPath string) error {
	return os.RemoveAll(outputPath)
}
