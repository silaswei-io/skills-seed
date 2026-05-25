package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

// GeneratorService 生成服务
type GeneratorService struct {
	patternRepo  domain.PatternRepository
	profileRepo  domain.ProjectProfileRepository
	skillsLoader *skills.Loader
	agent        agent.Agent // 添加AI Agent
	configRepo   config.Reader
}

type projectOverviewTemplateData struct {
	domain.ProjectProfile
	OverviewReferences []skills.ReferenceItem
}

type categoryReferenceMeta struct {
	Group       string
	Title       string
	Description string
}

// NewGeneratorService 创建生成服务
func NewGeneratorService(
	patternRepo domain.PatternRepository,
	profileRepo domain.ProjectProfileRepository,
	skillsLoader *skills.Loader,
	ag agent.Agent,
	configRepo config.Reader,
) *GeneratorService {
	return &GeneratorService{
		patternRepo:  patternRepo,
		profileRepo:  profileRepo,
		skillsLoader: skillsLoader,
		agent:        ag,
		configRepo:   configRepo,
	}
}

// GenerateSkills 生成 Skills 文件夹
func (s *GeneratorService) GenerateSkills(ctx context.Context, outputPath string) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_skills",
		"output_path", outputPath,
	)

	resolvedOutputPath, err := s.resolveOutputPath(outputPath)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.resolve_output_path",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	// 1. 获取所有模式
	patterns, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.load_patterns",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return err
	}

	if len(patterns) == 0 {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.generate_skills",
			"duration", time.Since(startedAt),
			"resolved_output_path", resolvedOutputPath,
			"patterns_count", 0,
			"skipped", true,
		)
		return nil
	}

	// 2. 序列化模式摘要为 JSON，不把代码示例直接发送给 Agent。
	patternsJSONBytes, err := json.MarshalIndent(summarizePatternsForAgent(patterns), "", "  ")
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.marshal_patterns",
			"duration", time.Since(startedAt),
			"patterns_count", len(patterns),
			"error", err,
		)
		return fmt.Errorf("%s: %w", i18n.Get("GeneratorMarshalPatternsFailed"), err)
	}
	patternsJSON := string(patternsJSONBytes)

	// 3. 记录现有 SKILL.md 路径，由 Agent 按需自行读取。
	existingSkillsPath := ""
	skillPath := filepath.Join(resolvedOutputPath, "SKILL.md")
	if _, err := os.Stat(skillPath); err == nil {
		existingSkillsPath = skillPath
	}

	// 4. 准备请求
	summaryReq := &agent.GenerateSkillsRequest{
		PatternsJSON:       patternsJSON,
		PatternsCount:      len(patterns),
		ExistingSkillsPath: existingSkillsPath,
		ProjectName:        s.configRepo.GetProjectConfig().Name,
		Language:           s.configRepo.GetProjectConfig().Language,
	}

	// 5. 调用 AI
	summaryResult, err := s.agent.GenerateSkillsSummary(ctx, summaryReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_summary",
			"duration", time.Since(startedAt),
			"patterns_count", len(patterns),
			"patterns_json_length", len(patternsJSON),
			"existing_skills_path", existingSkillsPath,
			"error", err,
		)
		return fmt.Errorf("%s: %w", i18n.Get("GeneratorGenerateSummaryFailed"), err)
	}

	// 6. 计算统计信息（用于 writeSkillsOutput）
	stats := s.calculateStats(patterns)

	if err := s.writeSkillsOutput(ctx, resolvedOutputPath, patterns, summaryResult, stats); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_skills",
			"duration", time.Since(startedAt),
			"patterns_count", len(patterns),
			"resolved_output_path", resolvedOutputPath,
			"error", err,
		)
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "generator.generate_skills",
		"duration", time.Since(startedAt),
		"patterns_count", len(patterns),
		"resolved_output_path", resolvedOutputPath,
		"categories_count", len(stats.ByCategory),
	)
	return nil
}

func summarizePatternsForAgent(patterns []domain.Pattern) []map[string]interface{} {
	summary := make([]map[string]interface{}, 0, len(patterns))
	for _, p := range patterns {
		summary = append(summary, map[string]interface{}{
			"id":              p.ID,
			"name":            p.Name,
			"category":        string(p.Category),
			"description":     p.Description,
			"rule":            p.Rule,
			"confidence":      p.Confidence,
			"frequency":       p.Frequency,
			"source":          string(p.Source),
			"business_method": p.BusinessMethod,
		})
	}
	return summary
}

