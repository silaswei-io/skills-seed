package generator

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

func (s *GeneratorService) ensureCategorySummaries(
	patterns []domain.Pattern,
	summaries map[string]agent.CategorySummary,
) map[string]agent.CategorySummary {
	if summaries == nil {
		summaries = map[string]agent.CategorySummary{}
	}

	byCategory := make(map[string][]domain.Pattern)
	for _, pattern := range patterns {
		category := string(pattern.Category)
		if category == "" {
			continue
		}
		byCategory[category] = append(byCategory[category], pattern)
	}

	for category, categoryPatterns := range byCategory {
		summary := summaries[category]
		if summary.Category == "" {
			summary.Category = category
		}
		if len(summary.Patterns) == 0 {
			summary.Patterns = domain.PatternNames(categoryPatterns)
		}
		if summary.Summary == "" {
			summary.Summary = fmt.Sprintf("%s 分类包含 %d 个项目特定模式：%s。", category, len(categoryPatterns), strings.Join(summary.Patterns, "、"))
		}
		summaries[category] = summary
	}

	return summaries
}

func categoryNamesWithPatterns(patterns []domain.Pattern) []string {
	return domain.CategoryNamesWithPatterns(patterns)
}

func referenceAvailability(profile *domain.ProjectProfile, patterns []domain.Pattern, enabled bool) ReferenceAvailability {
	refs := ReferenceAvailability{Enabled: enabled}
	if !enabled {
		return refs
	}
	refs.ProjectSpec = true
	refs.ProjectOverview = true
	if profile != nil {
		refs.BusinessMethods = len(profile.BusinessMethods) > 0
		refs.KeyModules = len(profile.KeyModules) > 0
		refs.CommonUtils = len(profile.CommonUtils) > 0
	}
	for _, pattern := range patterns {
		if pattern.Category == domain.CategoryBusiness {
			refs.BusinessPatterns = true
			break
		}
	}
	return refs
}

func categoryReferenceGroups(patterns []domain.Pattern, locale string) []skills.ReferenceGroup {
	groupOrder := []string{
		localizedText(locale, "架构与结构", "Architecture & Structure"),
		localizedText(locale, "业务与领域", "Business & Domain"),
		localizedText(locale, "技术模式", "Technical Patterns"),
		localizedText(locale, "高级主题", "Advanced Topics"),
		localizedText(locale, "其他模式", "Other Patterns"),
	}
	groupsByTitle := make(map[string]*skills.ReferenceGroup, len(groupOrder))
	for _, title := range groupOrder {
		groupsByTitle[title] = &skills.ReferenceGroup{Title: title}
	}

	for _, category := range categoryNamesWithPatterns(patterns) {
		meta := categoryReferenceMetadata(category, locale)
		group, ok := groupsByTitle[meta.Group]
		if !ok {
			group = groupsByTitle[groupOrder[len(groupOrder)-1]]
		}
		group.Items = append(group.Items, skills.ReferenceItem{
			Title:       meta.Title,
			Description: meta.Description,
			Path:        "./references/patterns/" + category + ".md",
		})
		if category == string(domain.CategoryBusiness) {
			for _, businessGroup := range businessPatternGroups(locale, businessPatterns(patterns)) {
				group.Items = append(group.Items, skills.ReferenceItem{
					Title:       businessGroup.Title,
					Description: businessGroup.Description,
					Path:        "./references/patterns/business/" + businessGroup.ID + ".md",
				})
			}
		}
	}

	groups := make([]skills.ReferenceGroup, 0, len(groupOrder))
	for _, title := range groupOrder {
		group := groupsByTitle[title]
		if len(group.Items) > 0 {
			groups = append(groups, *group)
		}
	}
	return groups
}

func conditionalCategoryReferenceGroups(patterns []domain.Pattern, locale string, enabled bool) []skills.ReferenceGroup {
	if !enabled {
		return nil
	}
	return categoryReferenceGroups(patterns, locale)
}

