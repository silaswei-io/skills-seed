package generator

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func patternImportanceGroups(patterns []domain.Pattern, locale string) []PatternImportanceGroup {
	if len(patterns) == 0 {
		return nil
	}
	groups := []PatternImportanceGroup{
		{Title: localizedText(locale, "核心开发路径", "Core Development Paths"), Description: localizedText(locale, "高置信、跨文件或高频出现，改动相关能力时应优先读取。", "High-confidence, cross-file, or frequent patterns to read first for related changes.")},
		{Title: localizedText(locale, "常用项目约定", "Common Project Conventions"), Description: localizedText(locale, "覆盖常见开发动作，但需要结合当前改动范围判断是否适用。", "Common development conventions that still need change-scope judgment.")},
		{Title: localizedText(locale, "局部模块经验", "Local Module Experience"), Description: localizedText(locale, "证据集中在少数模块，适合命中相邻路径时参考。", "Evidence is concentrated in a few modules; use when working in adjacent paths.")},
		{Title: localizedText(locale, "参考观察", "Reference Observations"), Description: localizedText(locale, "证据较弱或偏归纳，只能作为导航线索，不能作为硬约束。", "Weaker or more inferential findings; use as navigation hints, not hard constraints.")},
	}
	for _, pattern := range patterns {
		pattern = patternForTemplate(pattern)
		index := patternImportanceIndex(pattern)
		groups[index].Patterns = append(groups[index].Patterns, pattern)
	}
	result := make([]PatternImportanceGroup, 0, len(groups))
	for _, group := range groups {
		if len(group.Patterns) == 0 {
			continue
		}
		sort.SliceStable(group.Patterns, func(i, j int) bool {
			left := group.Patterns[i]
			right := group.Patterns[j]
			if left.Confidence != right.Confidence {
				return left.Confidence > right.Confidence
			}
			if left.Frequency != right.Frequency {
				return left.Frequency > right.Frequency
			}
			return left.Name < right.Name
		})
		result = append(result, group)
	}
	return result
}

func patternImportanceIndex(pattern domain.Pattern) int {
	evidenceCount := len(pattern.EvidenceLocations)
	if pattern.BusinessMethod != nil && strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()) != "" {
		evidenceCount++
	}
	if pattern.Metrics.EvidenceCount > evidenceCount {
		evidenceCount = pattern.Metrics.EvidenceCount
	}
	if pattern.Confidence >= 0.85 && (pattern.Frequency >= 2 || evidenceCount >= 2 || pattern.Category == domain.CategoryBusiness || pattern.Category == domain.CategoryAPI) {
		return 0
	}
	if pattern.Confidence >= 0.75 && (pattern.Frequency >= 1 || evidenceCount >= 1) {
		return 1
	}
	if evidenceCount >= 1 || strings.TrimSpace(pattern.ScopePath) != "" {
		return 2
	}
	return 3
}

func validationMatrix(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []ValidationMatrixItem {
	commands := validationCommands(profile, patterns, locale)
	if len(commands) == 0 {
		return nil
	}
	areas := validationAreas(profile, patterns, locale)
	matrix := make([]ValidationMatrixItem, 0, len(areas))
	for _, area := range areas {
		command := chooseValidationCommand(commands, area)
		if command.Command == "" {
			continue
		}
		matrix = append(matrix, ValidationMatrixItem{
			Area:     area.Name,
			Command:  command.Command,
			When:     firstNonEmptyString(command.When, area.When),
			Source:   command.Source,
			Evidence: limitStrings(area.Evidence, 3),
		})
	}
	return matrix
}

type validationArea struct {
	Name     string
	Needles  []string
	When     string
	Evidence []string
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
	evidence := make([]string, 0)
	if profile != nil {
		for _, module := range profile.KeyModules {
			text := strings.ToLower(module.Name + " " + module.Path + " " + module.Description + " " + strings.Join(module.Responsibilities, " ") + " " + strings.Join(module.KeyMethods, " "))
			if containsAny(text, needles...) {
				evidence = append(evidence, firstNonEmptyString(module.Path, module.Name))
			}
		}
	}
	for _, pattern := range patterns {
		text := strings.ToLower(pattern.Name + " " + string(pattern.Category) + " " + pattern.Description + " " + pattern.Rule + " " + pattern.ScopePath)
		if !containsAny(text, needles...) {
			continue
		}
		for _, location := range pattern.EvidenceLocations {
			if location.DisplayLocation() != "" {
				evidence = append(evidence, location.DisplayLocation())
			}
		}
		if pattern.BusinessMethod != nil && pattern.BusinessMethod.DisplayLocation() != "" {
			evidence = append(evidence, pattern.BusinessMethod.DisplayLocation())
		}
		if pattern.ScopePath != "" {
			evidence = append(evidence, pattern.ScopePath)
		}
	}
	return uniqueStrings(evidence)
}

func chooseValidationCommand(commands []validationCommand, area validationArea) validationCommand {
	for _, command := range commands {
		text := strings.ToLower(command.Command + " " + command.When + " " + command.Source)
		if containsAny(text, area.Needles...) {
			return command
		}
	}
	for _, command := range commands {
		if strings.TrimSpace(command.Command) != "" {
			return command
		}
	}
	return validationCommand{}
}