func (s *GeneratorService) writeSkillsOutput(ctx context.Context, outputPath string, patterns []domain.Pattern, summaryResult *agent.GenerateSkillsResult, stats *Stats) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.write_skills_output",
		"output_path", outputPath,
		"patterns_count", len(patterns),
	)

	if len(patterns) == 0 {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "generator.write_skills_output",
			"duration", time.Since(startedAt),
			"patterns_count", 0,
			"skipped", true,
		)
		return nil
	}
	if summaryResult == nil {
		return fmt.Errorf("%s", i18n.Get("GeneratorGenerateSummaryFailed"))
	}

	// 确保分类摘要非 nil，避免模板侧判空复杂化
	if summaryResult.CategorySummaries == nil {
		summaryResult.CategorySummaries = map[string]agent.CategorySummary{}
	}
	summaryResult.CategorySummaries = s.ensureCategorySummaries(patterns, summaryResult.CategorySummaries)

	// 5. 确保输出目录存在
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.create_output_dir",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	projectConfig := s.configRepo.GetProjectConfig()
	skillsTemplatesHash := metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS))
	profile, err := s.loadProjectProfile(ctx)
	if err != nil {
		return err
	}
	profile = cleanProjectProfile(profile)

	// 准备模板数据
	data := map[string]interface{}{
		"ProgramVersion":      metadata.ProgramVersion,
		"SkillsTemplatesHash": skillsTemplatesHash,
		"ProjectName":         projectConfig.Name,
		"SkillName":           generatedSkillName(projectConfig.Name),
		"Language":            projectConfig.Language,
		"PatternCount":        len(patterns),
		"AvgConfidence":       stats.AvgConfidence * 100,
		"Categories":          len(summaryResult.CategorySummaries),
		"LastUpdated":         time.Now().Format("2006-01-02 15:04:05"),
		"CategorySummaries":   summaryResult.CategorySummaries,
		"KeyPatterns":         summaryResult.KeyPatterns,
		"BusinessRules":       summaryResult.BusinessRules,
		"BestPractices":       summaryResult.BestPractices,
		"CommonPatterns":      summaryResult.CommonPatterns,
		"STATS":               stats,
		"OverviewReferences":  profileReferenceItems(profile, s.skillsLoader.GetLocale(), "./references/"),
		"ReferenceGroups":     categoryReferenceGroups(patterns, s.skillsLoader.GetLocale()),
	}

	// 生成主 SKILL.md 文件
	mainContent, err := s.skillsLoader.Render("skill", data)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.render_skill_template",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return err
	}

	mainPath := filepath.Join(outputPath, "SKILL.md")
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.write_skill_file",
			"duration", time.Since(startedAt),
			"path", mainPath,
			"error", err,
		)
		return err
	}

	if err := s.generateAgentMetadata(outputPath, data); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_agent_metadata",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
	}

	// 7. 生成参考文档
	if err := s.generateReferenceFiles(ctx, outputPath, summaryResult.CategorySummaries, patterns, profile); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "generator.generate_reference_files",
			"duration", time.Since(startedAt),
			"output_path", outputPath,
			"error", err,
		)
		return err
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

func (s *GeneratorService) loadProjectProfile(ctx context.Context) (*domain.ProjectProfile, error) {
	if s.profileRepo == nil {
		return nil, fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
	}

	profile, err := s.profileRepo.Get(ctx)
	if err != nil {
		if errors.Is(err, profilestore.ErrProfileNotFound) {
			return nil, fmt.Errorf("%s", i18n.Get("GenerateProjectProfileMissing"))
		}
		return nil, err
	}
	return profile, nil
}

