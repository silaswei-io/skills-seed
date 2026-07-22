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
	Area     string
	Command  string
	When     string
	Source   string
	Evidence []string
}

func Kind(command Command) domain.ValidationCommandKind {
	return domain.ClassifyValidationCommand(domain.ValidationCommand{
		Command: command.Command,
		When:    command.When,
		Type:    command.Type,
	})
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
		areaEvidence := limitStrings(area.Evidence, 3)
		commandArea := area
		commandArea.Evidence = areaEvidence
		choice := selector.Choose(commandArea)
		if choice.Command.Command == "" {
			continue
		}
		matrix = append(matrix, Recommendation{
			Area:     area.Name,
			Command:  choice.Command.Command,
			When:     firstNonEmptyString(choice.Command.When, area.When),
			Source:   choice.Command.Source,
			Evidence: commandEvidence(choice.Command),
		})
	}
	return matrix
}

// commandEvidence 只返回命令自身携带的来源和作用域声明，避免把业务源码误写成命令证据。
func commandEvidence(command Command) []string {
	evidence := make([]string, 0, len(command.Evidence)+len(command.ScopePaths)+2)
	evidence = append(evidence, command.Evidence...)
	if command.Source != "" {
		evidence = append(evidence, command.Source)
	}
	evidence = append(evidence, command.ScopePaths...)
	if command.Workdir != "" {
		evidence = append(evidence, command.Workdir)
	}
	evidence = append(evidence, CommandPaths(command.Command)...)
	return limitStrings(uniqueStrings(evidence), 3)
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
