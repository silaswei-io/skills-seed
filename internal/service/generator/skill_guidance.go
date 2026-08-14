package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func skillTriggerDescription(projectName, language, locale string, profile *domain.ProjectProfile) string {
	project := strings.TrimSpace(projectName)
	if profile != nil && strings.TrimSpace(profile.ProjectName) != "" {
		project = strings.TrimSpace(profile.ProjectName)
	}
	if project == "" {
		project = generatorText(locale, "GeneratorDefaultProjectName")
	}
	lang := strings.TrimSpace(language)
	if profile != nil && strings.TrimSpace(profile.Language) != "" {
		lang = strings.TrimSpace(profile.Language)
	}
	if lang == "" {
		lang = generatorText(locale, "GeneratorDefaultLanguageName")
	}
	return generatorTextWithParams(locale, "GeneratorSkillDescriptionDefault", map[string]interface{}{
		"Project":  project,
		"Language": lang,
	})
}
