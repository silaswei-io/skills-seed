package generator

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type skillWorkflow struct {
	Title       string
	AppliesWhen string
	Steps       []string
}

type validationCommand struct {
	Command string
	When    string
	Source  string
}

func skillTriggerDescription(projectName, language, locale string, profile *domain.ProjectProfile) string {
	project := strings.TrimSpace(projectName)
	if profile != nil && strings.TrimSpace(profile.ProjectName) != "" {
		project = strings.TrimSpace(profile.ProjectName)
	}
	if project == "" {
		project = "project"
	}
	lang := strings.TrimSpace(language)
	if profile != nil && strings.TrimSpace(profile.Language) != "" {
		lang = strings.TrimSpace(profile.Language)
	}
	if lang == "" {
		lang = "code"
	}
	hints := projectSpecificTriggerHints(profile, locale)
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		if len(hints) == 0 {
			return fmt.Sprintf("修改、审查或扩展 %s %s 代码且需要遵循该项目已学习的架构、业务流、派生产物或仓库约定时使用", project, lang)
		}
		return fmt.Sprintf("修改、审查或扩展 %s %s 代码且涉及 %s 等项目特定约定时使用", project, lang, strings.Join(hints, "、"))
	}
	if len(hints) == 0 {
		return fmt.Sprintf("Use when modifying, reviewing, or extending %s %s code and project-specific architecture, business flow, generated artifact, or repository conventions matter", project, lang)
	}
	return fmt.Sprintf("Use when modifying, reviewing, or extending %s %s code involving project-specific conventions such as %s", project, lang, strings.Join(hints, ", "))
}

func projectSpecificTriggerHints(profile *domain.ProjectProfile, locale string) []string {
	if profile == nil {
		return nil
	}
	hints := make([]string, 0, 4)
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if seen[key] {
			return
		}
		seen[key] = true
		hints = append(hints, value)
	}

	for _, module := range profile.KeyModules {
		name := strings.TrimSpace(module.Name)
		lower := strings.ToLower(name + " " + module.Path + " " + module.Description)
		switch {
		case containsAny(lower, "domain", "business", "service", "workflow", "process", "usecase", "application", "领域", "业务", "流程", "编排"):
			add(localizedText(locale, "核心业务流", "core business flow"))
		case containsAny(lower, "map", "transform", "convert", "adapter", "present", "dto", "转换", "适配", "映射"):
			add(localizedText(locale, "边界转换", "boundary transformation"))
		case containsAny(lower, "client", "remote", "external", "integration", "upstream", "downstream", "gateway", "adapter", "外部", "集成", "依赖"):
			add(localizedText(locale, "外部依赖封装", "external dependency wrappers"))
		case name != "":
			add(name)
		}
		if len(hints) >= 4 {
			break
		}
	}
	return hints
}

