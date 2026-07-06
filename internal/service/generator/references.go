package generator

import (
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
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
		if category == string(domain.CategoryBusiness) {
			for _, businessGroup := range businessPatternGroups(locale, businessPatterns(patterns)) {
				group.Items = append(group.Items, skills.ReferenceItem{
					Title:       businessGroup.Title,
					Description: businessGroup.Summary.Description,
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
		string(domain.CategoryTesting): {
			Group:       groupAdvanced,
			Title:       generatorText(locale, "GeneratorCategoryTestingTitle"),
			Description: generatorText(locale, "GeneratorCategoryTestingDescription"),
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

func businessPatterns(patterns []domain.Pattern) []domain.Pattern {
	result := make([]domain.Pattern, 0)
	for _, pattern := range patterns {
		if pattern.Category == domain.CategoryBusiness {
			result = append(result, patternForTemplate(pattern))
		}
	}
	return result
}

func cleanProjectProfile(profile *domain.ProjectProfile) *domain.ProjectProfile {
	return domain.CleanProjectProfile(profile)
}

func profileForSkillTemplates(profile *domain.ProjectProfile, patterns []domain.Pattern) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}
	filtered := *profile
	filtered.CommonUtils = filterCommonUtilsCoveredByBusinessPatterns(profile.CommonUtils, patterns)
	return &filtered
}

func filterCommonUtilsCoveredByBusinessPatterns(utils []domain.UtilityFunction, patterns []domain.Pattern) []domain.UtilityFunction {
	if len(utils) == 0 || len(patterns) == 0 {
		return utils
	}
	covered := businessPatternEvidenceIndex(patterns)
	if len(covered) == 0 {
		return utils
	}
	out := make([]domain.UtilityFunction, 0, len(utils))
	for _, utility := range utils {
		if covered[normalizeReferencePath(utility.File)] {
			continue
		}
		if covered[normalizeReferencePath(utility.Signature)] {
			continue
		}
		out = append(out, utility)
	}
	return out
}

func businessPatternEvidenceIndex(patterns []domain.Pattern) map[string]bool {
	covered := map[string]bool{}
	for _, pattern := range patterns {
		if pattern.Category != domain.CategoryBusiness {
			continue
		}
		for _, location := range pattern.EvidenceLocations {
			if key := normalizeReferencePath(location.Path); key != "" {
				covered[key] = true
			}
		}
		if pattern.BusinessMethod != nil {
			if key := normalizeReferencePath(pattern.BusinessMethod.Function); key != "" {
				covered[key] = true
			}
			if key := normalizeReferencePath(pattern.BusinessMethod.DisplayLocation()); key != "" {
				covered[key] = true
			}
		}
	}
	return covered
}

func normalizeReferencePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ":"); idx > 0 && strings.Contains(value[:idx], "/") {
		value = value[:idx]
	}
	return strings.ToLower(strings.Trim(filepath.ToSlash(value), "` "))
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
