package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type validationArea struct {
	Name     string
	Needles  []string
	When     string
	Evidence []string
}

type validationCommandMatch string

const (
	validationCommandMatchScoped   validationCommandMatch = "scoped"
	validationCommandMatchSemantic validationCommandMatch = "semantic"
	validationCommandMatchGeneric  validationCommandMatch = "generic"
	validationCommandMatchBroad    validationCommandMatch = "broad"
)

type validationCommandChoice struct {
	Command validationCommand
	Match   validationCommandMatch
}

type validationFallbackKind string

const (
	validationFallbackGeneric validationFallbackKind = "generic"
	validationFallbackBroad   validationFallbackKind = "broad"
)

func validationMatrix(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []ValidationMatrixItem {
	commands := validationCommands(profile, patterns, locale)
	if len(commands) == 0 {
		return nil
	}

	selector := validationCommandSelector{commands: commands}
	areas := validationAreas(profile, patterns, locale)
	matrix := make([]ValidationMatrixItem, 0, len(areas))
	for _, area := range areas {
		displayEvidence := limitStrings(area.Evidence, 3)
		commandArea := area
		commandArea.Evidence = displayEvidence
		choice := selector.Choose(commandArea)
		if choice.Command.Command == "" {
			continue
		}
		matrix = append(matrix, ValidationMatrixItem{
			Area:     area.Name,
			Command:  choice.Command.Command,
			When:     validationMatrixWhen(choice, area, locale),
			Source:   choice.Command.Source,
			Evidence: displayEvidence,
		})
	}
	return matrix
}

func validationAreas(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []validationArea {
	areas := []validationArea{
		{
			Name:    localizedText(locale, "接口 / 契约 / 生成链路", "API / Contract / Generation Chain"),
			Needles: []string{"api", "contract", "route", "handler", "generate", "generated", "proto", "swagger", "接口", "契约", "路由", "生成"},
			When:    localizedText(locale, "接口、路由、请求响应类型、生成物或适配层变化后运行。", "Run after changing APIs, routes, request/response types, generated artifacts, or adapters."),
		},
		{
			Name:    localizedText(locale, "业务流程 / 状态 / 编排", "Business Flow / State / Orchestration"),
			Needles: []string{"business", "domain", "workflow", "state", "orchestr", "service", "业务", "领域", "流程", "状态", "编排"},
			When:    localizedText(locale, "业务规则、状态流转、跨模块编排或幂等逻辑变化后运行。", "Run after changing business rules, state transitions, cross-module orchestration, or idempotency logic."),
		},
		{
			Name:    localizedText(locale, "持久化 / 查询 / 迁移", "Persistence / Query / Migration"),
			Needles: []string{"db", "database", "store", "repo", "model", "migrate", "sql", "query", "数据库", "持久化", "查询", "迁移"},
			When:    localizedText(locale, "模型、查询、事务、迁移或缓存持久化边界变化后运行。", "Run after changing models, queries, transactions, migrations, or persistence/cache boundaries."),
		},
		{
			Name:    localizedText(locale, "配置 / 中间件 / 启动链路", "Config / Middleware / Startup"),
			Needles: []string{"config", "middleware", "server", "bootstrap", "startup", "plugin", "配置", "中间件", "启动", "插件"},
			When:    localizedText(locale, "配置结构、中间件注册、启动参数或插件装配变化后运行。", "Run after changing config structs, middleware registration, startup parameters, or plugin wiring."),
		},
	}
	for i := range areas {
		areas[i].Evidence = validationAreaEvidence(profile, patterns, areas[i].Needles)
	}
	result := make([]validationArea, 0, len(areas))
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

func validationAreaEvidence(profile *domain.ProjectProfile, patterns []domain.Pattern, needles []string) []string {
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

func validationMatrixWhen(choice validationCommandChoice, area validationArea, locale string) string {
	base := firstNonEmptyString(choice.Command.When, area.When)
	switch choice.Match {
	case validationCommandMatchGeneric:
		return validationFallbackText(locale, validationFallbackGeneric, base)
	case validationCommandMatchBroad:
		return validationFallbackText(locale, validationFallbackBroad, base)
	default:
		return base
	}
}

func validationFallbackText(locale string, kind validationFallbackKind, base string) string {
	prefix := ""
	if kind == validationFallbackBroad {
		prefix = localizedText(locale,
			"未找到覆盖该范围的专用验证命令；此命令覆盖范围较宽，仅作为兜底验证，不能替代专项测试。",
			"No scope-specific validation command was found; this broad command is only fallback validation and does not replace scope-specific tests.",
		)
	} else {
		prefix = localizedText(locale,
			"未找到覆盖该范围的专用验证命令；先运行通用检查，但不能替代该范围的专项测试。",
			"No scope-specific validation command was found; run this general check first, but it does not replace scope-specific tests.",
		)
	}
	if base == "" {
		return prefix
	}
	return prefix + " " + base
}
