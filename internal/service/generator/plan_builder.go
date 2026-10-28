package generator

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
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
	profileCopy := *snapshot.RenderProfile
	profileCopy.BusinessMethods = reviewedBusinessMethods(profileCopy.BusinessMethods, snapshot.Patterns)
	profile := &profileCopy
	snapshot.RenderProfile = profile
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
	hasKnowledge := !opts.ResourceOnly
	references := referenceAvailability(profile, patterns, hasKnowledge)
	triggerDescription := skillTriggerDescription(templateProjectName, locale, profile)

	// 准备模板数据
	data := skillTemplateData{
		ProgramVersion:      opts.ProgramVersion,
		SkillsTemplatesHash: opts.SkillsTemplatesHash,
		ProjectName:         templateProjectName,
		SkillName:           skillName,
		SkillDescription:    triggerDescription,
		Language:            templateLanguage,
		HasKnowledge:        hasKnowledge,
		HasNavigation:       opts.Navigation != nil && len(opts.Navigation.Files)+len(opts.Navigation.Unindexed)+len(opts.Navigation.AuthorityPaths) > 0,
		HasPatterns:         len(patterns) > 0,
		PatternCount:        len(patterns),
		LastUpdated:         knowledgeUpdatedAt(profile, patterns),
		References:          references,
		OverviewReferences:  conditionalProfileReferenceItems(profile, locale, "./references/", references.Enabled),
		ReferenceGroups:     conditionalCategoryReferenceGroups(patterns, locale, references.Enabled),
		WorkflowReferences:  opts.WorkflowReferences,
		RuleReferences:      opts.RuleReferences,
		CommandRules:        commandPolicyRules(profile.EngineeringRules),
	}

	p := skillgen.NewPlan(outputPath)
	p.AddFile("SKILL.md", skillgen.CatalogTemplate, "project-skill", data)
	p.AgentMetadataData = data
	if opts.Navigation != nil && len(opts.Navigation.Files)+len(opts.Navigation.Unindexed)+len(opts.Navigation.AuthorityPaths) > 0 {
		var methods []domain.BusinessMethod
		if hasKnowledge {
			methods = profile.BusinessMethods
		}
		appendNavigation(p, opts.Navigation, methods)
	}

	if hasKnowledge {
		if err := b.appendReferenceFiles(p, summaryResult.CategorySummaries, snapshot, references, opts.RuleReferences, opts.Navigation); err != nil {
			logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
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

func (b *planBuilder) appendReferenceFiles(p *skillgen.Plan, summaries map[string]categorySummary, snapshot verifiedKnowledgeSnapshot, references ReferenceAvailability, ruleReferences []RuleReference, index *codegraph.Index) error {
	patterns := snapshot.Patterns
	profile := snapshot.RenderProfile
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_reference_files",
		"summaries_count", len(summaries),
		"patterns_count", len(patterns),
	)

	if references.ProjectOverview {
		if err := b.appendProjectOverview(p, profile); err != nil {
			return err
		}
	}

	categoriesWithPatterns := categoryNamesWithPatterns(patterns)
	if err := b.appendProfileReferenceFiles(p, profile, categoriesWithPatterns); err != nil {
		return err
	}
	if snapshot.Spec != nil {
		var authorityPaths []string
		if index != nil {
			authorityPaths = index.AuthorityPaths
		}
		if err := b.appendProjectSpec(p, snapshot.Spec, references, ruleReferences, authorityPaths); err != nil {
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

func commandPolicyRules(rules []domain.EngineeringRule) []domain.EngineeringRule {
	out := make([]domain.EngineeringRule, 0, len(rules))
	for _, rule := range rules {
		if rule.CommandPolicy != "" {
			out = append(out, rule)
		}
	}
	return out
}

func (b *planBuilder) appendProjectSpec(p *skillgen.Plan, spec *domain.ProjectSpec, references ReferenceAvailability, ruleReferences []RuleReference, authorityPaths []string) error {
	filtered := *spec
	filtered.EngineeringRules = nonResourceEngineeringRules(spec.EngineeringRules)
	p.AddFile("references/project-spec.md", skillgen.ReferenceTemplate, "project-spec", projectSpecTemplateData{
		ProjectSpec:    filtered,
		References:     references,
		RuleReferences: projectSpecRuleReferences(ruleReferences),
		AuthorityPaths: authorityPaths,
	})
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.append_project_spec",
	)
	return nil
}

func projectSpecRuleReferences(refs []RuleReference) []RuleReference {
	out := make([]RuleReference, 0, len(refs))
	for _, ref := range refs {
		ref.Path = filepath.ToSlash(filepath.Join("..", strings.TrimPrefix(filepath.ToSlash(ref.Path), "./")))
		out = append(out, ref)
	}
	return out
}

func nonResourceEngineeringRules(rules []domain.EngineeringRule) []domain.EngineeringRule {
	out := make([]domain.EngineeringRule, 0, len(rules))
	for _, rule := range rules {
		if strings.HasPrefix(filepath.ToSlash(rule.Source), ".skills-seed/rules/") {
			continue
		}
		out = append(out, rule)
	}
	return out
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
	if len(data.BusinessMethodIndex.Groups) > 0 {
		for _, group := range data.BusinessMethodIndex.Groups {
			for _, method := range group.Methods {
				p.AddFile(filepath.Join("references", "business-methods", filepath.Base(method.ReferencePath)), skillgen.RelativeTemplate, "project/references/business-method-detail", struct {
					Method            businessMethodView
					CodeFenceLanguage string
				}{Method: method, CodeFenceLanguage: data.CodeFenceLanguage})
			}
		}
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
		return b.appendSplitBusinessPatterns(p, categoryPatterns, allCategories)
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
		ClaimGroups:       knowledge.ClaimGroups(categoryPatterns, b.skillsLoader.GetLocale()),
		PatternCount:      len(categoryPatterns),
		LastUpdated:       knowledgeUpdatedAt(nil, categoryPatterns),
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

func (b *planBuilder) appendSplitBusinessPatterns(p *skillgen.Plan, categoryPatterns []domain.Pattern, allCategories []string) error {
	items := businessPatternItems(categoryPatterns)
	groups, singles := groupBusinessResponsibilities(items)
	indexData := businessIndexTemplateData{
		PatternCount:      len(categoryPatterns),
		LastUpdated:       knowledgeUpdatedAt(nil, categoryPatterns),
		Groups:            groups,
		Singles:           singles,
		RelatedReferences: businessPatternReferenceLinks(allCategories, b.skillsLoader.GetLocale(), "./"),
	}

	p.AddFile(
		filepath.Join("references", "patterns", "business.md"),
		skillgen.RelativeTemplate,
		"project/references/patterns/business-index",
		indexData,
	)
	for _, group := range groups {
		p.AddFile(filepath.Join("references", "patterns", "business", "groups", filepath.Base(group.ReferencePath)),
			skillgen.RelativeTemplate, "project/references/patterns/business-group", group)
	}

	for _, pattern := range items {
		data := businessDetailTemplateData{
			Category:          string(domain.CategoryBusiness),
			GroupTitle:        pattern.Name,
			PatternObjects:    []patternRenderModel{pattern},
			LastUpdated:       renderedKnowledgeUpdatedAt([]patternRenderModel{pattern}),
			RelatedReferences: businessPatternReferenceLinks(allCategories, b.skillsLoader.GetLocale(), "../../"),
		}
		p.AddFile(businessPatternItemPath(pattern), skillgen.RelativeTemplate, "project/references/patterns/business-detail", data)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.append_split_business_patterns",
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
	Navigation          *codegraph.Index // 仅用于结构导航，不解释业务含义。
	SkillName           string
	ProjectName         string
	Language            string
	ProgramVersion      string
	SkillsTemplatesHash string
	ResourceOnly        bool
	WorkflowReferences  []WorkflowReference
	RuleReferences      []RuleReference
}

func knowledgeUpdatedAt(profile *domain.ProjectProfile, patterns []domain.Pattern) string {
	if profile != nil && profile.GeneratedAt != "" {
		return profile.GeneratedAt
	}
	latest := time.Time{}
	for _, pattern := range patterns {
		if pattern.UpdatedAt.After(latest) {
			latest = pattern.UpdatedAt
		}
	}
	if latest.IsZero() {
		return "-"
	}
	return latest.Format("2006-01-02 15:04:05")
}

func renderedKnowledgeUpdatedAt(patterns []patternRenderModel) string {
	values := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		values = append(values, pattern.Pattern)
	}
	return knowledgeUpdatedAt(nil, values)
}
