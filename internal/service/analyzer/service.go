// Package analyzer 提供代码分析服务
//
// 本包实现代码模式分析、项目结构分析和代码库分析功能
//   - AnalyzePatterns: 分析代码模式，发现问题
//   - AnalyzeProject: 分析项目结构和特点
//   - AnalyzeCurrentCodebase: 分析现有代码库（不依赖 commit 历史）
//
// 服务职责
//   - 调用 AI Agent 进行代码分析
//   - 转换领域模型和 Agent 模型
//   - 包装错误为领域错误
package analyzer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/snapshotflow"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

// AnalyzerService 代码分析服务
// 职责：分析代码、提取模式、分析项目结构
type AnalyzerService struct {
	agent               agent.Agent
	configRepo          config.Reader
	structuralCollector structuralCollector
}

// FileSelectionContextRequest 描述 AI 文件筛选前的结构化候选线索请求。
type FileSelectionContextRequest struct {
	ProjectName    string
	Language       string
	FocusPaths     []string
	CandidateCount int
	UserContext    string
}

func (r FileSelectionContextRequest) Purpose() string {
	var b strings.Builder
	b.WriteString("pre-filter AI file analysis candidates before reading the full candidate list")
	if r.CandidateCount > 0 {
		b.WriteString("; local candidate count: ")
		b.WriteString(strconv.Itoa(r.CandidateCount))
	}
	if strings.TrimSpace(r.UserContext) != "" {
		b.WriteString("; user guidance is present")
	}
	return b.String()
}

// NewAnalyzerService 创建分析服务
func NewAnalyzerService(ag agent.Agent, configRepo config.Reader) *AnalyzerService {
	svc := &AnalyzerService{
		agent:      ag,
		configRepo: configRepo,
	}
	if configRepo != nil {
		cfg := configRepo.GetCurrentLearningConfig().Structural
		if cfg.Enabled {
			svc.structuralCollector = newStructuralCollector(cfg)
		}
	}
	return svc
}

func (s *AnalyzerService) collectStructuralContext(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	if s.configRepo == nil || s.structuralCollector == nil || projectRoot == "" {
		return "", nil
	}

	cfg := s.configRepo.GetCurrentLearningConfig().Structural
	if !cfg.Enabled || len(req.SeedPaths) == 0 {
		return "", nil
	}

	collector := s.structuralCollector
	if policyAware, ok := collector.(*renderedStructuralCollector); ok {
		collector = policyAware.withPolicy(fileanalysis.NewConfiguredSelectionPolicy(s.configRepo, projectRoot))
	}
	contextText, err := collector.Collect(ctx, projectRoot, req)
	if err == nil {
		return contextText, nil
	}

	logger.Warn(i18n.Get("AnalyzerStructuralCollectFailed"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
		"operation", "analyzer.structural_collect",
		"project_root", projectRoot,
		"error", err,
	)
	return "", nil
}

// CollectFileSelectionContext 在 AI 文件筛选前收集结构化候选线索。
// 该阶段只使用 CodeGraph/auto provider，避免 tree-sitter 在没有明确 seed 时全仓扫描。
func (s *AnalyzerService) CollectFileSelectionContext(ctx context.Context, projectRoot string, req FileSelectionContextRequest) string {
	if s == nil || s.configRepo == nil || projectRoot == "" {
		return ""
	}
	cfg := s.configRepo.GetCurrentLearningConfig().Structural
	if !cfg.Enabled {
		return ""
	}
	provider := config.NormalizeStructuralProvider(string(cfg.Provider))
	if provider == config.StructuralProviderTreeSitter {
		return ""
	}

	data, err := newCodeGraphProvider(cfg).Collect(ctx, projectRoot, structuralContextRequest{
		ProjectName: req.ProjectName,
		Language:    req.Language,
		Purpose:     req.Purpose(),
		FocusPaths:  req.FocusPaths,
		SeedPaths:   req.FocusPaths,
	})
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.file_selection_structural_context",
			"project_root", projectRoot,
			"candidate_count", req.CandidateCount,
			"error", err,
		)
		return ""
	}
	return structuralRenderer{}.Render(data, cfg.MaxSymbols)
}

func structuralSeedPaths(focusPaths []string, sampleFiles []agent.SampleFile, diffFiles []agent.DiffFileRef, mainFiles []string) []string {
	seeds := make([]string, 0, len(focusPaths)+len(sampleFiles)+len(diffFiles)+len(mainFiles))
	seen := make(map[string]bool)
	add := func(path string) {
		path = strings.TrimSpace(filepath.ToSlash(path))
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		seeds = append(seeds, path)
	}
	for _, path := range focusPaths {
		add(path)
	}
	for _, file := range sampleFiles {
		add(file.Path)
	}
	for _, file := range diffFiles {
		add(file.Path)
	}
	for _, path := range mainFiles {
		add(path)
	}
	return seeds
}

