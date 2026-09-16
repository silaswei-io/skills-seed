package config

import (
	"fmt"
	"regexp"

	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// skillsNamePattern 限制名称为小写字母、数字和单个连字符组成的安全目录名。
var skillsNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSkillsName 校验显式 Skill 名称；空值表示使用项目默认名。
func ValidateSkillsName(name string) error {
	if name == "" {
		return nil
	}
	if len(name) > 64 || !skillsNamePattern.MatchString(name) {
		return fmt.Errorf("skills.name: %s", i18n.Get("SkillsNameInvalid"))
	}
	return nil
}