func (s *GeneratorService) generateAgentMetadata(outputPath string, data map[string]interface{}) error {
	files, err := s.skillsLoader.RenderAgentMetadataFiles(data)
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

func (s *GeneratorService) resolveOutputPath(outputPath string) (string, error) {
	projectRoot := ""
	if s.configRepo != nil {
		projectRoot = s.configRepo.GetProjectConfig().RootPath
	}
	return utils.ResolvePath(projectRoot, outputPath)
}

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
			summary.Patterns = patternNames(categoryPatterns)
		}
		if summary.Summary == "" {
			summary.Summary = fmt.Sprintf("%s 分类包含 %d 个项目特定模式：%s。", category, len(categoryPatterns), strings.Join(summary.Patterns, "、"))
		}
		summaries[category] = summary
	}

	return summaries
}

func patternNames(patterns []domain.Pattern) []string {
	names := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.Name != "" {
			names = append(names, pattern.Name)
		}
	}
	return names
}

func categoryNamesWithPatterns(patterns []domain.Pattern) []string {
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		category := string(pattern.Category)
		if category == "" {
			continue
		}
		seen[category] = true
	}

	categories := make([]string, 0, len(seen))
	for category := range seen {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
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

func patternForTemplate(pattern domain.Pattern) domain.Pattern {
	if !isUsableBusinessMethod(pattern.BusinessMethod) {
		pattern.BusinessMethod = nil
	}
	return pattern
}

func validBusinessMethods(methods []*domain.BusinessMethod) []*domain.BusinessMethod {
	valid := make([]*domain.BusinessMethod, 0, len(methods))
	for _, method := range methods {
		if isUsableBusinessMethod(method) {
			valid = append(valid, method)
		}
	}
	return valid
}

func isUsableBusinessMethod(method *domain.BusinessMethod) bool {
	return method != nil && strings.TrimSpace(method.Name) != ""
}

func cleanProjectProfile(profile *domain.ProjectProfile) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}

	cleaned := *profile

	cleaned.KeyModules = make([]domain.ModuleInfo, 0, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		module.KeyMethods = filterGeneratedPlaceholders(module.KeyMethods)
		cleaned.KeyModules = append(cleaned.KeyModules, module)
	}

	cleaned.BusinessMethods = make([]domain.BusinessMethod, 0, len(profile.BusinessMethods))
	for _, method := range profile.BusinessMethods {
		if strings.TrimSpace(method.Name) != "" {
			cleaned.BusinessMethods = append(cleaned.BusinessMethods, method)
		}
	}

	cleaned.CommonUtils = make([]domain.UtilityFunction, 0, len(profile.CommonUtils))
	for _, utility := range profile.CommonUtils {
		if strings.TrimSpace(utility.Name) != "" {
			cleaned.CommonUtils = append(cleaned.CommonUtils, utility)
		}
	}

	return &cleaned
}

