package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/validation"
)

type skillWorkflow struct {
	Title       string
	AppliesWhen string
	Steps       []string
}

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
	hints := projectSpecificTriggerHints(profile, locale)
	if len(hints) == 0 {
		return generatorTextWithParams(locale, "GeneratorSkillDescriptionDefault", map[string]interface{}{
			"Project":  project,
			"Language": lang,
		})
	}
	return generatorTextWithParams(locale, "GeneratorSkillDescriptionWithHints", map[string]interface{}{
		"Project":  project,
		"Language": lang,
		"Hints":    generatorListJoin(locale, hints),
	})
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

	if len(profile.BusinessMethods) > 0 {
		add(generatorText(locale, "GeneratorTriggerHintBusinessEntry"))
	}
	for _, method := range profile.BusinessMethods {
		text := strings.ToLower(method.Name + " " + method.Description + " " + method.Usage + " " + method.Type + " " + method.Function + " " + method.DisplayLocation())
		switch {
		case containsAny(text, "auth", "permission", "access", "policy", "role", "权限", "授权", "访问", "策略", "角色"):
			add(generatorText(locale, "GeneratorTriggerHintAuthorizationPolicy"))
		case containsAny(text, "state", "status", "workflow", "transition", "lifecycle", "状态", "流程", "流转", "生命周期"):
			add(generatorText(locale, "GeneratorTriggerHintStateTransition"))
		case containsAny(text, "repo", "repository", "store", "database", "transaction", "query", "持久化", "数据库", "事务", "查询"):
			add(generatorText(locale, "GeneratorTriggerHintPersistenceOrchestration"))
		}
		if len(hints) >= 4 {
			return hints
		}
	}

	for _, module := range profile.KeyModules {
		name := strings.TrimSpace(module.Name)
		lower := strings.ToLower(name + " " + module.Path + " " + module.Description)
		switch {
		case containsAny(lower, "domain", "business", "service", "workflow", "process", "usecase", "application", "领域", "业务", "流程", "编排"):
			add(generatorText(locale, "GeneratorTriggerHintCoreBusinessFlow"))
		case containsAny(lower, "map", "transform", "convert", "adapter", "present", "dto", "转换", "适配", "映射"):
			add(generatorText(locale, "GeneratorTriggerHintBoundaryTransformation"))
		case containsAny(lower, "client", "remote", "external", "integration", "upstream", "downstream", "gateway", "adapter", "外部", "集成", "依赖"):
			add(generatorText(locale, "GeneratorTriggerHintExternalDependencyWrapper"))
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
			generatorText(locale, "GeneratorWorkflowAPIStepReadSpec"),
			generatorText(locale, "GeneratorWorkflowAPIStepKeepBoundaries"),
			generatorText(locale, "GeneratorWorkflowAPIStepCheckTransform"),
		}
		if hasGeneratedArtifactSignals(profile) {
			steps = append([]string{
				generatorText(locale, "GeneratorWorkflowAPIStepRegenerateArtifacts"),
			}, steps...)
		}
		workflows = append(workflows, skillWorkflow{
			Title:       generatorText(locale, "GeneratorWorkflowAPITitle"),
			AppliesWhen: generatorText(locale, "GeneratorWorkflowAPIWhen"),
			Steps:       steps,
		})
	}
	if hasBusinessSignals(profile, patterns) {
		workflows = append(workflows, skillWorkflow{
			Title:       generatorText(locale, "GeneratorWorkflowBusinessTitle"),
			AppliesWhen: generatorText(locale, "GeneratorWorkflowBusinessWhen"),
			Steps: []string{
				generatorText(locale, "GeneratorWorkflowBusinessStepReadMap"),
				generatorText(locale, "GeneratorWorkflowBusinessStepVerifyCode"),
				generatorText(locale, "GeneratorWorkflowBusinessStepConfirmSemantics"),
			},
		})
	}
	if hasRPCSignals(profile, patterns) {
		workflows = append(workflows, skillWorkflow{
			Title:       generatorText(locale, "GeneratorWorkflowExternalDependencyTitle"),
			AppliesWhen: generatorText(locale, "GeneratorWorkflowExternalDependencyWhen"),
			Steps: []string{
				generatorText(locale, "GeneratorWorkflowExternalDependencyStepReadStructure"),
				generatorText(locale, "GeneratorWorkflowExternalDependencyStepKeepInfraBoundary"),
			},
		})
	}
	if hasConfigSignals(profile, patterns) {
		workflows = append(workflows, skillWorkflow{
			Title:       generatorText(locale, "GeneratorWorkflowConfigTitle"),
			AppliesWhen: generatorText(locale, "GeneratorWorkflowConfigWhen"),
			Steps: []string{
				generatorText(locale, "GeneratorWorkflowConfigStepReadPatterns"),
				generatorText(locale, "GeneratorWorkflowConfigStepHandleReloadErrors"),
			},
		})
	}
	return workflows
}

func validationCommands(profile *domain.ProjectProfile, _ []domain.Pattern, _ string) []validationCommand {
	return validation.Commands(profile)
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
