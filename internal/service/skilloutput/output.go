package skilloutput

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const generatedMarker = "generated-by: skills-seed"

// ManualSkillExistsError 表示目标目录已有非 skills-seed 生成的 SKILL.md。
type ManualSkillExistsError struct {
	Path string
}

func (e *ManualSkillExistsError) Error() string {
	return "manual skill exists: " + e.Path
}

// EnsureWritable 确认输出目录可以由 skills-seed 覆盖。
func EnsureWritable(outputPath string) error {
	skillPath := filepath.Join(outputPath, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if strings.Contains(string(content), generatedMarker) {
		return nil
	}
	return &ManualSkillExistsError{Path: skillPath}
}

// Rebuild 删除并重建 skills-seed 生成的输出目录。
func Rebuild(outputPath string) error {
	if err := EnsureWritable(outputPath); err != nil {
		return err
	}
	if err := os.RemoveAll(outputPath); err != nil {
		return err
	}
	return os.MkdirAll(outputPath, 0755)
}