func skillWorkflows(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []skillWorkflow {
	workflows := make([]skillWorkflow, 0, 4)
	if hasAPISignals(profile, patterns) {
		steps := []string{
			localizedText(locale, "先读项目规范和接口/结构模式，确认契约来源、响应形态、转换层和派生产物边界", "Read Project Spec plus interface/structure patterns first to confirm contract source, response shape, transformation layer, and generated-artifact boundaries"),
			localizedText(locale, "修改手写逻辑时保持入口层、编排层和领域实现的职责边界", "Keep responsibility boundaries between entry points, orchestration code, and domain implementation when editing handwritten code"),
			localizedText(locale, "新增或改变对外字段时同步检查转换逻辑和接口类型，避免直接暴露内部模型", "When adding or changing externally visible fields, check transformation logic and interface types together instead of exposing internal models directly"),
		}
		if hasGeneratedArtifactSignals(profile) {
			steps = append([]string{
				localizedText(locale, "先修改接口契约源文件，再重新生成派生产物", "Edit interface contract source files first, then regenerate derived artifacts"),
			}, steps...)
		}
		workflows = append(workflows, skillWorkflow{
			Title:       localizedText(locale, "新增或调整 API", "Add Or Change API"),
			AppliesWhen: localizedText(locale, "涉及接口、路由、请求/响应类型、转换逻辑或派生产物", "Interfaces, routes, request/response types, transformation logic, or generated artifacts change"),
			Steps:       steps,
		})
	}
	if hasBusinessSignals(profile, patterns) {
		workflows = append(workflows, skillWorkflow{
			Title:       localizedText(locale, "修改业务流程", "Change Business Flow"),
			AppliesWhen: localizedText(locale, "涉及状态流转、缓存 key、任务重试、幂等或跨实体业务编排", "State transitions, cache keys, task retry, idempotency, or cross-entity business orchestration change"),
			Steps: []string{
				localizedText(locale, "先读强业务方法和对应业务模式详情，只把有代码证据的规则当作硬约束", "Read Business Methods and the matching business pattern detail first; treat only code-backed rules as hard constraints"),
				localizedText(locale, "对照当前代码确认锁、状态、错误分支和缓存/任务顺序；引用与代码冲突时以代码为准", "Verify locks, states, error branches, and cache/task ordering against current code; current code wins over references"),
				localizedText(locale, "新增产品语义前确认现有代码是否已经实现；没有证据时把它标记为待确认需求，而不是项目规则", "Before adding product semantics, confirm the current code already implements them; if not, mark them as requirements to confirm, not project rules"),
			},
		})
	}
	if hasRPCSignals(profile, patterns) {
		workflows = append(workflows, skillWorkflow{
			Title:       localizedText(locale, "接入或调整外部依赖", "Add Or Change External Dependency"),
			AppliesWhen: localizedText(locale, "涉及远程客户端、外部服务调用或共享上下文注入", "Remote clients, external service calls, or shared context injection change"),
			Steps: []string{
				localizedText(locale, "先读结构模式和项目规范，确认封装层、接口别名和上下文注入约定", "Read structure patterns and Project Spec first to confirm wrapper layers, interface aliases, and context injection conventions"),
				localizedText(locale, "把连接或客户端初始化集中在既有基础设施边界，业务代码通过领域化接口调用", "Keep connection or client initialization inside the existing infrastructure boundary and call dependencies through domain-oriented interfaces"),
			},
		})
	}
	if hasConfigSignals(profile, patterns) {
		workflows = append(workflows, skillWorkflow{
			Title:       localizedText(locale, "修改配置或热加载", "Change Config Or Hot Reload"),
			AppliesWhen: localizedText(locale, "涉及配置结构、配置文件、运行时可更新字段或配置监听器", "Config structures, config files, runtime-updatable fields, or config listeners change"),
			Steps: []string{
				localizedText(locale, "先读配置模式，确认配置结构、默认文件和热加载监听边界", "Read config patterns first to confirm config structs, default files, and hot-reload listener boundaries"),
				localizedText(locale, "新增运行时可更新字段时同步处理 listener 错误路径，不能让 reload 失败导致服务崩溃", "When adding runtime-updatable fields, handle listener error paths so reload failures do not crash the service"),
			},
		})
	}
	return workflows
}

func validationCommands(profile *domain.ProjectProfile, _ []domain.Pattern, _ string) []validationCommand {
	if profile == nil {
		return nil
	}
	learned := domain.CleanValidationCommands(profile.ValidationCommands)
	if len(learned) == 0 {
		return nil
	}
	commands := make([]validationCommand, 0, len(learned))
	for _, learnedCommand := range learned {
		commands = append(commands, validationCommand{
			Command: learnedCommand.Command,
			When:    learnedCommand.When,
			Source:  learnedCommand.Source,
		})
	}
	return commands
}

func hasBusinessSignals(profile *domain.ProjectProfile, patterns []domain.Pattern) bool {
	if profile != nil && len(profile.BusinessMethods) > 0 {
		return true
	}
	return hasCategory(patterns, domain.CategoryBusiness)
}

func hasAPISignals(profile *domain.ProjectProfile, patterns []domain.Pattern) bool {
	if hasCategory(patterns, domain.CategoryAPI) {
		return true
	}
	return profileContainsAny(profile, "api", "interface", "endpoint", "route", "request", "response", "contract", "schema", "接口", "契约", "请求", "响应", "路由")
}

func hasRPCSignals(profile *domain.ProjectProfile, patterns []domain.Pattern) bool {
	return profileContainsAny(profile, "remote", "client", "external service", "integration", "upstream", "downstream", "adapter", "外部服务", "远程", "客户端", "集成")
}

func hasConfigSignals(profile *domain.ProjectProfile, patterns []domain.Pattern) bool {
	return hasCategory(patterns, domain.CategoryConfig) || profileContainsAny(profile, "config", "configuration", "settings", "hot-reload", "watcher", "配置", "热加载", "监听")
}

func hasCategory(patterns []domain.Pattern, category domain.Category) bool {
	for _, pattern := range patterns {
		if pattern.Category == category {
			return true
		}
	}
	return false
}

func hasGeneratedArtifactSignals(profile *domain.ProjectProfile) bool {
	return profileContainsAny(profile, "generated", "derive", "codegen", "generator", "source of truth", "do not edit", "生成", "派生", "不要手改")
}

func profileContainsAny(profile *domain.ProjectProfile, needles ...string) bool {
	if profile == nil {
		return false
	}
	haystack := strings.ToLower(strings.Join(profile.Frameworks, " ") + " " +
		profile.Architecture + " " +
		profile.Structure + " " +
		strings.Join(profile.FrameworkPatterns, " ") + " " +
		strings.Join(profile.ConfigPatterns, " ") + " " +
		strings.Join(profile.Dependencies, " ") + " " +
		profile.DependencyGraph + " " +
		profile.DataFlow + " " +
		profile.Summary)
	for _, layer := range profile.Layers {
		haystack += " " + strings.ToLower(layer.Name+" "+layer.Description+" "+strings.Join(layer.Responsibilities, " ")+" "+strings.Join(layer.Files, " "))
	}
	for _, module := range profile.KeyModules {
		haystack += " " + strings.ToLower(module.Name+" "+module.Path+" "+module.Description+" "+strings.Join(module.Responsibilities, " ")+" "+strings.Join(module.Dependencies, " ")+" "+strings.Join(module.Dependents, " ")+" "+strings.Join(module.KeyMethods, " "))
	}
	for _, needle := range needles {
		if strings.Contains(haystack, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
