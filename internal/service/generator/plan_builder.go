package generator

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge/claim"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
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
func (b *planBuilder) Build(outputPath string, patterns []domain.Pattern, summaryResult *agent.GenerateSkillsResult, stats *Stats, profile *domain.ProjectProfile, spec *domain.ProjectSpec, opts PlanOptions) (*skillgen.Plan, error) {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.build_skills_plan",
		"output_path", outputPath,
		"patterns_count", len(patterns),
	)

	if summaryResult == nil {
		return nil, fmt.Errorf("%s", i18n.Get("GeneratorGenerateSummaryFailed"))
	}

	skillName := opts.SkillName
	templateProjectName := opts.ProjectName
	templateLanguage := opts.Language

	if profile == nil {
		return nil, fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
	}
	profile = domain.CleanProjectProfile(profile)
	profile, patterns = sanitizeGenerationInputs(profile, patterns, opts.ProjectRoot)
	profile = profileForSkillTemplates(profile, patterns)
	if skillName == "" {
		skillName = generatedSkillName(opts.ProjectName)
	}
	if profile.ProjectName != "" {
		templateProjectName = profile.ProjectName
	}
	if profile.Language != "" {
		templateLanguage = profile.Language
	}
	locale := b.skillsLoader.GetLocale()
	references := referenceAvailability(profile, patterns, !opts.SkipReferences)
	triggerDescription := skillTriggerDescription(templateProjectName, templateLanguage, locale, profile)

	// 准备模板数据
	data := map[string]interface{}{
		"ProgramVersion":         opts.ProgramVersion,
		"SkillsTemplatesHash":    opts.SkillsTemplatesHash,
		"ProjectName":            templateProjectName,
		"SkillName":              skillName,
		"SkillDescription":       triggerDescription,
		"Language":               templateLanguage,
		"PatternCount":           len(patterns),
		"AvgConfidence":          stats.AvgConfidence * 100,
		"Categories":             len(summaryResult.CategorySummaries),
		"LastUpdated":            time.Now().Format("2006-01-02 15:04:05"),
		"CategorySummaries":      summaryResult.CategorySummaries,
		"KeyPatterns":            summaryResult.KeyPatterns,
		"BusinessRules":          summaryResult.BusinessRules,
		"BestPractices":          summaryResult.BestPractices,
		"CommonPatterns":         summaryResult.CommonPatterns,
		"KeyInsights":            summaryResult.KeyInsights,
		"ImprovementSuggestions": summaryResult.ImprovementSuggestions,
		"STATS":                  stats,
		"References":             references,
		"OverviewReferences":     conditionalProfileReferenceItems(profile, locale, "./references/", references.Enabled),
		"ReferenceGroups":        conditionalCategoryReferenceGroups(patterns, locale, references.Enabled),
		"Workflows":              skillWorkflows(profile, patterns, locale),
		"WorkflowReferences":     opts.WorkflowReferences,
		"ValidationCommands":     validationCommands(profile, patterns, locale),
		"ValidationMatrix":       validationMatrix(profile, patterns, locale),
		"StateSummaries":         []string{},
	}

	p := skillgen.NewPlan(outputPath)
	p.AddFile("SKILL.md", skillgen.CatalogTemplate, "project-skill", data)
	p.AgentMetadataData = data

	if !opts.SkipReferences {
		if err := b.appendReferenceFiles(p, summaryResult.CategorySummaries, patterns, profile, spec, references); err != nil {
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

func (b *planBuilder) appendReferenceFiles(p *skillgen.Plan, summaries map[string]agent.CategorySummary, patterns []domain.Pattern, profile *domain.ProjectProfile, spec *domain.ProjectSpec, references ReferenceAvailability) error {
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
	if spec != nil {
		if err := b.appendProjectSpec(p, spec, profile, patterns, references); err != nil {
			return err
		}
	}

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

func (b *planBuilder) appendProjectSpec(p *skillgen.Plan, spec *domain.ProjectSpec, profile *domain.ProjectProfile, patterns []domain.Pattern, references ReferenceAvailability) error {
	p.AddFile("references/project-spec.md", skillgen.ReferenceTemplate, "project-spec", projectSpecTemplateData{
		ProjectSpec:      *spec,
		References:       references,
		SourceOfTruth:    []SourceOfTruthItem{},
		ValidationMatrix: validationMatrix(profile, patterns, b.skillsLoader.GetLocale()),
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
		ValidationMatrix:    validationMatrix(profile, nil, b.skillsLoader.GetLocale()),
	}
	p.AddFile("references/project-overview.md", skillgen.ProjectOverviewTemplate, "project-overview", data)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.append_project_overview",
		"placeholder", false,
	)
	return nil
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

func (b *planBuilder) appendCategoryPattern(p *skillgen.Plan, categoryName string, summary agent.CategorySummary, allPatterns []domain.Pattern, allCategories []string, language string) error {
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
		return b.appendSplitBusinessPatterns(p, summary, categoryPatterns, allPatterns, allCategories, language)
	}

	logger.Diagnostic(i18n.Get("LoggerGeneratorGeneratingPatternFile"),
		"category", categoryName,
		"patternObjects", len(categoryPatterns),
		"summaryPatterns", len(summary.Patterns),
		"firstPatternGoodExample", len(categoryPatterns[0].GoodExample),
	)

	data := map[string]interface{}{
		"Category":          summary.Category,
		"Summary":           summary.Summary,
		"Patterns":          summary.Patterns,
		"PatternObjects":    patternsForTemplate(categoryPatterns, b.skillsLoader.GetLocale()),
		"ClaimGroups":       claim.Groups(categoryPatterns, b.skillsLoader.GetLocale()),
		"UsageScenes":       summary.UsageScenes,
		"Priority":          summary.Priority,
		"PatternCount":      len(categoryPatterns),
		"Confidence":        calculateCategoryConfidence(allPatterns, categoryName) * 100,
		"LastUpdated":       time.Now().Format("2006-01-02 15:04:05"),
		"BusinessMethods":   domain.ValidBusinessMethods(summary.BusinessMethods),
		"CodeFenceLanguage": codeFenceLanguage(language),
		"RelatedReferences": categoryReferenceLinks(categoryName, allCategories, b.skillsLoader.GetLocale(), "./"),
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

func (b *planBuilder) appendSplitBusinessPatterns(p *skillgen.Plan, summary agent.CategorySummary, categoryPatterns, allPatterns []domain.Pattern, allCategories []string, language string) error {
	groups := businessPatternGroups(b.skillsLoader.GetLocale(), categoryPatterns)
	p.AddDir(filepath.Join("references", "patterns", string(domain.CategoryBusiness)))

	indexData := map[string]interface{}{
		"Category":          summary.Category,
		"Summary":           summary.Summary,
		"UsageScenes":       summary.UsageScenes,
		"Priority":          summary.Priority,
		"PatternCount":      len(categoryPatterns),
		"Confidence":        calculateCategoryConfidence(allPatterns, string(domain.CategoryBusiness)) * 100,
		"LastUpdated":       time.Now().Format("2006-01-02 15:04:05"),
		"Groups":            groups,
		"ClaimGroups":       claim.Groups(categoryPatterns, b.skillsLoader.GetLocale()),
		"RelatedReferences": businessPatternReferenceLinks(allCategories, b.skillsLoader.GetLocale(), "./"),
		"CoverageWarnings":  businessCoverageWarnings(groups, b.skillsLoader.GetLocale()),
	}

	p.AddFile(
		filepath.Join("references", "patterns", "business.md"),
		skillgen.RelativeTemplate,
		"project/references/patterns/business-index",
		indexData,
	)

	for _, group := range groups {
		data := map[string]interface{}{
			"Category":          summary.Category,
			"GroupTitle":        group.Title,
			"GroupSummary":      group.Summary,
			"GroupLocations":    group.Locations,
			"GroupSignals":      group.Signals,
			"PatternObjects":    group.Patterns,
			"PatternCount":      len(group.Patterns),
			"Confidence":        calculatePatternConfidence(group.Patterns) * 100,
			"LastUpdated":       time.Now().Format("2006-01-02 15:04:05"),
			"CodeFenceLanguage": codeFenceLanguage(language),
			"RelatedReferences": businessPatternReferenceLinks(allCategories, b.skillsLoader.GetLocale(), "../"),
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
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return "go"
	case "typescript", "ts":
		return "typescript"
	case "javascript", "js", "node", "nodejs":
		return "javascript"
	case "python", "py":
		return "python"
	case "java":
		return "java"
	case "rust", "rs":
		return "rust"
	case "csharp", "c#":
		return "csharp"
	case "cpp", "c++":
		return "cpp"
	case "c":
		return "c"
	case "php":
		return "php"
	case "ruby", "rb":
		return "ruby"
	case "swift":
		return "swift"
	case "kotlin":
		return "kotlin"
	default:
		return ""
	}
}

// calculateCategoryConfidence 计算特定分类的平均置信度
func calculateCategoryConfidence(patterns []domain.Pattern, category string) float64 {
	var total float64
	var count int
	for _, p := range patterns {
		if string(p.Category) == category {
			total += p.Confidence
			count++
		}
	}
	if count == 0 {
		return 0.0
	}
	return total / float64(count)
}

func calculatePatternConfidence(patterns []domain.Pattern) float64 {
	if len(patterns) == 0 {
		return 0
	}
	var total float64
	for _, pattern := range patterns {
		total += pattern.Confidence
	}
	return total / float64(len(patterns))
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
	ProjectRoot         string
}