func filterGeneratedPlaceholders(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(strings.ToUpper(trimmed), "TODO") {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
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

// generateReferenceFiles 生成所有参考文档
func (s *GeneratorService) generateReferenceFiles(ctx context.Context, outputPath string, summaries map[string]agent.CategorySummary, patterns []domain.Pattern, profile *domain.ProjectProfile) error {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "generator.generate_reference_files",
		"output_path", outputPath,
		"summaries_count", len(summaries),
		"patterns_count", len(patterns),
	)

	// 确保 references 目录存在
	refsPath := filepath.Join(outputPath, "references")
	if err := os.MkdirAll(refsPath, 0755); err != nil {
		return err
	}

	// 1. 生成 project-overview.md（如果不存在）
	if err := s.generateProjectOverview(refsPath, profile); err != nil {
		return err
	}
	if err := s.generateProfileReferenceFiles(refsPath, profile); err != nil {
		return err
	}

	// 2. 创建 patterns 目录，并清理旧版 examples 目录
	patternsPath := filepath.Join(refsPath, "patterns")
	examplesPath := filepath.Join(refsPath, "examples")

	if err := os.MkdirAll(patternsPath, 0755); err != nil {
		return err
	}
	if err := os.RemoveAll(examplesPath); err != nil {
		return err
	}

	// 3. 为每个实际存在的分类生成 patterns/{category}.md
	for _, categoryName := range categoryNamesWithPatterns(patterns) {
		summary := summaries[categoryName]
		if err := s.generateCategoryPattern(patternsPath, categoryName, summary, patterns); err != nil {
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

// generateProjectOverview 生成项目概览
func (s *GeneratorService) generateProjectOverview(refsPath string, profile *domain.ProjectProfile) error {
	overviewPath := filepath.Join(refsPath, "project-overview.md")

	data := projectOverviewTemplateData{
		ProjectProfile:     *profile,
		OverviewReferences: profileReferenceItems(profile, s.skillsLoader.GetLocale(), "./"),
	}
	content, err := s.skillsLoader.RenderProjectOverview(data)
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

func (s *GeneratorService) generateProfileReferenceFiles(refsPath string, profile *domain.ProjectProfile) error {
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
		content, err := s.skillsLoader.RenderReferenceFile(file.templateName, profile)
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

// generateCategoryPattern 生成分类模式文档到 patterns/ 目录
func (s *GeneratorService) generateCategoryPattern(patternsPath, categoryName string, summary agent.CategorySummary, allPatterns []domain.Pattern) error {
	// 筛选出当前分类的完整模式对象
	var categoryPatterns []domain.Pattern
	for _, p := range allPatterns {
		if string(p.Category) == categoryName {
			categoryPatterns = append(categoryPatterns, patternForTemplate(p))
		}
	}

	// 调试日志
	if len(categoryPatterns) == 0 {
		logger.Warn(i18n.Get("LoggerGeneratorSkipEmptyCategoryPattern"), "category", categoryName)
		return nil
	}

	logger.Diagnostic(i18n.Get("LoggerGeneratorGeneratingPatternFile"),
		"category", categoryName,
		"patternObjects", len(categoryPatterns),
		"summaryPatterns", len(summary.Patterns),
		"firstPatternGoodExample", len(categoryPatterns[0].GoodExample),
	)

	// 准备模板数据
	data := map[string]interface{}{
		"Category":        summary.Category,
		"Summary":         summary.Summary,
		"Patterns":        summary.Patterns, // 模式名称列表
		"PatternObjects":  categoryPatterns, // 完整的模式对象（包含good_example等）
		"UsageScenes":     summary.UsageScenes,
		"Priority":        summary.Priority,
		"PatternCount":    len(categoryPatterns),
		"Confidence":      s.calculateCategoryConfidence(allPatterns, categoryName) * 100,
		"LastUpdated":     time.Now().Format("2006-01-02 15:04:05"),
		"BusinessMethods": validBusinessMethods(summary.BusinessMethods),
	}

	// 渲染模板
	content, err := s.skillsLoader.RenderPattern(templateCategoryName(categoryName), data)
	if err != nil {
		return fmt.Errorf("%s: category=%s: %w", i18n.Get("GeneratorRenderPatternTemplateFailed"), categoryName, err)
	}

	// 写入文件到 patterns/{category}.md
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

// Stats 统计信息
type Stats struct {
	Total          int
	AvgConfidence  float64
	HighConfidence []domain.Pattern
	Frequent       []domain.Pattern
	ByCategory     map[string][]domain.Pattern
}

// calculateStats 计算统计信息
func (s *GeneratorService) calculateStats(patterns []domain.Pattern) *Stats {
	stats := &Stats{
		Total:      len(patterns),
		ByCategory: make(map[string][]domain.Pattern),
	}

	if len(patterns) == 0 {
		return stats
	}

	// 计算平均置信度
	var totalConfidence float64
	for _, p := range patterns {
		totalConfidence += p.Confidence
	}
	stats.AvgConfidence = totalConfidence / float64(len(patterns))

	// 按分类统计
	for _, p := range patterns {
		category := string(p.Category)
		stats.ByCategory[category] = append(stats.ByCategory[category], p)
	}

	// 筛选高置信度模式（>0.8）
	for _, p := range patterns {
		if p.Confidence > 0.8 {
			stats.HighConfidence = append(stats.HighConfidence, p)
		}
	}

	// 筛选频繁模式（>3次）
	for _, p := range patterns {
		if p.Frequency > 3 {
			stats.Frequent = append(stats.Frequent, p)
		}
	}

	return stats
}

// calculateCategoryConfidence 计算特定分类的平均置信度
func (s *GeneratorService) calculateCategoryConfidence(patterns []domain.Pattern, category string) float64 {
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

func templateCategoryName(category string) string {
	switch category {
	case string(domain.CategoryError):
		return "error-handling"
	default:
		return category
	}
}
