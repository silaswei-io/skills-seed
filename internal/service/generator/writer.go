package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

// SkillWriter 封装所有"渲染模板 + 写文件"逻辑
type SkillWriter struct {
	skillsLoader *skills.Loader
}

func NewSkillWriter(loader *skills.Loader) *SkillWriter {
	return &SkillWriter{skillsLoader: loader}
}

func (w *SkillWriter) SkillsLoader() *skills.Loader {
	return w.skillsLoader
}

// WriteSkillsOutput 写入主 SKILL.md + agent 元数据 + 所有参考文档
func (w *SkillWriter) WriteSkillsOutput(ctx context.Context, outputPath string, patterns []domain.Pattern, summaryResult *agent.GenerateSkillsResult, stats *Stats, profile *domain.ProjectProfile, spec *domain.ProjectSpec, opts SkillWriteOptions) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.write_skills_output",
		"output_path", outputPath,
		"patterns_count", len(patterns),
	)

	if summaryResult == nil {
		return fmt.Errorf("%s", i18n.Get("GeneratorGenerateSummaryFailed"))
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.create_output_dir",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	skillName := opts.SkillName
	templateProjectName := opts.ProjectName
	templateLanguage := opts.Language

	if profile == nil {
		return fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
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
	locale := w.skillsLoader.GetLocale()
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

	// 生成主 SKILL.md 文件
	mainPath := filepath.Join(outputPath, "SKILL.md")
	mainContent, err := w.skillsLoader.Render("project-skill", data)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.render_skill_template",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return err
	}

	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.write_skill_file",
			"duration", time.Since(startedAt),
			"path", mainPath,
			"error", err,
		)
		return err
	}

	if err := w.GenerateAgentMetadata(outputPath, data); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_agent_metadata",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	// 生成参考文档
	if !opts.SkipReferences {
		if err := w.GenerateReferenceFiles(ctx, outputPath, summaryResult.CategorySummaries, patterns, profile, spec, references); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "generator.generate_reference_files",
				"duration", time.Since(startedAt),
				"output_path", outputPath,
				"error", err,
			)
			return err
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.write_skills_output",
		"duration", time.Since(startedAt),
		"main_path", mainPath,
		"main_content_length", len(mainContent),
		"patterns_count", len(patterns),
		"categories_count", len(summaryResult.CategorySummaries),
	)
	return nil
}

// GenerateAgentMetadata 渲染并写入 agent 元数据文件
func (w *SkillWriter) GenerateAgentMetadata(outputPath string, data interface{}) error {
	files, err := w.skillsLoader.RenderAgentMetadataFiles(data)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		outputFile := filepath.Join(outputPath, file.Path)
		if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(outputFile, []byte(file.Content), 0644); err != nil {
			return err
		}

		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.generate_agent_metadata",
			"path", outputFile,
			"content_length", len(file.Content),
		)
	}
	return nil
}

// GenerateReferenceFiles 生成所有参考文档
func (w *SkillWriter) GenerateReferenceFiles(ctx context.Context, outputPath string, summaries map[string]agent.CategorySummary, patterns []domain.Pattern, profile *domain.ProjectProfile, spec *domain.ProjectSpec, references ReferenceAvailability) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_reference_files",
		"output_path", outputPath,
		"summaries_count", len(summaries),
		"patterns_count", len(patterns),
	)

	refsPath := filepath.Join(outputPath, "references")
	if err := os.MkdirAll(refsPath, 0755); err != nil {
		return err
	}

	if err := w.generateProjectOverview(refsPath, profile); err != nil {
		return err
	}
	patternsPath := filepath.Join(refsPath, "patterns")
	examplesPath := filepath.Join(refsPath, "examples")

	if err := os.MkdirAll(patternsPath, 0755); err != nil {
		return err
	}
	if err := os.RemoveAll(examplesPath); err != nil {
		return err
	}

	categoriesWithPatterns := categoryNamesWithPatterns(patterns)
	if err := w.generateProfileReferenceFiles(refsPath, profile, categoriesWithPatterns); err != nil {
		return err
	}
	if spec != nil {
		if err := w.generateProjectSpec(refsPath, spec, profile, patterns, references); err != nil {
			return err
		}
	}

	for _, categoryName := range categoriesWithPatterns {
		summary := summaries[categoryName]
		if err := w.generateCategoryPattern(patternsPath, categoryName, summary, patterns, categoriesWithPatterns, profile.Language); err != nil {
			return err
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_reference_files",
		"duration", time.Since(startedAt),
		"refs_path", refsPath,
		"summaries_count", len(summaries),
	)
	return nil
}