// AnalyzePatternsRequest 分析模式请求
type AnalyzePatternsRequest struct {
	Files         []domain.FileInfo
	KnownPatterns []domain.Pattern
	RecentCommits []domain.CommitInfo
}

// AnalyzePatternsResult 分析模式结果
type AnalyzePatternsResult struct {
	Issues      []domain.Issue
	Suggestions []string
	Confidence  float64
}

// AnalyzePatterns 分析代码模式
// 从文件和提交中分析编码模式，发现潜在问题
func (s *AnalyzerService) AnalyzePatterns(ctx context.Context, req *AnalyzePatternsRequest) (*AnalyzePatternsResult, error) {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "analyzer.analyze_patterns",
		"files_count", len(req.Files),
		"known_patterns_count", len(req.KnownPatterns),
		"recent_commits_count", len(req.RecentCommits),
	)

	projectContext := agent.ProjectContext{
		Name:     "project",
		Language: "unknown",
	}
	if s.configRepo != nil {
		projectConfig := s.configRepo.GetProjectConfig()
		if projectConfig.Name != "" {
			projectContext.Name = projectConfig.Name
		}
		if projectConfig.Language != "" {
			projectContext.Language = projectConfig.Language
		}
	}

	// 调用 Agent
	analyzeReq := &agent.AnalyzeRequest{
		Files:         req.Files,
		Patterns:      req.KnownPatterns,
		RecentCommits: req.RecentCommits,
		Context:       projectContext,
	}

	result, err := s.agent.AnalyzeCode(ctx, analyzeReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_patterns",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, domain.NewDomainError(
			domain.ErrAIService,
			i18n.Get("AnalyzerAnalyzePatternsFailed"),
			err,
		)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_patterns",
		"duration", time.Since(startedAt),
		"issues_count", len(result.Issues),
		"suggestions_count", len(result.Suggestions),
		"confidence", result.Confidence,
	)

	return &AnalyzePatternsResult{
		Issues:      result.Issues,
		Suggestions: result.Suggestions,
		Confidence:  result.Confidence,
	}, nil
}

// AnalyzeProjectRequest 项目分析请求
type AnalyzeProjectRequest struct {
	ProjectName         string
	RootPath            string
	Language            string
	Structure           string
	StructuralContext   string
	ReadmePath          string
	MainFiles           []string
	ExistingProfileJSON string
	FocusPaths          []string
	UserContext         string
}

// AnalyzeProjectResult 项目分析结果
type AnalyzeProjectResult struct {
	Language           string
	Frameworks         []string
	Architecture       string
	Structure          string
	Layers             []domain.ArchitectureLayer
	DependencyGraph    string
	DataFlow           string
	FrameworkPatterns  []string
	CommonUtils        []domain.UtilityFunction
	KeyModules         []domain.ModuleInfo
	ConfigPatterns     []string
	Dependencies       []string
	BusinessMethods    []domain.BusinessMethod
	ValidationCommands []domain.ValidationCommand
	Summary            string
}

// AnalyzeProject 分析项目结构和特点
func (s *AnalyzerService) AnalyzeProject(ctx context.Context, req *AnalyzeProjectRequest) (*AnalyzeProjectResult, error) {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "analyzer.analyze_project",
		"project_name", req.ProjectName,
		"root_path", req.RootPath,
		"language", req.Language,
		"structure_length", len(req.Structure),
		"readme_path", req.ReadmePath,
		"main_files_count", len(req.MainFiles),
		"has_existing_profile", req.ExistingProfileJSON != "",
		"existing_profile_bytes", len(req.ExistingProfileJSON),
		"focus_paths_count", len(req.FocusPaths),
	)

	structuralContext, err := s.collectStructuralContext(ctx, req.RootPath, structuralContextRequest{
		ProjectName: req.ProjectName,
		Language:    req.Language,
		Purpose:     "project profile analysis",
		FocusPaths:  req.FocusPaths,
		SeedPaths:   structuralSeedPaths(req.FocusPaths, nil, nil, req.MainFiles),
	})
	if err != nil {
		return nil, err
	}
	if req.StructuralContext != "" {
		structuralContext = req.StructuralContext
	}

	agentReq := &agent.AnalyzeProjectRequest{
		ProjectName:         req.ProjectName,
		RootPath:            req.RootPath,
		Language:            req.Language,
		Structure:           req.Structure,
		StructuralContext:   structuralContext,
		ReadmePath:          req.ReadmePath,
		MainFiles:           req.MainFiles,
		ExistingProfileJSON: req.ExistingProfileJSON,
		FocusPaths:          req.FocusPaths,
		UserContext:         req.UserContext,
	}

	result, err := s.agent.AnalyzeProject(ctx, agentReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_project",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, domain.NewDomainError(
			domain.ErrAIService,
			i18n.Get("AnalyzerAnalyzeProjectFailed"),
			err,
		)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_project",
		"duration", time.Since(startedAt),
		"frameworks_count", len(result.Frameworks),
		"dependencies_count", len(result.Dependencies),
		"business_methods_count", len(result.BusinessMethods),
		"key_modules_count", len(result.KeyModules),
	)

	return &AnalyzeProjectResult{
		Language:           result.Language,
		Frameworks:         result.Frameworks,
		Architecture:       result.Architecture,
		Structure:          result.Structure,
		Layers:             result.Layers,
		DependencyGraph:    result.DependencyGraph,
		DataFlow:           result.DataFlow,
		FrameworkPatterns:  result.FrameworkPatterns,
		CommonUtils:        result.CommonUtils,
		KeyModules:         result.KeyModules,
		ConfigPatterns:     result.ConfigPatterns,
		Dependencies:       result.Dependencies,
		BusinessMethods:    result.BusinessMethods,
		ValidationCommands: result.ValidationCommands,
		Summary:            result.Summary,
	}, nil
}

