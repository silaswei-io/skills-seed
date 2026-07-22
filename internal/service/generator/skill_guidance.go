package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/validation"
)

type validationCommand = validation.Command

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

func validationCommands(profile *domain.ProjectProfile) []validationCommand {
	return validation.Commands(profile)
}

func validationGaps(profile *domain.ProjectProfile, matrix []ValidationMatrixItem, locale string) []string {
	commands := validationCommands(profile)
	hasTest := false
	hasStaticCheck := false
	for _, command := range commands {
		switch validation.Kind(command) {
		case domain.ValidationCommandTest:
			hasTest = true
		case domain.ValidationCommandStaticCheck:
			hasStaticCheck = true
		}
	}

	gaps := make([]string, 0, 3)
	if !hasTest {
		gaps = append(gaps, generatorText(locale, "GeneratorValidationGapTest"))
	}
	if !hasStaticCheck {
		gaps = append(gaps, generatorText(locale, "GeneratorValidationGapStaticCheck"))
	}
	if len(matrix) == 0 {
		gaps = append(gaps, generatorText(locale, "GeneratorValidationGapScopedMatrix"))
	}
	return gaps
}
