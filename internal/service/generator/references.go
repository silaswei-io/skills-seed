package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/knowledge/patternview"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

func (s *GeneratorService) ensureCategorySummaries(
	patterns []domain.Pattern,
	summaries map[string]categorySummary,
) map[string]categorySummary {
	if summaries == nil {
		summaries = map[string]categorySummary{}
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
			locale := ""
			if s.skillsLoader != nil {
				locale = s.skillsLoader.GetLocale()
			}
			summary.Summary = generatorTextWithParams(locale, "GeneratorCategorySummary", map[string]interface{}{
				"Category": category,
				"Count":    len(categoryPatterns),
				"Patterns": generatorListJoin(locale, summary.Patterns),
			})
		}
		summaries[category] = summary
	}

	return summaries
}

func categoryNamesWithPatterns(patterns []domain.Pattern) []string {
	return domain.CategoryNamesWithPatterns(patterns)
}

func patternsForSkillTemplates(patterns []domain.Pattern) []domain.Pattern {
	return patternview.Render(patterns)
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
		generatorText(locale, "GeneratorReferenceGroupArchitecture"),
		generatorText(locale, "GeneratorReferenceGroupBusiness"),
		generatorText(locale, "GeneratorReferenceGroupTechnical"),
		generatorText(locale, "GeneratorReferenceGroupAdvanced"),
		generatorText(locale, "GeneratorReferenceGroupOther"),
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
	groupArchitecture := generatorText(locale, "GeneratorReferenceGroupArchitecture")
	groupBusiness := generatorText(locale, "GeneratorReferenceGroupBusiness")
	groupTechnical := generatorText(locale, "GeneratorReferenceGroupTechnical")
	groupAdvanced := generatorText(locale, "GeneratorReferenceGroupAdvanced")
	groupOther := generatorText(locale, "GeneratorReferenceGroupOther")

	metadataByCategory := map[string]categoryReferenceMeta{
		string(domain.CategoryStructure): {
			Group:       groupArchitecture,
			Title:       generatorText(locale, "GeneratorCategoryStructureTitle"),
			Description: generatorText(locale, "GeneratorCategoryStructureDescription"),
		},
		string(domain.CategoryNaming): {
			Group:       groupArchitecture,
			Title:       generatorText(locale, "GeneratorCategoryNamingTitle"),
			Description: generatorText(locale, "GeneratorCategoryNamingDescription"),
		},
		string(domain.CategoryBusiness): {
			Group:       groupBusiness,
			Title:       generatorText(locale, "GeneratorCategoryBusinessTitle"),
			Description: generatorText(locale, "GeneratorCategoryBusinessDescription"),
		},
		string(domain.CategoryDatabase): {
			Group:       groupBusiness,
			Title:       generatorText(locale, "GeneratorCategoryDatabaseTitle"),
			Description: generatorText(locale, "GeneratorCategoryDatabaseDescription"),
		},
		string(domain.CategoryAPI): {
			Group:       groupBusiness,
			Title:       generatorText(locale, "GeneratorCategoryAPITitle"),
			Description: generatorText(locale, "GeneratorCategoryAPIDescription"),
		},
		string(domain.CategoryError): {
			Group:       groupTechnical,
			Title:       generatorText(locale, "GeneratorCategoryErrorTitle"),
			Description: generatorText(locale, "GeneratorCategoryErrorDescription"),
		},
		string(domain.CategoryMiddleware): {
			Group:       groupTechnical,
			Title:       generatorText(locale, "GeneratorCategoryMiddlewareTitle"),
			Description: generatorText(locale, "GeneratorCategoryMiddlewareDescription"),
		},
		string(domain.CategoryConfig): {
			Group:       groupTechnical,
			Title:       generatorText(locale, "GeneratorCategoryConfigTitle"),
			Description: generatorText(locale, "GeneratorCategoryConfigDescription"),
		},
		string(domain.CategoryUtils): {
			Group:       groupTechnical,
			Title:       generatorText(locale, "GeneratorCategoryUtilsTitle"),
			Description: generatorText(locale, "GeneratorCategoryUtilsDescription"),
		},
		string(domain.CategoryConcurrency): {
			Group:       groupAdvanced,
			Title:       generatorText(locale, "GeneratorCategoryConcurrencyTitle"),
			Description: generatorText(locale, "GeneratorCategoryConcurrencyDescription"),
		},
	}

	if meta, ok := metadataByCategory[category]; ok {
		return meta
	}
	return categoryReferenceMeta{
		Group:       groupOther,
		Title:       category,
		Description: generatorText(locale, "GeneratorCategoryDefaultDescription"),
	}
}

func profileReferenceItems(profile *domain.ProjectProfile, locale, prefix string) []skills.ReferenceItem {
	if profile == nil {
		return nil
	}

	items := make([]skills.ReferenceItem, 0, 3)
	if len(profile.BusinessMethods) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       generatorText(locale, "GeneratorProfileReferenceBusinessMethodsTitle"),
			Description: generatorText(locale, "GeneratorProfileReferenceBusinessMethodsDescription"),
			Path:        prefix + "business-methods.md",
		})
	}
	if len(profile.KeyModules) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       generatorText(locale, "GeneratorProfileReferenceModulesTitle"),
			Description: generatorText(locale, "GeneratorProfileReferenceModulesDescription"),
			Path:        prefix + "modules.md",
		})
	}
	if len(profile.CommonUtils) > 0 {
		items = append(items, skills.ReferenceItem{
			Title:       generatorText(locale, "GeneratorProfileReferenceCommonUtilsTitle"),
			Description: generatorText(locale, "GeneratorProfileReferenceCommonUtilsDescription"),
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

func patternsForTemplate(patterns []domain.Pattern) []patternRenderModel {
	result := make([]patternRenderModel, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = patternForTemplate(pattern)
		hardConstraint := pattern.AllowsHardConstraint()
		statement := knowledge.DisplayPatternText(pattern)
		if !hardConstraint {
			pattern.BadExample = ""
			pattern.Rule = ""
			pattern.Description = statement
		}
		result = append(result, patternRenderModel{
			Pattern:             pattern,
			HardConstraint:      hardConstraint,
			HighRiskOperational: pattern.HighRiskOperational(),
		})
	}
	return result
}

func cleanProjectProfile(profile *domain.ProjectProfile) *domain.ProjectProfile {
	return domain.CleanProjectProfile(profile)
}

func profileForSkillTemplates(profile *domain.ProjectProfile, _ []domain.Pattern) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}
	filtered := *profile
	filtered.BusinessMethods = routeableBusinessMethods(profile.BusinessMethods)
	filtered.CommonUtils = nil
	return &filtered
}

func routeableBusinessMethods(methods []domain.BusinessMethod) []domain.BusinessMethod {
	out := make([]domain.BusinessMethod, 0, len(methods))
	for _, method := range methods {
		if domain.IsRouteableBusinessMethod(method) {
			out = append(out, method)
		}
	}
	return out
}

func generatorText(locale, key string) string {
	return i18n.GetForLocale(locale, key)
}

func generatorTextWithParams(locale, key string, params map[string]interface{}) string {
	return i18n.GetForLocaleWithParams(locale, key, params)
}

func generatorListJoin(locale string, values []string) string {
	return strings.Join(values, generatorText(locale, "GeneratorListSeparator"))
}

func templateCategoryName(category string) string {
	switch category {
	case string(domain.CategoryError):
		return "error-handling"
	default:
		return category
	}
}