// AnalyzeCurrentCodebaseRequest 当前代码库分析请求
type AnalyzeCurrentCodebaseRequest struct {
	ProjectName        string
	RootPath           string
	Language           string
	LearningMode       config.LearningMode
	RuntimeLabel       string
	AnalysisUnit       domain.AnalysisUnit
	FocusPaths         []string
	Structure          string
	StructuralContext  string
	MainFiles          []string
	SampleFiles        []agent.SampleFile
	DiffFiles          []agent.DiffFileRef
	KnownPatternsJSON  string
	KnownPatternsCount int
	UserContext        string
	ChangeProfile      string
	LearningBudget     config.LearningBudget
}

// AnalyzeCurrentCodebaseResult 当前代码库分析结果
type AnalyzeCurrentCodebaseResult struct {
	Patterns                  []domain.Pattern
	ProfileDelta              domain.ProjectProfileDelta
	ProfileRefreshRecommended agent.ProfileRefreshRecommendation
}

type AnalyzeCurrentCodebaseBatchUnit struct {
	AnalysisUnit  domain.AnalysisUnit
	FocusAbsPaths []string
}

type AnalyzeCurrentCodebaseBatchOptions struct {
	RuntimeLabel   string
	LearningMode   config.LearningMode
	ChangeProfile  string
	LearningBudget config.LearningBudget
	RunContext     *CodebaseRunContext
	Units          []AnalyzeCurrentCodebaseBatchUnit
}

type AnalyzeCurrentCodebaseUnitResult struct {
	AnalysisUnit              domain.AnalysisUnit
	Patterns                  []domain.Pattern
	ProfileDelta              domain.ProjectProfileDelta
	ProfileRefreshRecommended agent.ProfileRefreshRecommendation
}

type AnalyzeCurrentCodebaseBatchResult struct {
	Units []AnalyzeCurrentCodebaseUnitResult
}

// PlanAnalysisUnitsRequest 请求按业务能力规划当前待学习文件。
type PlanAnalysisUnitsRequest struct {
	ProjectName       string
	RootPath          string
	Language          string
	LearningMode      config.LearningMode
	LearningScope     config.LearningScope
	FocusPaths        []string
	StructuralContext string
	UserContext       string
}

// PlanAnalysisUnits 按业务能力拆分当前待学习文件。
func (s *AnalyzerService) PlanAnalysisUnits(ctx context.Context, req *PlanAnalysisUnitsRequest) ([]domain.AnalysisUnit, error) {
	structuralContext := req.StructuralContext
	if structuralContext == "" {
		var err error
		structuralContext, err = s.collectStructuralContext(ctx, req.RootPath, structuralContextRequest{
			ProjectName: req.ProjectName,
			Language:    req.Language,
			Purpose:     "current codebase analysis planning",
			FocusPaths:  req.FocusPaths,
			SeedPaths:   req.FocusPaths,
		})
		if err != nil {
			return nil, err
		}
	}
	agentReq := &agent.PlanAnalysisUnitsRequest{
		ProjectName:       req.ProjectName,
		RootPath:          req.RootPath,
		Language:          req.Language,
		LearningMode:      req.LearningMode,
		LearningScope:     req.LearningScope,
		FocusPaths:        req.FocusPaths,
		StructuralContext: structuralContext,
		UserContext:       req.UserContext,
	}
	result, err := s.agent.PlanAnalysisUnits(ctx, agentReq)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}
	return result.Units, nil
}

