package generator

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge/claim"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

// planBuilder 负责把已学习知识转换成可渲染的 Skills 生成计划。
type planBuilder struct {
	skillsLoader *skills.Loader
}

func newPlanBuilder(loader *skills.Loader) *planBuilder {
	return &planBuilder{skillsLoader: loader}
}

// Build 生成主 SKILL.md、agent 元数据和 references 的渲染计划。
func (b *planBuilder) Build(outputPath string, snapshot verifiedKnowledgeSnapshot, summaryResult generationSummary, opts PlanOptions) (*skillgen.Plan, error) {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.build_skills_plan",
		"output_path", outputPath,
		"patterns_count", len(snapshot.Patterns),
	)

	skillName := opts.SkillName
	templateProjectName := opts.ProjectName
	templateLanguage := opts.Language

	if snapshot.RenderProfile == nil {
		return nil, fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
	}
	profile := snapshot.RenderProfile
	patterns := snapshot.Patterns
	if skillName == "" {
		skillName = skillgen.GeneratedSkillName(opts.ProjectName)
	}
	if profile.ProjectName != "" {
		templateProjectName = profile.ProjectName
	}
	if profile.Language != "" {
		templateLanguage = profile.Language
	}
	locale := b.skillsLoader.GetLocale()
	references := referenceAvailability(profile, patterns, !opts.SkipReferences)
	references.Validation = references.Enabled
	references.Testing = references.Enabled && (snapshot.GoTests.HasModules() || snapshot.GoTests.HasTests())
	triggerDescription := skillTriggerDescription(templateProjectName, templateLanguage, locale, profile)

	// 准备模板数据
	data := skillTemplateData{
		ProgramVersion:      opts.ProgramVersion,
		SkillsTemplatesHash: opts.SkillsTemplatesHash,
		ProjectName:         templateProjectName,
		SkillName:           skillName,
		SkillDescription:    triggerDescription,
		Language:            templateLanguage,
		PatternCount:        len(patterns),
		Categories:          len(summaryResult.CategorySummaries),
		LastUpdated:         time.Now().Format("2006-01-02 15:04:05"),
		KeyInsights:         summaryResult.KeyInsights,
		References:          references,
		OverviewReferences:  conditionalProfileReferenceItems(profile, locale, "./references/", references.Enabled),
		ReferenceGroups:     conditionalCategoryReferenceGroups(patterns, locale, references.Enabled),
		WorkflowReferences:  opts.WorkflowReferences,
		StateSummaries:      []string{},
	}

	p := skillgen.NewPlan(outputPath)
	p.AddFile("SKILL.md", skillgen.CatalogTemplate, "project-skill", data)
	p.AgentMetadataData = data

	if !opts.SkipReferences {
		if err := b.appendReferenceFiles(p, summaryResult.CategorySummaries, snapshot, references); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "generator.append_reference_files",
				"duration", time.Since(startedAt),
				"output_path", outputPath,
				"error", err,
			)
			return nil, err
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.build_skills_plan",
		"duration", time.Since(startedAt),
		"patterns_count", len(patterns),
		"categories_count", len(summaryResult.CategorySummaries),
	)
	return p, nil
}

func (b *planBuilder) appendReferenceFiles(p *skillgen.Plan, summaries map[string]categorySummary, snapshot verifiedKnowledgeSnapshot, references ReferenceAvailability) error {
	patterns := snapshot.Patterns
	profile := snapshot.RenderProfile
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_reference_files",
		"summaries_count", len(summaries),
		"patterns_count", len(patterns),
	)

	p.AddDir("references")
	p.AddDir("references/patterns")
	p.RemovePath("references/examples")

	if err := b.appendProjectOverview(p, profile); err != nil {
		return err
	}

	categoriesWithPatterns := categoryNamesWithPatterns(patterns)
	if err := b.appendProfileReferenceFiles(p, profile, categoriesWithPatterns); err != nil {
		return err
	}
	if snapshot.Spec != nil {
		if err := b.appendProjectSpec(p, snapshot.Spec, references); err != nil {
			return err
		}
	}
	b.appendValidationReferences(p, profile, patterns, snapshot.GoTests, references)

	for _, categoryName := range categoriesWithPatterns {
		summary := summaries[categoryName]
		if err := b.appendCategoryPattern(p, categoryName, summary, patterns, categoriesWithPatterns, profile.Language); err != nil {
			return err
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_reference_files",
		"duration", time.Since(startedAt),
		"summaries_count", len(summaries),
	)
	return nil
}

