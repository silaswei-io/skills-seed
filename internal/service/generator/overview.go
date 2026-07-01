package generator

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func projectOverviewSummary(profile *domain.ProjectProfile, locale string) string {
	if profile == nil {
		return ""
	}
	summary := strings.TrimSpace(profile.Summary)
	if summary != "" && !looksLikeUnitScopedOverview(summary, profile) {
		return summary
	}
	return learnedCoverageSummary(profile, locale)
}

func projectArchitectureSummary(profile *domain.ProjectProfile, locale string) string {
	if profile == nil {
		return ""
	}
	architecture := strings.TrimSpace(profile.Architecture)
	if architecture != "" && !looksLikeUnitScopedOverview(architecture, profile) {
		return architecture
	}
	if len(profile.Layers) == 0 {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return fmt.Sprintf("已从当前代码学习到 %d 个架构层次；本节用于导航已沉淀的结构事实，具体职责以“架构层次”和“关键模块”章节为准。", len(profile.Layers))
	}
	return fmt.Sprintf("The current codebase has %d learned architecture layers. This section is a navigation summary; use the Architecture Layers and Key Modules sections for concrete responsibilities.", len(profile.Layers))
}

func looksLikeUnitScopedOverview(value string, profile *domain.ProjectProfile) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || profile == nil {
		return false
	}
	if !hasBroadProfileEvidence(profile) {
		return false
	}
	return containsAny(normalized, "单元", "unit", "analysis unit")
}

func hasBroadProfileEvidence(profile *domain.ProjectProfile) bool {
	if profile == nil {
		return false
	}
	signals := 0
	if len(profile.KeyModules) > 1 {
		signals++
	}
	if len(profile.BusinessMethods) > 1 {
		signals++
	}
	if len(profile.Layers) > 1 {
		signals++
	}
	return signals >= 1
}

func learnedCoverageSummary(profile *domain.ProjectProfile, locale string) string {
	const previewLimit = 8
	domains := learnedDomainNames(profile, previewLimit)
	total := learnedDomainTotal(profile)
	switch {
	case strings.HasPrefix(strings.ToLower(locale), "zh") && len(domains) > 0:
		if total > len(domains) {
			return fmt.Sprintf("当前项目画像已覆盖 %d 个模块/业务域；此处仅列出前 %d 个用于导航：%s。完整列表见关键模块、入口方法和分类模式；具体业务规则、架构边界和实现细节需结合当前代码确认，不能把局部摘要当作完整项目事实。", total, len(domains), strings.Join(domains, "、"))
		}
		return fmt.Sprintf("当前项目画像已覆盖 %d 个模块/业务域：%s。本节用于快速定位项目范围和参考入口；具体业务规则、架构边界和实现细节需结合当前代码、项目规范和分类模式确认，不能把单个业务域摘要当作完整项目事实。", total, strings.Join(domains, "、"))
	case len(domains) > 0:
		if total > len(domains) {
			return fmt.Sprintf("The learned project profile currently covers %d modules or business domains; this preview lists the first %d for navigation: %s. Use Key Modules, entry methods, and category patterns for the full reference set; confirm concrete business rules, architecture boundaries, and implementation details against current code instead of promoting one local summary to whole-project fact.", total, len(domains), strings.Join(domains, ", "))
		}
		return fmt.Sprintf("The learned project profile currently covers %d modules or business domains: %s. Use this section to quickly locate project scope and reference entry points; confirm concrete business rules, architecture boundaries, and implementation details against current code, Project Spec, and category patterns instead of promoting one domain summary to whole-project fact.", total, strings.Join(domains, ", "))
	case strings.HasPrefix(strings.ToLower(locale), "zh"):
		return "项目概览摘要尚未从完整项目画像中稳定提取；请结合当前代码、项目规范和分类模式确认全局结论。"
	default:
		return "A stable whole-project overview summary has not been extracted yet; confirm global conclusions against current code, Project Spec, and category patterns."
	}
}

func learnedDomainTotal(profile *domain.ProjectProfile) int {
	if profile == nil {
		return 0
	}
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		seen[strings.ToLower(value)] = true
	}
	for _, module := range profile.KeyModules {
		add(firstNonEmptyString(module.Name, module.Path))
	}
	for _, method := range profile.BusinessMethods {
		add(method.Name)
	}
	for _, layer := range profile.Layers {
		add(layer.Name)
	}
	return len(seen)
}

func learnedDomainNames(profile *domain.ProjectProfile, limit int) []string {
	if profile == nil || limit <= 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, limit)
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
		out = append(out, value)
	}
	for _, module := range profile.KeyModules {
		add(firstNonEmptyString(module.Name, module.Path))
		if len(out) >= limit {
			return out
		}
	}
	for _, method := range profile.BusinessMethods {
		add(method.Name)
		if len(out) >= limit {
			return out
		}
	}
	for _, layer := range profile.Layers {
		add(layer.Name)
		if len(out) >= limit {
			return out
		}
	}
	return out
}