// AnalyzeCurrentCodebase 分析当前代码库
// 从现有代码中提取初始模式（不依赖 commit 历史）
func (s *AnalyzerService) AnalyzeCurrentCodebase(ctx context.Context, req *AnalyzeCurrentCodebaseRequest) (*AnalyzeCurrentCodebaseResult, error) {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "analyzer.analyze_current_codebase",
		"project_name", req.ProjectName,
		"root_path", req.RootPath,
		"language", req.Language,
		"structure_length", len(req.Structure),
		"main_files_count", len(req.MainFiles),
		"sample_files_count", len(req.SampleFiles),
	)

	structuralContext, err := s.collectStructuralContext(ctx, req.RootPath, structuralContextRequest{
		ProjectName: req.ProjectName,
		Language:    req.Language,
		Purpose:     "current codebase pattern extraction",
		FocusPaths:  req.FocusPaths,
		SeedPaths:   structuralSeedPaths(nil, req.SampleFiles, req.DiffFiles, req.MainFiles),
	})
	if err != nil {
		return nil, err
	}
	if req.StructuralContext != "" {
		structuralContext = req.StructuralContext
	}

	agentReq := &agent.AnalyzeCurrentCodebaseRequest{
		ProjectName:       req.ProjectName,
		RootPath:          req.RootPath,
		Language:          req.Language,
		LearningMode:      req.LearningMode,
		RuntimeLabel:      req.RuntimeLabel,
		AnalysisUnit:      req.AnalysisUnit,
		FocusPaths:        req.FocusPaths,
		Structure:         req.Structure,
		StructuralContext: structuralContext,
		MainFiles:         req.MainFiles,
		SampleFiles:       req.SampleFiles,
		DiffFiles:         req.DiffFiles,
		UserContext:       req.UserContext,
		ChangeProfile:     req.ChangeProfile,
		LearningBudget:    req.LearningBudget,
	}

	result, err := s.agent.AnalyzeCurrentCodebase(ctx, agentReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_current_codebase",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, domain.NewDomainError(
			domain.ErrAIService,
			i18n.Get("AnalyzerAnalyzeCodebaseFailed"),
			err,
		)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_current_codebase",
		"duration", time.Since(startedAt),
		"patterns_count", len(result.Patterns),
		"profile_delta", !result.ProfileDelta.IsZero(),
		"profile_refresh_recommended", result.ProfileRefreshRecommended.Needed,
	)

	return &AnalyzeCurrentCodebaseResult{
		Patterns:                  result.Patterns,
		ProfileDelta:              result.ProfileDelta,
		ProfileRefreshRecommended: result.ProfileRefreshRecommended,
	}, nil
}

func (s *AnalyzerService) AnalyzeCurrentCodebaseBatch(ctx context.Context, projectRoot, projectName, language string, opts AnalyzeCurrentCodebaseBatchOptions) (*AnalyzeCurrentCodebaseBatchResult, error) {
	startedAt := time.Now()
	runContext := opts.RunContext
	if runContext == nil {
		var err error
		runContext, err = s.BuildCodebaseRunContext(ctx, projectRoot, language, AnalyzeCodebaseOptions{UseSnapshotDiffs: true})
		if err != nil {
			return nil, err
		}
	}

	units := make([]agent.AnalyzeCurrentCodebaseBatchUnit, 0, len(opts.Units))
	unitByID := make(map[string]domain.AnalysisUnit, len(opts.Units))
	for _, unit := range opts.Units {
		focusPaths := utils.RelativePaths(projectRoot, unit.FocusAbsPaths)
		units = append(units, agent.AnalyzeCurrentCodebaseBatchUnit{
			AnalysisUnit: unit.AnalysisUnit,
			FocusPaths:   focusPaths,
			SampleFiles:  filterSampleFilesByFocus(runContext.SampleFiles, focusPaths),
			DiffFiles:    filterDiffFilesByFocus(runContext.DiffFiles, focusPaths),
		})
		unitByID[unit.AnalysisUnit.ID] = unit.AnalysisUnit
	}

	agentReq := &agent.AnalyzeCurrentCodebaseBatchRequest{
		ProjectName:       projectName,
		RootPath:          projectRoot,
		Language:          language,
		LearningMode:      opts.LearningMode,
		RuntimeLabel:      opts.RuntimeLabel,
		Units:             units,
		Structure:         runContext.ProjectStructure,
		MainFiles:         append([]string(nil), runContext.MainFiles...),
		UserContext:       runtimecontext.UserContext(ctx),
		ChangeProfile:     opts.ChangeProfile,
		LearningBudget:    opts.LearningBudget,
		StructuralContext: "",
	}

	structuralContext, err := s.collectStructuralContext(ctx, projectRoot, structuralContextRequest{
		ProjectName: projectName,
		Language:    language,
		Purpose:     "current codebase batch pattern extraction",
		FocusPaths:  batchFocusPaths(units),
		SeedPaths:   batchSeedPaths(units, runContext.MainFiles),
	})
	if err != nil {
		return nil, err
	}
	agentReq.StructuralContext = structuralContext

	result, err := s.agent.AnalyzeCurrentCodebaseBatch(ctx, agentReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_current_codebase_batch",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}

	out := make([]AnalyzeCurrentCodebaseUnitResult, 0, len(result.Units))
	for _, unitResult := range result.Units {
		unit := unitByID[unitResult.UnitID]
		if unit.ID == "" && len(opts.Units) == 1 {
			unit = opts.Units[0].AnalysisUnit
		}
		patterns := unitResult.Patterns
		for i := range patterns {
			if patterns[i].AnalysisUnitID == "" {
				patterns[i].AnalysisUnitID = unit.ID
			}
			if patterns[i].AnalysisUnitName == "" {
				patterns[i].AnalysisUnitName = unit.Name
			}
		}
		out = append(out, AnalyzeCurrentCodebaseUnitResult{
			AnalysisUnit:              unit,
			Patterns:                  patterns,
			ProfileDelta:              unitResult.ProfileDelta,
			ProfileRefreshRecommended: unitResult.ProfileRefreshRecommended,
		})
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_current_codebase_batch",
		"duration", time.Since(startedAt),
		"units_count", len(out),
	)
	return &AnalyzeCurrentCodebaseBatchResult{Units: out}, nil
}

// GetProjectStructure 获取项目目录结构
func (s *AnalyzerService) GetProjectStructure(projectRoot string) (string, error) {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "analyzer.get_project_structure",
		"project_root", projectRoot,
	)

	var structure strings.Builder
	selectionPolicy := fileanalysis.NewConfiguredSelectionPolicy(s.configRepo, projectRoot)
	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对路径
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			structure.WriteString(".\n")
			return nil
		}
		if selectionPolicy.IsExcluded(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 只显示前3层
		depth := strings.Count(relPath, "/")
		if depth > 3 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 添加缩进
		indent := strings.Repeat("  ", depth)
		structure.WriteString(indent)
		if info.IsDir() {
			structure.WriteString("[dir] ")
		} else {
			structure.WriteString("[file] ")
		}
		structure.WriteString(info.Name())
		structure.WriteString("\n")

		return nil
	})

	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.get_project_structure.walk",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return "", err
	}

	result := structure.String()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.get_project_structure",
		"method", "walk",
		"duration", time.Since(startedAt),
		"output_length", len(result),
	)

	return result, nil
}