func (b *planBuilder) appendProjectSpec(p *skillgen.Plan, spec *domain.ProjectSpec, references ReferenceAvailability) error {
	p.AddFile("references/project-spec.md", skillgen.ReferenceTemplate, "project-spec", projectSpecTemplateData{
		ProjectSpec: *spec,
		References:  references,
	})
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.append_project_spec",
	)
	return nil
}

func (b *planBuilder) appendProjectOverview(p *skillgen.Plan, profile *domain.ProjectProfile) error {
	data := projectOverviewTemplateData{
		ProjectProfile:      *profile,
		OverviewReferences:  profileReferenceItems(profile, b.skillsLoader.GetLocale(), "./"),
		OverviewSummary:     projectOverviewSummary(profile, b.skillsLoader.GetLocale()),
		ArchitectureSummary: projectArchitectureSummary(profile, b.skillsLoader.GetLocale()),
	}
	p.AddFile("references/project-overview.md", skillgen.ProjectOverviewTemplate, "project-overview", data)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.append_project_overview",
		"placeholder", false,
	)
	return nil
}

func (b *planBuilder) appendValidationReferences(p *skillgen.Plan, profile *domain.ProjectProfile, patterns []domain.Pattern, goTests sourcecode.GoTestInventory, references ReferenceAvailability) {
	locale := b.skillsLoader.GetLocale()
	matrix := validationMatrix(profile, patterns, locale)
	p.AddFile("references/validation.md", skillgen.ReferenceTemplate, "validation", validationReferenceTemplateData{
		Commands: validationCommands(profile),
		Matrix:   matrix,
		Gaps:     validationGaps(profile, matrix, locale),
	})
	if references.Testing {
		p.AddFile("references/testing.md", skillgen.ReferenceTemplate, "testing", testingReferenceTemplateData{Inventory: goTests})
	}
}

func (b *planBuilder) appendProfileReferenceFiles(p *skillgen.Plan, profile *domain.ProjectProfile, categoriesWithPatterns []string) error {
	categorySet := make(map[string]bool, len(categoriesWithPatterns))
	for _, category := range categoriesWithPatterns {
		categorySet[category] = true
	}
	data := profileReferenceTemplateData{
		ProjectProfile:      *profile,
		HasBusinessPatterns: categorySet[string(domain.CategoryBusiness)],
		HasUtilityPatterns:  categorySet[string(domain.CategoryUtils)],
		CodeFenceLanguage:   codeFenceLanguage(profile.Language),
		BusinessMethodIndex: buildBusinessMethodIndex(profile.BusinessMethods, b.skillsLoader.GetLocale()),
	}
	files := []struct {
		templateName string
		outputName   string
		enabled      bool
		data         any
	}{
		{templateName: "business-methods", outputName: "business-methods.md", enabled: len(profile.BusinessMethods) > 0, data: data},
		{templateName: "modules", outputName: "modules.md", enabled: len(profile.KeyModules) > 0, data: moduleReferenceData(profile)},
		{templateName: "common-utils", outputName: "common-utils.md", enabled: len(profile.CommonUtils) > 0, data: data},
	}

	for _, file := range files {
		if !file.enabled {
			continue
		}
		outputPath := filepath.Join("references", file.outputName)
		p.AddFile(outputPath, skillgen.ReferenceTemplate, file.templateName, file.data)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.append_profile_reference",
			"reference", file.templateName,
			"path", outputPath,
		)
	}
	return nil
}

func moduleReferenceData(profile *domain.ProjectProfile) moduleReferenceTemplateData {
	data := moduleReferenceTemplateData{ProjectProfile: *profile}
	nameCounts := make(map[string]int, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		name := strings.TrimSpace(module.Name)
		if name == "" {
			name = strings.TrimSpace(module.Path)
		}
		nameCounts[strings.ToLower(name)]++
	}
	for _, module := range profile.KeyModules {
		displayName := strings.TrimSpace(module.Name)
		if displayName == "" {
			displayName = strings.TrimSpace(module.Path)
		}
		if nameCounts[strings.ToLower(displayName)] > 1 {
			displayName = disambiguatedModuleName(displayName, module.Path)
		}
		module.DisplayName = displayName
		data.KeyModules = append(data.KeyModules, module)
	}
	return data
}

func disambiguatedModuleName(name, path string) string {
	path = strings.Trim(strings.TrimSpace(filepath.ToSlash(path)), "/")
	if path == "" {
		return name
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return name + " (" + strings.Join(parts[len(parts)-2:], "/") + ")"
	}
	return name + " (" + path + ")"
}