func (w *SkillWriter) generateProjectSpec(refsPath string, spec *domain.ProjectSpec, profile *domain.ProjectProfile, patterns []domain.Pattern, references ReferenceAvailability) error {
	specPath := filepath.Join(refsPath, "project-spec.md")
	content, err := w.skillsLoader.RenderReferenceFile("project-spec", projectSpecTemplateData{
		ProjectSpec:      *spec,
		References:       references,
		SourceOfTruth:    []SourceOfTruthItem{},
		ValidationMatrix: validationMatrix(profile, patterns, w.skillsLoader.GetLocale()),
	})
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectSpecRenderFailed"), err)
	}
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_project_spec",
		"path", specPath,
		"content_length", len(content),
	)
	return nil
}

func (w *SkillWriter) generateProjectOverview(refsPath string, profile *domain.ProjectProfile) error {
	overviewPath := filepath.Join(refsPath, "project-overview.md")

	data := projectOverviewTemplateData{
		ProjectProfile:      *profile,
		OverviewReferences:  profileReferenceItems(profile, w.skillsLoader.GetLocale(), "./"),
		OverviewSummary:     projectOverviewSummary(profile, w.skillsLoader.GetLocale()),
		ArchitectureSummary: projectArchitectureSummary(profile, w.skillsLoader.GetLocale()),
		ValidationMatrix:    validationMatrix(profile, nil, w.skillsLoader.GetLocale()),
	}
	content, err := w.skillsLoader.RenderProjectOverview(data)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGenerateOverviewFailed"), err)
	}
	if err := os.WriteFile(overviewPath, []byte(content), 0644); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_project_overview",
		"path", overviewPath,
		"content_length", len(content),
		"placeholder", false,
	)
	return nil
}

func (w *SkillWriter) generateProfileReferenceFiles(refsPath string, profile *domain.ProjectProfile, categoriesWithPatterns []string) error {
	categorySet := make(map[string]bool, len(categoriesWithPatterns))
	for _, category := range categoriesWithPatterns {
		categorySet[category] = true
	}
	data := profileReferenceTemplateData{
		ProjectProfile:      *profile,
		HasBusinessPatterns: categorySet[string(domain.CategoryBusiness)],
		HasUtilityPatterns:  categorySet[string(domain.CategoryUtils)],
		CodeFenceLanguage:   codeFenceLanguage(profile.Language),
		BusinessMethodIndex: buildBusinessMethodIndex(profile.BusinessMethods, w.skillsLoader.GetLocale()),
	}
	files := []struct {
		templateName string
		outputName   string
		enabled      bool
	}{
		{templateName: "business-methods", outputName: "business-methods.md", enabled: len(profile.BusinessMethods) > 0},
		{templateName: "modules", outputName: "modules.md", enabled: len(profile.KeyModules) > 0},
		{templateName: "common-utils", outputName: "common-utils.md", enabled: len(profile.CommonUtils) > 0},
	}

	for _, file := range files {
		if !file.enabled {
			continue
		}
		content, err := w.skillsLoader.RenderReferenceFile(file.templateName, data)
		if err != nil {
			return fmt.Errorf("%s: reference=%s: %w", i18n.Get("InitGenerateOverviewFailed"), file.templateName, err)
		}
		outputPath := filepath.Join(refsPath, file.outputName)
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.generate_profile_reference",
			"reference", file.templateName,
			"path", outputPath,
			"content_length", len(content),
		)
	}
	return nil
}