// FindMainFiles 查找主要入口文件
func (s *AnalyzerService) FindMainFiles(projectRoot string) []string {
	startedAt := time.Now()
	var mainFiles []string

	// 常见的主入口文件模式
	patterns := []string{
		"main.go",
		"cmd/*/main.go",
		"cmd/*/*/main.go",
		"command/*/main.go",
		"command/*/*/main.go",
		"index.js",
		"index.ts",
		"app.js",
		"app.py",
		"main.py",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectRoot, pattern))
		if err == nil {
			for _, match := range matches {
				relPath, _ := filepath.Rel(projectRoot, match)
				mainFiles = append(mainFiles, relPath)
			}
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.find_main_files",
		"project_root", projectRoot,
		"duration", time.Since(startedAt),
		"main_files_count", len(mainFiles),
	)

	return mainFiles
}

// FindReadmePath 查找项目 README 文件路径
func (s *AnalyzerService) FindReadmePath(projectRoot string) string {
	readmePath := filepath.Join(projectRoot, "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		return ""
	}
	return "README.md"
}

// AnalyzeProjectFull 完整项目分析
func (s *AnalyzerService) AnalyzeProjectFull(ctx context.Context, projectRoot, projectName string) (*AnalyzeProjectResult, error) {
	return s.AnalyzeProjectFullWithLanguage(ctx, projectRoot, projectName, "")
}

// AnalyzeProjectFullWithLanguage 完整项目分析，可由命令行显式指定语言覆盖配置
func (s *AnalyzerService) AnalyzeProjectFullWithLanguage(ctx context.Context, projectRoot, projectName, requestedLanguage string) (*AnalyzeProjectResult, error) {
	return s.AnalyzeProjectFullWithOptions(ctx, projectRoot, projectName, requestedLanguage, AnalyzeProjectOptions{})
}

// AnalyzeProjectOptions 控制 AnalyzeProjectFullWithOptions 的项目画像分析上下文
type AnalyzeProjectOptions struct {
	ExistingProfile *domain.ProjectProfile
	FocusPaths      []string
}

// AnalyzeProjectFullWithOptions 完整项目分析，支持基于已有画像和指定路径做增量刷新
func (s *AnalyzerService) AnalyzeProjectFullWithOptions(ctx context.Context, projectRoot, projectName, requestedLanguage string, opts AnalyzeProjectOptions) (*AnalyzeProjectResult, error) {
	startedAt := time.Now()
	focusPaths := utils.RelativePaths(projectRoot, opts.FocusPaths)
	existingProfileJSON := ""
	if len(focusPaths) > 0 {
		existingProfileJSON = marshalProjectProfile(opts.ExistingProfile)
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "analyzer.analyze_project_full",
		"project_root", projectRoot,
		"project_name", projectName,
		"has_existing_profile", existingProfileJSON != "",
		"existing_profile_bytes", len(existingProfileJSON),
		"focus_paths_count", len(opts.FocusPaths),
	)

	// 获取项目语言
	language := requestedLanguage
	if s.configRepo != nil {
		if language == "" {
			language = s.configRepo.GetProjectConfig().Language
		}
	}
	if language == "" {
		language = "unknown"
	}
	structure, _ := s.GetProjectStructure(projectRoot)
	if len(focusPaths) > 0 {
		structure = focusedStructure(focusPaths)
	}

	// 调用 Agent 分析项目（Agent 会自己探索项目结构）
	req := &AnalyzeProjectRequest{
		ProjectName:         projectName,
		RootPath:            projectRoot,
		Language:            language,
		Structure:           structure,
		ReadmePath:          s.FindReadmePath(projectRoot),
		MainFiles:           s.FindMainFiles(projectRoot),
		ExistingProfileJSON: existingProfileJSON,
		FocusPaths:          focusPaths,
		UserContext:         runtimecontext.UserContext(ctx),
	}

	result, err := s.AnalyzeProject(ctx, req)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_project_full",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, err
	}
	if result.Language == "" {
		result.Language = language
	}
	if result.Structure == "" {
		result.Structure = structure
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_project_full",
		"duration", time.Since(startedAt),
		"incremental_profile", existingProfileJSON != "" && len(focusPaths) > 0,
	)
	return result, nil
}

