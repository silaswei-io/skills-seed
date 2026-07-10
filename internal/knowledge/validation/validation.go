package validation

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

type AreaKind string

const (
	AreaAPI         AreaKind = "api"
	AreaBusiness    AreaKind = "business"
	AreaPersistence AreaKind = "persistence"
	AreaRuntime     AreaKind = "runtime"
)

type MatchKind string

const (
	MatchScoped   MatchKind = "scoped"
	MatchSemantic MatchKind = "semantic"
	MatchGeneric  MatchKind = "generic"
	MatchBroad    MatchKind = "broad"
)

type Command struct {
	Command    string
	When       string
	Source     string
	Workdir    string
	ScopePaths []string
	Evidence   []string
	Type       string
}

type Area struct {
	Name     string
	Needles  []string
	When     string
	Evidence []string
	Kind     AreaKind
}

type Recommendation struct {
	Area        string
	Command     string
	When        string
	Source      string
	Evidence    []string
	Confidence  float64
	Coverage    float64
	MatchKind   string
	Recommended bool
	Warning     string
}

func Commands(profile *domain.ProjectProfile) []Command {
	if profile == nil {
		return nil
	}
	learned := domain.CleanValidationCommands(profile.ValidationCommands)
	if len(learned) == 0 {
		return nil
	}
	commands := make([]Command, 0, len(learned))
	for _, learnedCommand := range learned {
		commands = append(commands, Command{
			Command:    learnedCommand.Command,
			When:       learnedCommand.When,
			Source:     learnedCommand.Source,
			Workdir:    learnedCommand.Workdir,
			ScopePaths: append([]string(nil), learnedCommand.ScopePaths...),
			Evidence:   append([]string(nil), learnedCommand.Evidence...),
			Type:       learnedCommand.Type,
		})
	}
	return commands
}

func Matrix(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []Recommendation {
	commands := Commands(profile)
	if len(commands) == 0 {
		return nil
	}

	selector := commandSelector{commands: commands}
	areas := Areas(profile, patterns, locale)
	matrix := make([]Recommendation, 0, len(areas))
	for _, area := range areas {
		displayEvidence := limitStrings(area.Evidence, 3)
		commandArea := area
		commandArea.Evidence = displayEvidence
		choice := selector.Choose(commandArea)
		if choice.Command.Command == "" {
			continue
		}
		when, warning := matrixWhen(choice, area, locale)
		if choice.Match != MatchScoped && choice.Match != MatchSemantic {
			continue
		}
		matrix = append(matrix, Recommendation{
			Area:        area.Name,
			Command:     choice.Command.Command,
			When:        when,
			Source:      choice.Command.Source,
			Evidence:    displayEvidence,
			Confidence:  choice.Confidence,
			Coverage:    choice.Coverage,
			MatchKind:   string(choice.Match),
			Recommended: choice.Match == MatchScoped || choice.Match == MatchSemantic,
			Warning:     warning,
		})
	}
	return matrix
}

func Areas(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []Area {
	areas := []Area{
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaAPIName"),
			Needles: []string{"api", "contract", "route", "handler", "generate", "generated", "proto", "swagger", "接口", "契约", "路由", "生成"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaAPIWhen"),
			Kind:    AreaAPI,
		},
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaBusinessName"),
			Needles: []string{"business", "domain", "workflow", "state", "orchestr", "service", "业务", "领域", "流程", "状态", "编排"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaBusinessWhen"),
			Kind:    AreaBusiness,
		},
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaPersistenceName"),
			Needles: []string{"db", "database", "store", "repo", "model", "migrate", "sql", "query", "数据库", "持久化", "查询", "迁移"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaPersistenceWhen"),
			Kind:    AreaPersistence,
		},
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaRuntimeName"),
			Needles: []string{"config", "middleware", "server", "bootstrap", "startup", "plugin", "配置", "中间件", "启动", "插件"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaRuntimeWhen"),
			Kind:    AreaRuntime,
		},
	}
	for i := range areas {
		areas[i].Evidence = areaEvidence(profile, patterns, areas[i].Needles)
	}
	result := make([]Area, 0, len(areas))
	for _, area := range areas {
		if len(area.Evidence) > 0 {
			result = append(result, area)
		}
	}
	if len(result) == 0 && len(areas) > 0 {
		result = append(result, areas[0])
	}
	return result
}

func areaEvidence(profile *domain.ProjectProfile, patterns []domain.Pattern, needles []string) []string {
	patternEvidence := make([]string, 0)
	for _, pattern := range patterns {
		text := strings.ToLower(pattern.Name + " " + string(pattern.Category) + " " + pattern.Description + " " + pattern.Rule + " " + pattern.ScopePath)
		if !containsAny(text, needles...) {
			continue
		}
		for _, location := range pattern.EvidenceLocations {
			if location.DisplayLocation() != "" {
				patternEvidence = append(patternEvidence, location.DisplayLocation())
			}
		}
		if pattern.BusinessMethod != nil && pattern.BusinessMethod.DisplayLocation() != "" {
			patternEvidence = append(patternEvidence, pattern.BusinessMethod.DisplayLocation())
		}
		if pattern.ScopePath != "" {
			patternEvidence = append(patternEvidence, pattern.ScopePath)
		}
	}
	if len(patternEvidence) > 0 {
		return uniqueStrings(patternEvidence)
	}

	evidence := make([]string, 0)
	if profile != nil {
		for _, module := range profile.KeyModules {
			text := strings.ToLower(module.Name + " " + module.Path + " " + module.Description + " " + strings.Join(module.Responsibilities, " ") + " " + strings.Join(module.KeyMethods, " "))
			if containsAny(text, needles...) {
				evidence = append(evidence, firstNonEmptyString(module.Path, module.Name))
			}
		}
	}
	return uniqueStrings(evidence)
}

func matrixWhen(choice commandChoice, area Area, locale string) (string, string) {
	base := firstNonEmptyString(choice.Command.When, area.When)
	warning := ""
	switch choice.Match {
	case MatchGeneric:
		warning = fallbackText(locale, "generic")
	case MatchBroad:
		warning = fallbackText(locale, "broad")
	}
	if warning == "" {
		return base, ""
	}
	if base == "" {
		return warning, warning
	}
	return warning + " " + base, warning
}

func fallbackText(locale string, kind string) string {
	if kind == "broad" {
		return i18n.GetForLocale(locale, "KnowledgeValidationFallbackBroad")
	}
	return i18n.GetForLocale(locale, "KnowledgeValidationFallbackGeneric")
}