func (b *planBuilder) appendCategoryPattern(p *skillgen.Plan, categoryName string, summary categorySummary, allPatterns []domain.Pattern, allCategories []string, language string) error {
	var categoryPatterns []domain.Pattern
	for _, pattern := range allPatterns {
		if string(pattern.Category) == categoryName {
			categoryPatterns = append(categoryPatterns, patternForTemplate(pattern))
		}
	}

	if len(categoryPatterns) == 0 {
		logger.Warn(i18n.Get("LoggerGeneratorSkipEmptyCategoryPattern"), "category", categoryName)
		return nil
	}

	if categoryName == string(domain.CategoryBusiness) {
		return b.appendSplitBusinessPatterns(p, summary, categoryPatterns, allCategories, language)
	}

	logger.Diagnostic(i18n.Get("LoggerGeneratorGeneratingPatternFile"),
		"category", categoryName,
		"patternObjects", len(categoryPatterns),
		"summaryPatterns", len(summary.Patterns),
		"firstPatternGoodExample", len(categoryPatterns[0].GoodExample),
	)

	data := categoryPatternTemplateData{
		Category:          summary.Category,
		Summary:           summary.Summary,
		PatternObjects:    patternsForTemplate(categoryPatterns),
		ClaimGroups:       claim.Groups(categoryPatterns, b.skillsLoader.GetLocale()),
		PatternCount:      len(categoryPatterns),
		LastUpdated:       time.Now().Format("2006-01-02 15:04:05"),
		CodeFenceLanguage: codeFenceLanguage(language),
		RelatedReferences: categoryReferenceLinks(categoryName, allCategories, b.skillsLoader.GetLocale(), "./"),
	}

	outputPath := filepath.Join("references", "patterns", categoryName+".md")
	p.AddFile(outputPath, skillgen.PatternTemplate, templateCategoryName(categoryName), data)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.append_category_pattern",
		"category", categoryName,
		"path", outputPath,
		"patterns_count", len(categoryPatterns),
	)
	return nil
}

func (b *planBuilder) appendSplitBusinessPatterns(p *skillgen.Plan, summary categorySummary, categoryPatterns []domain.Pattern, allCategories []string, language string) error {
	groups := businessPatternGroups(b.skillsLoader.GetLocale(), categoryPatterns)
	p.AddDir(filepath.Join("references", "patterns", string(domain.CategoryBusiness)))

	indexData := businessIndexTemplateData{
		Category:          summary.Category,
		Summary:           summary.Summary,
		PatternCount:      len(categoryPatterns),
		LastUpdated:       time.Now().Format("2006-01-02 15:04:05"),
		Groups:            groups,
		RelatedReferences: businessPatternReferenceLinks(allCategories, b.skillsLoader.GetLocale(), "./"),
		CoverageWarnings:  businessCoverageWarnings(groups, b.skillsLoader.GetLocale()),
	}

	p.AddFile(
		filepath.Join("references", "patterns", "business.md"),
		skillgen.RelativeTemplate,
		"project/references/patterns/business-index",
		indexData,
	)

	for _, group := range groups {
		data := businessDetailTemplateData{
			Category:          summary.Category,
			GroupTitle:        group.Title,
			GroupSummary:      group.Summary,
			PatternObjects:    group.Patterns,
			PatternCount:      len(group.Patterns),
			LastUpdated:       time.Now().Format("2006-01-02 15:04:05"),
			CodeFenceLanguage: codeFenceLanguage(language),
			RelatedReferences: businessPatternReferenceLinks(allCategories, b.skillsLoader.GetLocale(), "../"),
		}
		outputPath := filepath.Join("references", "patterns", string(domain.CategoryBusiness), group.ID+".md")
		p.AddFile(outputPath, skillgen.RelativeTemplate, "project/references/patterns/business-detail", data)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.append_split_business_patterns",
		"groups_count", len(groups),
		"patterns_count", len(categoryPatterns),
	)
	return nil
}

func codeFenceLanguage(language string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("+#-_.", r) {
			return r
		}
		return -1
	}, strings.ToLower(strings.TrimSpace(language)))
}

// PlanOptions 封装生成计划所需的项目级参数。
type PlanOptions struct {
	SkillName           string
	ProjectName         string
	Language            string
	ProgramVersion      string
	SkillsTemplatesHash string
	SkipReferences      bool
	WorkflowReferences  []WorkflowReference
}