func marshalProjectProfile(profile *domain.ProjectProfile) string {
	if profile == nil {
		return ""
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		logger.Warn("marshal project profile failed", "error", err)
		return ""
	}
	return string(data)
}

// NewProjectProfile 将分析结果转换为持久化项目画像格式。
func NewProjectProfile(result *AnalyzeProjectResult, projectName, language string) *domain.ProjectProfile {
	if result == nil {
		return nil
	}
	if language == "" {
		language = result.Language
	}
	if language == "" {
		language = "unknown"
	}

	return &domain.ProjectProfile{
		ProjectName:        projectName,
		Language:           language,
		Frameworks:         result.Frameworks,
		Architecture:       result.Architecture,
		Structure:          result.Structure,
		CommonUtils:        result.CommonUtils,
		KeyModules:         result.KeyModules,
		ConfigPatterns:     result.ConfigPatterns,
		Dependencies:       result.Dependencies,
		Layers:             result.Layers,
		DependencyGraph:    result.DependencyGraph,
		DataFlow:           result.DataFlow,
		FrameworkPatterns:  result.FrameworkPatterns,
		BusinessMethods:    result.BusinessMethods,
		ValidationCommands: result.ValidationCommands,
		Summary:            result.Summary,
		GeneratedAt:        time.Now().Format("2006-01-02 15:04:05"),
	}
}

// AnalyzeCodebaseOptions 控制当前代码学习如何收集上下文。
type AnalyzeCodebaseOptions struct {
	FocusPaths         []string
	RuntimeLabel       string
	AnalysisUnit       domain.AnalysisUnit
	LearningMode       config.LearningMode
	SelectedFiles      []domain.FileInfo
	SelectedFilesSet   bool
	KnownPatternsJSON  string
	KnownPatternsCount int
	UseSnapshotDiffs   bool
	RunContext         *CodebaseRunContext
}

const maxSampleFiles = 15

// CodebaseRunContext 保存一次 learn current 运行内可复用的代码库上下文。
type CodebaseRunContext struct {
	ProjectStructure string
	MainFiles        []string
	SampleFiles      []agent.SampleFile
	DiffFiles        []agent.DiffFileRef
	SnapshotFlow     *snapshotflow.Result
}

// BuildCodebaseRunContext 预收集 learn current 中多个分析单元可复用的上下文。
func (s *AnalyzerService) BuildCodebaseRunContext(ctx context.Context, projectRoot, language string, opts AnalyzeCodebaseOptions) (*CodebaseRunContext, error) {
	structure, _ := s.GetProjectStructure(projectRoot)
	mainFiles := s.FindMainFiles(projectRoot)
	sampleFiles := s.collectSampleFilesFromRoots(projectRoot, opts.FocusPaths, language)
	var diffFiles []agent.DiffFileRef
	var snapshotFlow *snapshotflow.Result
	var err error
	focusPaths := utils.RelativePaths(projectRoot, opts.FocusPaths)
	if opts.UseSnapshotDiffs || len(focusPaths) == 0 {
		selectedFiles := append([]domain.FileInfo(nil), opts.SelectedFiles...)
		selectionPolicy := fileanalysis.NewConfiguredSelectionPolicy(s.configRepo, projectRoot)
		if len(selectedFiles) == 0 && !opts.SelectedFilesSet {
			selection, selectErr := fileanalysis.SelectFiles(fileanalysis.SelectOptions{
				Root:          projectRoot,
				Policy:        selectionPolicy,
				FocusAbsPaths: opts.FocusPaths,
			})
			if selectErr != nil {
				return nil, selectErr
			}
			selectedFiles = selection.Files
		}
		snapshotFlow, err = snapshotflow.BuildScopedWithOptions(ctx, projectRoot, selectedFiles, focusPaths, snapshotflow.Options{
			DiffAllowed: func(path string) bool {
				return !selectionPolicy.IsExcluded(path)
			},
		})
		if err != nil {
			return nil, err
		}
		sampleFiles = sampleFilesFromFileInfos(snapshotFlow.AddedFiles)
		diffFiles = snapshotFlow.DiffFiles
	}
	return &CodebaseRunContext{
		ProjectStructure: structure,
		MainFiles:        append([]string(nil), mainFiles...),
		SampleFiles:      append([]agent.SampleFile(nil), sampleFiles...),
		DiffFiles:        append([]agent.DiffFileRef(nil), diffFiles...),
		SnapshotFlow:     snapshotFlow,
	}, nil
}