func categoryReferenceMetadata(category, locale string) categoryReferenceMeta {
	groupArchitecture := localizedText(locale, "架构与结构", "Architecture & Structure")
	groupBusiness := localizedText(locale, "业务与领域", "Business & Domain")
	groupTechnical := localizedText(locale, "技术模式", "Technical Patterns")
	groupAdvanced := localizedText(locale, "高级主题", "Advanced Topics")
	groupOther := localizedText(locale, "其他模式", "Other Patterns")

	metadataByCategory := map[string]categoryReferenceMeta{
		string(domain.CategoryStructure): {
			Group:       groupArchitecture,
			Title:       localizedText(locale, "结构模式", "Structure Patterns"),
			Description: localizedText(locale, "代码组织模式", "Code organization patterns"),
		},
		string(domain.CategoryNaming): {
			Group:       groupArchitecture,
			Title:       localizedText(locale, "命名模式", "Naming Patterns"),
			Description: localizedText(locale, "命名约定", "Naming conventions"),
		},
		string(domain.CategoryBusiness): {
			Group:       groupBusiness,
			Title:       localizedText(locale, "业务模式", "Business Patterns"),
			Description: localizedText(locale, "业务逻辑模式", "Business logic patterns"),
		},
		string(domain.CategoryDatabase): {
			Group:       groupBusiness,
			Title:       localizedText(locale, "数据库模式", "Database Patterns"),
			Description: localizedText(locale, "数据库操作", "Database operations"),
		},
		string(domain.CategoryAPI): {
			Group:       groupBusiness,
			Title:       localizedText(locale, "API 模式", "API Patterns"),
			Description: localizedText(locale, "API 设计", "API design"),
		},
		string(domain.CategoryError): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "错误处理", "Error Handling"),
			Description: localizedText(locale, "错误处理模式", "Error handling patterns"),
		},
		string(domain.CategoryMiddleware): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "中间件模式", "Middleware Patterns"),
			Description: localizedText(locale, "中间件设计", "Middleware design"),
		},
		string(domain.CategoryConfig): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "配置模式", "Config Patterns"),
			Description: localizedText(locale, "配置管理", "Configuration management"),
		},
		string(domain.CategoryUtils): {
			Group:       groupTechnical,
			Title:       localizedText(locale, "工具模式", "Utils Patterns"),
			Description: localizedText(locale, "工具方法", "Utility methods"),
		},
		string(domain.CategoryConcurrency): {
			Group:       groupAdvanced,
			Title:       localizedText(locale, "并发编程", "Concurrency Patterns"),
			Description: localizedText(locale, "并发编程模式", "Concurrency programming patterns"),
		},
		string(domain.CategoryTesting): {
			Group:       groupAdvanced,
			Title:       localizedText(locale, "测试模式", "Testing Patterns"),
			Description: localizedText(locale, "测试约定", "Testing conventions"),
		},
	}

	if meta, ok := metadataByCategory[category]; ok {
		return meta
	}
	return categoryReferenceMeta{
		Group:       groupOther,
		Title:       category,
		Description: localizedText(locale, "项目特定模式", "Project-specific patterns"),
	}
}

func profileReferenceItems(profile *domain.ProjectProfile, locale, prefix string) []skills.ReferenceItem {
	if profile == nil {
		return nil
	}

	items := make([]skills.ReferenceItem, 0, 3)
	if len(profile.BusinessMethods) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       localizedText(locale, "强业务方法", "Business Methods"),
			Description: localizedText(locale, "复杂业务逻辑方法完整清单", "Full list of complex business logic methods"),
			Path:        prefix + "business-methods.md",
		})
	}
	if len(profile.KeyModules) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       localizedText(locale, "关键模块", "Key Modules"),
			Description: localizedText(locale, "关键模块职责、依赖和入口方法", "Key module responsibilities, dependencies, and entry methods"),
			Path:        prefix + "modules.md",
		})
	}
	if len(profile.CommonUtils) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       localizedText(locale, "通用工具", "Common Utilities"),
			Description: localizedText(locale, "通用工具方法及使用场景", "Common utility functions and usage scenarios"),
			Path:        prefix + "common-utils.md",
		})
	}
	return items
}

func conditionalProfileReferenceItems(profile *domain.ProjectProfile, locale, prefix string, enabled bool) []skills.ReferenceItem {
	if !enabled {
		return nil
	}
	return profileReferenceItems(profile, locale, prefix)
}

func patternForTemplate(pattern domain.Pattern) domain.Pattern {
	if !domain.IsUsableBusinessMethod(pattern.BusinessMethod) {
		pattern.BusinessMethod = nil
	}
	return pattern
}

func cleanProjectProfile(profile *domain.ProjectProfile) *domain.ProjectProfile {
	return domain.CleanProjectProfile(profile)
}

func localizedText(locale, zhCN, enUS string) string {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return zhCN
	}
	return enUS
}

func generatedSkillName(projectName string) string {
	var b strings.Builder
	previousHyphen := false
	for _, r := range strings.ToLower(projectName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousHyphen = false
			continue
		}
		if !previousHyphen {
			b.WriteRune('-')
			previousHyphen = true
		}
	}

	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "project"
	}
	if !strings.HasSuffix(name, "-dev") {
		name += "-dev"
	}
	return name
}

func (s *GeneratorService) projectSpecFromProfileAndPatterns(profile *domain.ProjectProfile, patterns []domain.Pattern, project config.WorkspaceProjectConfig) *domain.ProjectSpec {
	return domain.NewProjectSpecFromProfile(profile, patterns, domain.WorkspaceProjectOverride{
		ID:       project.ID,
		Path:     project.Path,
		Type:     project.Type,
		Language: project.Language,
	})
}

func templateCategoryName(category string) string {
	switch category {
	case string(domain.CategoryError):
		return "error-handling"
	default:
		return category
	}
}
