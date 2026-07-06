package generator

import "github.com/silaswei-io/skills-seed/internal/domain"

type categoryLinkRule struct {
	Category  string
	ReasonKey string
}

func categoryReferenceLinks(category string, allCategories []string, locale, pathPrefix string) []PatternReferenceLink {
	existing := make(map[string]bool, len(allCategories))
	for _, name := range allCategories {
		existing[name] = true
	}
	if pathPrefix == "" {
		pathPrefix = "./"
	}

	rules := relatedCategoryRules()[category]
	links := make([]PatternReferenceLink, 0, len(rules))
	for _, rule := range rules {
		if !existing[rule.Category] || rule.Category == category {
			continue
		}
		meta := categoryReferenceMetadata(rule.Category, locale)
		links = append(links, PatternReferenceLink{
			Title:  meta.Title,
			Path:   pathPrefix + rule.Category + ".md",
			Reason: generatorText(locale, rule.ReasonKey),
		})
	}
	return links
}

func businessPatternReferenceLinks(allCategories []string, locale, pathPrefix string) []PatternReferenceLink {
	return categoryReferenceLinks(string(domain.CategoryBusiness), allCategories, locale, pathPrefix)
}

func relatedCategoryRules() map[string][]categoryLinkRule {
	return map[string][]categoryLinkRule{
		string(domain.CategoryBusiness): {
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkBusinessAPI"),
			linkRule(domain.CategoryDatabase, "GeneratorReferenceLinkBusinessDatabase"),
			linkRule(domain.CategoryError, "GeneratorReferenceLinkBusinessError"),
		},
		string(domain.CategoryAPI): {
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkAPIBusiness"),
			linkRule(domain.CategoryError, "GeneratorReferenceLinkAPIError"),
			linkRule(domain.CategoryMiddleware, "GeneratorReferenceLinkAPIMiddleware"),
			linkRule(domain.CategoryConfig, "GeneratorReferenceLinkAPIConfig"),
		},
		string(domain.CategoryDatabase): {
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkDatabaseBusiness"),
			linkRule(domain.CategoryConcurrency, "GeneratorReferenceLinkDatabaseConcurrency"),
			linkRule(domain.CategoryError, "GeneratorReferenceLinkDatabaseError"),
		},
		string(domain.CategoryMiddleware): {
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkMiddlewareAPI"),
			linkRule(domain.CategoryConfig, "GeneratorReferenceLinkMiddlewareConfig"),
			linkRule(domain.CategoryError, "GeneratorReferenceLinkMiddlewareError"),
		},
		string(domain.CategoryConfig): {
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkConfigAPI"),
			linkRule(domain.CategoryMiddleware, "GeneratorReferenceLinkConfigMiddleware"),
			linkRule(domain.CategoryDatabase, "GeneratorReferenceLinkConfigDatabase"),
		},
		string(domain.CategoryError): {
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkErrorAPI"),
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkErrorBusiness"),
			linkRule(domain.CategoryDatabase, "GeneratorReferenceLinkErrorDatabase"),
		},
		string(domain.CategoryUtils): {
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkUtilsBusiness"),
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkUtilsAPI"),
			linkRule(domain.CategoryTesting, "GeneratorReferenceLinkUtilsTesting"),
		},
		string(domain.CategoryConcurrency): {
			linkRule(domain.CategoryDatabase, "GeneratorReferenceLinkConcurrencyDatabase"),
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkConcurrencyBusiness"),
			linkRule(domain.CategoryTesting, "GeneratorReferenceLinkConcurrencyTesting"),
		},
		string(domain.CategoryTesting): {
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkTestingBusiness"),
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkTestingAPI"),
			linkRule(domain.CategoryDatabase, "GeneratorReferenceLinkTestingDatabase"),
		},
		string(domain.CategoryStructure): {
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkStructureBusiness"),
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkStructureAPI"),
			linkRule(domain.CategoryConfig, "GeneratorReferenceLinkStructureConfig"),
		},
		string(domain.CategoryNaming): {
			linkRule(domain.CategoryAPI, "GeneratorReferenceLinkNamingAPI"),
			linkRule(domain.CategoryDatabase, "GeneratorReferenceLinkNamingDatabase"),
			linkRule(domain.CategoryBusiness, "GeneratorReferenceLinkNamingBusiness"),
		},
	}
}

func linkRule(category domain.Category, reasonKey string) categoryLinkRule {
	return categoryLinkRule{
		Category:  string(category),
		ReasonKey: reasonKey,
	}
}