// AnalyzeCodebaseFull 完整代码库分析（提取模式并转换为 domain.Pattern）
func (s *AnalyzerService) AnalyzeCodebaseFull(ctx context.Context, projectRoot, projectName, language string) (*AnalyzeCurrentCodebaseResult, []domain.Pattern, error) {
	return s.AnalyzeCodebaseFullWithOptions(ctx, projectRoot, projectName, language, AnalyzeCodebaseOptions{UseSnapshotDiffs: true})
}

// AnalyzeCodebaseFullWithOptions 完整代码库分析，支持指定扫描范围
func (s *AnalyzerService) AnalyzeCodebaseFullWithOptions(ctx context.Context, projectRoot, projectName, language string, opts AnalyzeCodebaseOptions) (*AnalyzeCurrentCodebaseResult, []domain.Pattern, error) {
	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "analyzer.analyze_codebase_full",
		"project_root", projectRoot,
		"project_name", projectName,
		"language", language,
		"focus_paths_count", len(opts.FocusPaths),
		"selected_files_count", len(opts.SelectedFiles),
	)

	runContext := opts.RunContext
	if runContext == nil {
		var err error
		runContext, err = s.BuildCodebaseRunContext(ctx, projectRoot, language, opts)
		if err != nil {
			return nil, nil, err
		}
	}

	structure := runContext.ProjectStructure
	focusPaths := utils.RelativePaths(projectRoot, opts.FocusPaths)
	if len(focusPaths) > 0 {
		structure = focusedStructure(focusPaths)
	}
	mainFiles := append([]string(nil), runContext.MainFiles...)
	sampleFiles := filterSampleFilesByFocus(runContext.SampleFiles, focusPaths)
	diffFiles := filterDiffFilesByFocus(runContext.DiffFiles, focusPaths)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.collect_codebase_context",
		"duration", time.Since(startedAt),
		"structure_length", len(structure),
		"focus_paths_count", len(focusPaths),
		"main_files_count", len(mainFiles),
		"sample_files_count", len(sampleFiles),
		"diff_files_count", len(diffFiles),
	)

	req := &AnalyzeCurrentCodebaseRequest{
		ProjectName:  projectName,
		RootPath:     projectRoot,
		Language:     language,
		LearningMode: opts.LearningMode,
		RuntimeLabel: opts.RuntimeLabel,
		AnalysisUnit: opts.AnalysisUnit,
		FocusPaths:   focusPaths,
		Structure:    structure,
		MainFiles:    mainFiles,
		SampleFiles:  sampleFiles,
		DiffFiles:    diffFiles,
		UserContext:  runtimecontext.UserContext(ctx),
	}

	result, err := s.AnalyzeCurrentCodebase(ctx, req)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_codebase_full",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, nil, err
	}
	for i := range result.Patterns {
		if result.Patterns[i].AnalysisUnitID == "" {
			result.Patterns[i].AnalysisUnitID = opts.AnalysisUnit.ID
		}
		if result.Patterns[i].AnalysisUnitName == "" {
			result.Patterns[i].AnalysisUnitName = opts.AnalysisUnit.Name
		}
	}

	// Patterns 已经是 []domain.Pattern，直接使用
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_codebase_full",
		"duration", time.Since(startedAt),
		"patterns_count", len(result.Patterns),
	)
	return result, result.Patterns, nil
}

func sampleFilesFromFileInfos(files []domain.FileInfo) []agent.SampleFile {
	limit := len(files)
	if limit > maxSampleFiles {
		limit = maxSampleFiles
	}
	samples := make([]agent.SampleFile, 0, limit)
	for _, file := range files {
		if len(samples) >= maxSampleFiles {
			break
		}
		samples = append(samples, agent.SampleFile{Path: file.Path})
	}
	return samples
}

