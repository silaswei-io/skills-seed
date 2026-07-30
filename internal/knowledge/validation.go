package knowledge

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

type ValidationAreaKind string

const (
	ValidationAreaAPI         ValidationAreaKind = "api"
	ValidationAreaBusiness    ValidationAreaKind = "business"
	ValidationAreaPersistence ValidationAreaKind = "persistence"
	ValidationAreaRuntime     ValidationAreaKind = "runtime"
)

type ValidationCommand struct {
	Command    string
	When       string
	Source     string
	Workdir    string
	ScopePaths []string
	Evidence   []string
	Type       string
}

type ValidationArea struct {
	Name     string
	Needles  []string
	When     string
	Evidence []string
	Kind     ValidationAreaKind
}

type ValidationRecommendation struct {
	Area     string
	Command  string
	When     string
	Source   string
	Evidence []string
}

func ValidationCommandKind(command ValidationCommand) domain.ValidationCommandKind {
	return domain.ClassifyValidationCommand(domain.ValidationCommand{
		Command: command.Command,
		When:    command.When,
		Type:    command.Type,
	})
}

func ValidationCommands(profile *domain.ProjectProfile) []ValidationCommand {
	if profile == nil {
		return nil
	}
	learned := domain.CleanValidationCommands(profile.ValidationCommands)
	if len(learned) == 0 {
		return nil
	}
	commands := make([]ValidationCommand, 0, len(learned))
	for _, learnedCommand := range learned {
		commands = append(commands, ValidationCommand{
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

func ValidationMatrix(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []ValidationRecommendation {
	commands := ValidationCommands(profile)
	if len(commands) == 0 {
		return nil
	}

	selector := commandSelector{commands: commands}
	areas := ValidationAreas(profile, patterns, locale)
	matrix := make([]ValidationRecommendation, 0, len(areas))
	for _, area := range areas {
		areaEvidence := limitStrings(area.Evidence, 3)
		commandArea := area
		commandArea.Evidence = areaEvidence
		choice := selector.Choose(commandArea)
		if choice.Command.Command == "" {
			continue
		}
		matrix = append(matrix, ValidationRecommendation{
			Area:     area.Name,
			Command:  choice.Command.Command,
			When:     stringx.FirstNonBlank(choice.Command.When, area.When),
			Source:   choice.Command.Source,
			Evidence: commandEvidence(choice.Command),
		})
	}
	return matrix
}

// commandEvidence 只返回命令自身携带的来源和作用域声明，避免把业务源码误写成命令证据。
func commandEvidence(command ValidationCommand) []string {
	evidence := make([]string, 0, len(command.Evidence)+len(command.ScopePaths)+2)
	evidence = append(evidence, command.Evidence...)
	if command.Source != "" {
		evidence = append(evidence, command.Source)
	}
	evidence = append(evidence, command.ScopePaths...)
	if command.Workdir != "" {
		evidence = append(evidence, command.Workdir)
	}
	evidence = append(evidence, ValidationCommandPaths(command.Command)...)
	return limitStrings(stringx.UniqueNonBlank(evidence), 3)
}

func ValidationAreas(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []ValidationArea {
	areas := []ValidationArea{
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaAPIName"),
			Needles: []string{"api", "interface", "contract", "schema", "message", "event", "adapter", "route", "handler", "component", "generate", "generated", "proto", "swagger", "openapi", "接口", "契约", "消息", "事件", "适配", "路由", "组件", "生成"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaAPIWhen"),
			Kind:    ValidationAreaAPI,
		},
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaBusinessName"),
			Needles: []string{"business", "domain", "product", "workflow", "flow", "state", "action", "policy", "permission", "orchestr", "capability", "service", "业务", "领域", "产品", "流程", "状态", "动作", "策略", "权限", "编排", "能力"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaBusinessWhen"),
			Kind:    ValidationAreaBusiness,
		},
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaPersistenceName"),
			Needles: []string{"db", "database", "state", "storage", "store", "repo", "model", "cache", "index", "migrate", "sql", "query", "transaction", "数据库", "状态", "存储", "持久化", "缓存", "索引", "查询", "事务", "迁移"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaPersistenceWhen"),
			Kind:    ValidationAreaPersistence,
		},
		{
			Name:    i18n.GetForLocale(locale, "KnowledgeValidationAreaRuntimeName"),
			Needles: []string{"config", "middleware", "pipeline", "interceptor", "filter", "hook", "runtime", "server", "bootstrap", "startup", "plugin", "extension", "deploy", "配置", "中间件", "管道", "拦截", "过滤", "运行", "启动", "插件", "扩展", "部署"},
			When:    i18n.GetForLocale(locale, "KnowledgeValidationAreaRuntimeWhen"),
			Kind:    ValidationAreaRuntime,
		},
	}
	for i := range areas {
		areas[i].Evidence = areaEvidence(profile, patterns, areas[i].Needles)
	}
	result := make([]ValidationArea, 0, len(areas))
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
		return stringx.UniqueNonBlank(patternEvidence)
	}

	evidence := make([]string, 0)
	if profile != nil {
		for _, module := range profile.KeyModules {
			text := strings.ToLower(module.Name + " " + module.Path + " " + module.Description + " " + strings.Join(module.Responsibilities, " ") + " " + strings.Join(module.KeyMethods, " "))
			if containsAny(text, needles...) {
				evidence = append(evidence, stringx.FirstNonBlank(module.Path, module.Name))
			}
		}
	}
	return stringx.UniqueNonBlank(evidence)
}