func (w *SkillWriter) generateCategoryPattern(patternsPath, categoryName string, summary agent.CategorySummary, allPatterns []domain.Pattern, allCategories []string, language string) error {
	var categoryPatterns []domain.Pattern
	for _, p := range allPatterns {
		if string(p.Category) == categoryName {
			categoryPatterns = append(categoryPatterns, patternForTemplate(p))
		}
	}

	if len(categoryPatterns) == 0 {
		logger.Warn(i18n.Get("LoggerGeneratorSkipEmptyCategoryPattern"), "category", categoryName)
		return nil
	}

	if categoryName == string(domain.CategoryBusiness) {
		return w.generateSplitBusinessPatterns(patternsPath, summary, categoryPatterns, allPatterns, allCategories, language)
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
		"PatternObjects":    categoryPatterns,
		"ImportanceGroups":  patternImportanceGroups(categoryPatterns, w.skillsLoader.GetLocale()),
		"UsageScenes":       summary.UsageScenes,
		"Priority":          summary.Priority,
		"PatternCount":      len(categoryPatterns),
		"Confidence":        calculateCategoryConfidence(allPatterns, categoryName) * 100,
		"LastUpdated":       time.Now().Format("2006-01-02 15:04:05"),
		"BusinessMethods":   domain.ValidBusinessMethods(summary.BusinessMethods),
		"CodeFenceLanguage": codeFenceLanguage(language),
		"RelatedReferences": categoryReferenceLinks(categoryName, allCategories, w.skillsLoader.GetLocale(), "./"),
	}

	content, err := w.skillsLoader.RenderPattern(templateCategoryName(categoryName), data)
	if err != nil {
		return fmt.Errorf("%s: category=%s: %w", i18n.Get("GeneratorRenderPatternTemplateFailed"), categoryName, err)
	}
	outputPath := filepath.Join(patternsPath, categoryName+".md")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_category_pattern",
		"category", categoryName,
		"path", outputPath,
		"content_length", len(content),
		"patterns_count", len(categoryPatterns),
	)
	return nil
}

func (w *SkillWriter) generateSplitBusinessPatterns(patternsPath string, summary agent.CategorySummary, categoryPatterns, allPatterns []domain.Pattern, allCategories []string, language string) error {
	groups := businessPatternGroups(w.skillsLoader.GetLocale(), categoryPatterns)
	if err := os.MkdirAll(filepath.Join(patternsPath, string(domain.CategoryBusiness)), 0755); err != nil {
		return err
	}

	indexData := map[string]interface{}{
		"Category":          summary.Category,
		"Summary":           summary.Summary,
		"UsageScenes":       summary.UsageScenes,
		"Priority":          summary.Priority,
		"PatternCount":      len(categoryPatterns),
		"Confidence":        calculateCategoryConfidence(allPatterns, string(domain.CategoryBusiness)) * 100,
		"LastUpdated":       time.Now().Format("2006-01-02 15:04:05"),
		"Groups":            groups,
		"ImportanceGroups":  patternImportanceGroups(categoryPatterns, w.skillsLoader.GetLocale()),
		"RelatedReferences": businessPatternReferenceLinks(allCategories, w.skillsLoader.GetLocale(), "./"),
	}

	indexContent, err := w.skillsLoader.RenderRelative("project/references/patterns/business-index", indexData)
	if err != nil {
		return fmt.Errorf("%s: category=%s: %w", i18n.Get("GeneratorRenderPatternTemplateFailed"), domain.CategoryBusiness, err)
	}
	if err := os.WriteFile(filepath.Join(patternsPath, "business.md"), []byte(indexContent), 0644); err != nil {
		return err
	}

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
			"RelatedReferences": businessPatternReferenceLinks(allCategories, w.skillsLoader.GetLocale(), "../"),
		}
		content, err := w.skillsLoader.RenderRelative("project/references/patterns/business-detail", data)
		if err != nil {
			return fmt.Errorf("%s: category=%s group=%s: %w", i18n.Get("GeneratorRenderPatternTemplateFailed"), domain.CategoryBusiness, group.ID, err)
		}
		outputPath := filepath.Join(patternsPath, string(domain.CategoryBusiness), group.ID+".md")
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_split_business_patterns",
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

// SkillWriteOptions 封装 WriteSkillsOutput 所需的项目级参数
type SkillWriteOptions struct {
	SkillName           string
	ProjectName         string
	Language            string
	ProgramVersion      string
	SkillsTemplatesHash string
	SkipReferences      bool
	WorkflowReferences  []WorkflowReference
	ProjectRoot         string
}