func batchFocusPaths(units []agent.AnalyzeCurrentCodebaseBatchUnit) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, unit := range units {
		for _, path := range unit.FocusPaths {
			path = normalizeRelPath(path)
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func batchSeedPaths(units []agent.AnalyzeCurrentCodebaseBatchUnit, mainFiles []string) []string {
	seen := make(map[string]bool)
	var paths []string
	add := func(path string) {
		path = normalizeRelPath(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, path := range mainFiles {
		add(path)
	}
	for _, unit := range units {
		for _, path := range unit.FocusPaths {
			add(path)
		}
		for _, file := range unit.SampleFiles {
			add(file.Path)
		}
		for _, file := range unit.DiffFiles {
			add(file.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func filterSampleFilesByFocus(files []agent.SampleFile, focusPaths []string) []agent.SampleFile {
	out := make([]agent.SampleFile, 0, len(files))
	for _, file := range files {
		if pathInFocus(file.Path, focusPaths) {
			out = append(out, file)
		}
	}
	return out
}

func filterDiffFilesByFocus(files []agent.DiffFileRef, focusPaths []string) []agent.DiffFileRef {
	out := make([]agent.DiffFileRef, 0, len(files))
	for _, file := range files {
		if pathInFocus(file.Path, focusPaths) {
			out = append(out, file)
		}
	}
	return out
}

func pathInFocus(path string, focusPaths []string) bool {
	path = normalizeRelPath(path)
	if path == "" {
		return false
	}
	if len(focusPaths) == 0 {
		return true
	}
	for _, focus := range focusPaths {
		focus = normalizeRelPath(focus)
		if focus == "" {
			continue
		}
		if path == focus || strings.HasPrefix(path, focus+"/") {
			return true
		}
	}
	return false
}

func normalizeRelPath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(filepath.Clean(path)))
	if path == "." {
		return ""
	}
	return strings.TrimPrefix(path, "./")
}

// collectSampleFiles 收集项目中的代表性代码文件
func (s *AnalyzerService) collectSampleFiles(projectRoot, language string) []agent.SampleFile {
	return s.collectSampleFilesFromRoots(projectRoot, nil, language)
}

func (s *AnalyzerService) collectSampleFilesFromRoots(projectRoot string, scanRoots []string, language string) []agent.SampleFile {
	startedAt := time.Now()
	extensions := sampleFileExtensions(language)

	var files []agent.SampleFile
	seenFiles := make(map[string]bool)
	selectionPolicy := fileanalysis.NewConfiguredSelectionPolicy(s.configRepo, projectRoot)
	if len(scanRoots) == 0 {
		scanRoots = []string{projectRoot}
	}

	for _, scanRoot := range scanRoots {
		if len(files) >= maxSampleFiles {
			break
		}
		if scanRoot == "" {
			continue
		}

		selection, err := fileanalysis.SelectFiles(fileanalysis.SelectOptions{
			Root:          projectRoot,
			Policy:        selectionPolicy,
			FocusAbsPaths: []string{scanRoot},
		})
		if err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "analyzer.collect_sample_files",
				"duration", time.Since(startedAt),
				"scan_root", scanRoot,
				"error", err,
			)
			continue
		}

		for _, selected := range selection.Files {
			if len(files) >= maxSampleFiles {
				break
			}
			relPath := filepath.ToSlash(selected.Path)
			if seenFiles[relPath] || !matchesAnySuffix(relPath, extensions) {
				continue
			}
			absPath := filepath.Join(projectRoot, filepath.FromSlash(relPath))
			info, err := os.Stat(absPath)
			if err != nil || info.Size() == 0 {
				continue
			}
			files = append(files, agent.SampleFile{
				Path: relPath,
			})
			seenFiles[relPath] = true

			logger.Diagnostic(i18n.Get("LoggerAnalyzerSampleFileCollected"), "file", relPath, "size", info.Size())
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.collect_sample_files",
		"project_root", projectRoot,
		"language", language,
		"extensions", strings.Join(extensions, ","),
		"scan_roots_count", len(scanRoots),
		"duration", time.Since(startedAt),
		"sample_files_count", len(files),
	)

	return files
}

func sampleFileExtensions(language string) []string {
	switch language {
	case "go":
		return []string{".go"}
	case "typescript":
		return []string{".ts", ".tsx"}
	case "javascript":
		return []string{".js", ".jsx"}
	case "python":
		return []string{".py"}
	case "java":
		return []string{".java"}
	default:
		return nil
	}
}

func matchesAnySuffix(path string, suffixes []string) bool {
	if len(suffixes) == 0 {
		return true
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func focusedStructure(focusPaths []string) string {
	var b strings.Builder
	b.WriteString("Focused scan paths:\n")
	for _, path := range focusPaths {
		b.WriteString("- ")
		b.WriteString(path)
		b.WriteByte('\n')
	}

	parentPaths := focusedParentPaths(focusPaths)
	if len(parentPaths) > 0 {
		b.WriteString("\nFocused path parents:\n")
		for _, path := range parentPaths {
			b.WriteString("- ")
			b.WriteString(path)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func focusedParentPaths(focusPaths []string) []string {
	seen := make(map[string]bool)
	var parents []string
	for _, path := range focusPaths {
		path = strings.TrimSpace(filepath.ToSlash(path))
		path = strings.Trim(path, "/")
		if path == "" || path == "." {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(path))
		for dir != "." && dir != "/" && dir != "" {
			if !seen[dir] {
				seen[dir] = true
				parents = append(parents, dir)
			}
			next := filepath.ToSlash(filepath.Dir(dir))
			if next == dir {
				break
			}
			dir = next
		}
	}
	sort.Strings(parents)
	return parents
}
