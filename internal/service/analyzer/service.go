// Package analyzer 提供代码分析服务
//
// 本包实现项目结构分析和当前代码学习上下文分析功能
//   - AnalyzeProject: 分析项目结构和特点
//   - PlanLearningAgenda: 为当前代码学习规划证据焦点
//   - AnalyzeCurrentCodebaseBatch: 在当前学习阶段会话中提取模式
//   - AnalyzeCurrentDeltaBatch: 基于 diff 判断知识变化
//
// 服务职责
//   - 调用 AI Agent 进行代码分析
//   - 转换领域模型和 Agent 模型
//   - 包装错误为领域错误
package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/snapshotflow"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

// AnalyzerService 代码分析服务
// 职责：分析代码、提取模式、分析项目结构
type AnalyzerService struct {
	agent               agent.Agent
	configRepo          config.Reader
	symbolResolver      sourcecode.Resolver
	structuralCollector structuralCollector
}

// NewAnalyzerService 创建分析服务
func NewAnalyzerService(ag agent.Agent, configRepo config.Reader) *AnalyzerService {
	structuralConfig := config.StructuralConfig{Provider: config.StructuralProviderAuto}
	if configRepo != nil {
		structuralConfig = configRepo.GetCurrentLearningConfig().Structural
	}
	svc := &AnalyzerService{
		agent:          ag,
		configRepo:     configRepo,
		symbolResolver: sourcecode.NewResolver(structuralConfig),
	}
	if configRepo != nil {
		cfg := structuralConfig
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

// AnalyzeProjectRequest 项目分析请求
type AnalyzeProjectRequest struct {
	ProjectName          string
	RootPath             string
	Language             string
	Structure            string
	StructuralContext    string
	ReadmePath           string
	MainFiles            []string
	EngineeringKnowledge []string
	ExistingProfileJSON  string
	FocusPaths           []string
	UserContext          string
	LearningSession      agent.LearningSession
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
	EngineeringRules   []domain.EngineeringRule
	ValidationCommands []domain.ValidationCommand
	Summary            string
}

// analyzeProjectProfile 在当前学习阶段会话中分析项目结构和特点。
func (s *AnalyzerService) analyzeProjectProfile(ctx context.Context, req *AnalyzeProjectRequest) (*AnalyzeProjectResult, error) {
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
	engineeringKnowledge := req.EngineeringKnowledge
	if engineeringKnowledge == nil {
		engineeringKnowledge, err = engineeringKnowledgePaths(req.RootPath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.Get("AnalyzerCollectEngineeringKnowledgeFailed"), err)
		}
	}

	agentReq := &agent.AnalyzeProjectRequest{
		ProjectName:          req.ProjectName,
		RootPath:             req.RootPath,
		Language:             req.Language,
		Structure:            req.Structure,
		StructuralContext:    structuralContext,
		ReadmePath:           req.ReadmePath,
		MainFiles:            req.MainFiles,
		EngineeringKnowledge: engineeringKnowledge,
		ExistingProfileJSON:  req.ExistingProfileJSON,
		FocusPaths:           req.FocusPaths,
		UserContext:          req.UserContext,
	}

	if req.LearningSession == nil {
		return nil, fmt.Errorf("%s", i18n.Get("AnalyzerLearningSessionRequired"))
	}
	result, err := req.LearningSession.RefreshProjectProfile(ctx, agentReq)
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
	if err := agent.RequireResult(result, "AnalyzeProject"); err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeProjectFailed"), err)
	}
	refs := append(sourcecode.UtilityReferences(result.CommonUtils), sourcecode.BusinessMethodReferences(result.BusinessMethods)...)
	catalog, err := s.symbolResolver.Resolve(ctx, req.RootPath, refs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AnalyzerResolveProjectProfileSymbolsFailed"), err)
	}
	entryVerifier := sourcecode.NewVerifier(catalog)
	result.CommonUtils = entryVerifier.VerifyUtilities(result.CommonUtils)
	result.BusinessMethods = entryVerifier.VerifyBusinessMethods(result.BusinessMethods)
	var engineeringRuleIssues []error
	result.EngineeringRules, engineeringRuleIssues = validateEngineeringRules(req.RootPath, engineeringKnowledge, req.UserContext != "", result.EngineeringRules)
	for _, issue := range engineeringRuleIssues {
		logger.Diagnostic(i18n.Get("AnalyzerDroppedInvalidEngineeringRule"),
			"operation", "analyzer.validate_engineering_rules",
			"reason", issue.Error(),
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
		EngineeringRules:   result.EngineeringRules,
		ValidationCommands: result.ValidationCommands,
		Summary:            result.Summary,
	}, nil
}

type AnalyzeCurrentEvidenceFocus struct {
	EvidenceFocus domain.EvidenceFocus
	FocusAbsPaths []string
}

type AnalyzeCurrentCodebaseBatchOptions struct {
	RuntimeLabel    string
	LearningMode    config.LearningMode
	ChangeProfile   string
	RunContext      *CodebaseRunContext
	LearningSession agent.LearningSession
	Focuses         []AnalyzeCurrentEvidenceFocus
}

type AnalyzeCurrentEvidenceResult struct {
	EvidenceFocus             domain.EvidenceFocus
	Patterns                  []domain.Pattern
	ProfileRefreshRecommended agent.ProfileRefreshRecommendation
}

type AnalyzeCurrentCodebaseBatchResult struct {
	Focuses []AnalyzeCurrentEvidenceResult
}

type AnalyzeCurrentDeltaFocus struct {
	EvidenceFocus   domain.EvidenceFocus
	FocusAbsPaths   []string
	RelatedPatterns []domain.Pattern
}

type AnalyzeCurrentDeltaBatchOptions struct {
	RuntimeLabel    string
	LearningMode    config.LearningMode
	ChangeProfile   string
	RunContext      *CodebaseRunContext
	LearningSession agent.LearningSession
	Focuses         []AnalyzeCurrentDeltaFocus
}

type AnalyzeCurrentDeltaBatchResult struct {
	Changes                   []domain.KnowledgeChange
	ProfileRefreshRecommended agent.ProfileRefreshRecommendation
}

// PlanLearningAgendaRequest 请求按业务能力规划当前待学习文件。
type PlanLearningAgendaRequest struct {
	ProjectName       string
	RootPath          string
	Language          string
	LearningMode      config.LearningMode
	LearningScope     config.LearningScope
	FocusPaths        []string
	StructuralContext string
	UserContext       string
	LearningSession   agent.LearningSession
}

// SelectLearningCandidatesRequest 请求 AI 从本地候选文件中收敛学习入口。
type SelectLearningCandidatesRequest struct {
	ProjectName         string
	RootPath            string
	Language            string
	LearningMode        config.LearningMode
	LearningScope       config.LearningScope
	CandidatePaths      []string
	RequiredPaths       []string
	StructuralSeedPaths []string
	UserContext         string
	Progress            func(SelectLearningCandidatesStage)
	LearningSession     agent.LearningSession
}

type SelectLearningCandidatesStage string

const (
	SelectLearningCandidatesStageStructuralContext SelectLearningCandidatesStage = "structural_context"
	SelectLearningCandidatesStageCodeGraphIndex    SelectLearningCandidatesStage = "codegraph_index"
	SelectLearningCandidatesStageCodeGraphContext  SelectLearningCandidatesStage = "codegraph_context"
	SelectLearningCandidatesStageCodeGraphRepair   SelectLearningCandidatesStage = "codegraph_repair"
	SelectLearningCandidatesStageTreeSitterContext SelectLearningCandidatesStage = "treesitter_context"
	SelectLearningCandidatesStageAgent             SelectLearningCandidatesStage = "agent"
)

// SelectLearningCandidates 在大候选集上执行 AI 候选收敛。
func (s *AnalyzerService) SelectLearningCandidates(ctx context.Context, req *SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
	structuralContext := ""
	var err error
	report := func(stage SelectLearningCandidatesStage) {
		if req.Progress != nil {
			req.Progress(stage)
		}
	}
	report(SelectLearningCandidatesStageStructuralContext)
	structuralContext, err = s.collectStructuralContext(ctx, req.RootPath, structuralContextRequest{
		ProjectName: req.ProjectName,
		Language:    req.Language,
		Purpose:     "current codebase learning candidate selection",
		FocusPaths:  req.StructuralSeedPaths,
		SeedPaths:   req.StructuralSeedPaths,
		Progress: func(stage structuralContextStage) {
			switch stage {
			case structuralContextStageCodeGraphIndex:
				report(SelectLearningCandidatesStageCodeGraphIndex)
			case structuralContextStageCodeGraphContext:
				report(SelectLearningCandidatesStageCodeGraphContext)
			case structuralContextStageCodeGraphRepair:
				report(SelectLearningCandidatesStageCodeGraphRepair)
			case structuralContextStageTreeSitter:
				report(SelectLearningCandidatesStageTreeSitterContext)
			}
		},
	})
	if err != nil {
		return nil, err
	}
	report(SelectLearningCandidatesStageAgent)
	agentReq := &agent.SelectLearningCandidatesRequest{
		ProjectName:       req.ProjectName,
		RootPath:          req.RootPath,
		Language:          req.Language,
		LearningMode:      req.LearningMode,
		LearningScope:     req.LearningScope,
		CandidatePaths:    req.CandidatePaths,
		RequiredPaths:     req.RequiredPaths,
		StructuralContext: structuralContext,
		UserContext:       req.UserContext,
	}
	if req.LearningSession == nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), fmt.Errorf("%s", i18n.Get("AnalyzerLearningSessionRequiredForAgenda")))
	}
	result, err := req.LearningSession.SelectLearningCandidates(ctx, agentReq)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}
	return result, nil
}

// PlanLearningAgenda 按业务能力拆分当前待学习文件。
func (s *AnalyzerService) PlanLearningAgenda(ctx context.Context, req *PlanLearningAgendaRequest) ([]domain.EvidenceFocus, error) {
	structuralContext := req.StructuralContext
	if structuralContext == "" {
		var err error
		structuralContext, err = s.collectStructuralContext(ctx, req.RootPath, structuralContextRequest{
			ProjectName: req.ProjectName,
			Language:    req.Language,
			Purpose:     "current codebase learning agenda planning",
			FocusPaths:  req.FocusPaths,
			SeedPaths:   req.FocusPaths,
		})
		if err != nil {
			return nil, err
		}
	}
	agentReq := &agent.PlanLearningAgendaRequest{
		ProjectName:       req.ProjectName,
		RootPath:          req.RootPath,
		Language:          req.Language,
		LearningMode:      req.LearningMode,
		LearningScope:     req.LearningScope,
		FocusPaths:        req.FocusPaths,
		StructuralContext: structuralContext,
		UserContext:       req.UserContext,
	}
	if req.LearningSession == nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), fmt.Errorf("%s", i18n.Get("AnalyzerLearningSessionRequiredForAgenda")))
	}
	result, err := req.LearningSession.PlanLearningAgenda(ctx, agentReq)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}
	if err := agent.RequireResult(result, "PlanLearningAgenda"); err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}
	return result.Focuses, nil
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

	focuses := make([]agent.AnalyzeCurrentEvidenceFocus, 0, len(opts.Focuses))
	focusByID := make(map[string]domain.EvidenceFocus, len(opts.Focuses))
	focusByName := make(map[string]domain.EvidenceFocus, len(opts.Focuses))
	for _, focus := range opts.Focuses {
		focusPaths := projectpath.Relative(projectRoot, focus.FocusAbsPaths)
		focuses = append(focuses, agent.AnalyzeCurrentEvidenceFocus{
			EvidenceFocus: focus.EvidenceFocus,
			FocusPaths:    focusPaths,
			SampleFiles:   filterSampleFilesByFocus(runContext.SampleFiles, focusPaths),
			DiffFiles:     filterDiffFilesByFocus(runContext.DiffFiles, focusPaths),
		})
		focusByID[focus.EvidenceFocus.ID] = focus.EvidenceFocus
		focusByName[focus.EvidenceFocus.Name] = focus.EvidenceFocus
	}

	agentReq := &agent.AnalyzeCurrentCodebaseBatchRequest{
		ProjectName:       projectName,
		RootPath:          projectRoot,
		Language:          language,
		LearningMode:      opts.LearningMode,
		RuntimeLabel:      opts.RuntimeLabel,
		Focuses:           focuses,
		Structure:         runContext.ProjectStructure,
		MainFiles:         append([]string(nil), runContext.MainFiles...),
		UserContext:       runtimecontext.UserContext(ctx),
		ChangeProfile:     opts.ChangeProfile,
		StructuralContext: "",
	}

	structuralContext, err := s.collectStructuralContext(ctx, projectRoot, structuralContextRequest{
		ProjectName: projectName,
		Language:    language,
		Purpose:     "current codebase batch pattern extraction",
		FocusPaths:  batchFocusPaths(focuses),
		SeedPaths:   batchSeedPaths(focuses, runContext.MainFiles),
	})
	if err != nil {
		return nil, err
	}
	agentReq.StructuralContext = structuralContext

	if opts.LearningSession == nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), fmt.Errorf("%s", i18n.Get("AnalyzerLearningSessionRequiredForBatch")))
	}
	result, err := opts.LearningSession.AnalyzeCurrentCodebaseBatch(ctx, agentReq)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "analyzer.analyze_current_codebase_batch",
			"duration", time.Since(startedAt),
			"error", err,
		)
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}
	if err := agent.RequireResult(result, "AnalyzeCurrentCodebaseBatch"); err != nil {
		return nil, domain.NewDomainError(domain.ErrAIService, i18n.Get("AnalyzerAnalyzeCodebaseFailed"), err)
	}

	mergedResults := make(map[string]agent.AnalyzeCurrentEvidenceResult, len(opts.Focuses))
	for _, focusResult := range result.Focuses {
		focus, ok := resolveBatchResultFocus(focusResult, focusByID, focusByName)
		if !ok && len(opts.Focuses) == 1 {
			focus = opts.Focuses[0].EvidenceFocus
			ok = true
		}
		if !ok {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("AnalyzerAnalyzeBatchUnknownFocus", map[string]interface{}{"Focus": focusResult.FocusID}))
		}
		existing := mergedResults[focus.ID]
		if existing.FocusID == "" {
			existing.FocusID = focus.ID
			existing.FocusName = focus.Name
		}
		existing.Patterns = append(existing.Patterns, focusResult.Patterns...)
		if focusResult.ProfileRefreshRecommended.Needed {
			existing.ProfileRefreshRecommended = focusResult.ProfileRefreshRecommended
		}
		mergedResults[focus.ID] = existing
	}
	for _, requested := range opts.Focuses {
		if _, ok := mergedResults[requested.EvidenceFocus.ID]; !ok {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("AnalyzerAnalyzeBatchOmittedFocus", map[string]interface{}{"Focus": requested.EvidenceFocus.ID}))
		}
	}
	allPatterns := make([]domain.Pattern, 0)
	for _, result := range mergedResults {
		allPatterns = append(allPatterns, result.Patterns...)
	}
	validator, err := newCurrentPatternValidator(ctx, projectRoot, allPatterns, s.symbolResolver)
	if err != nil {
		return nil, err
	}
	out := make([]AnalyzeCurrentEvidenceResult, 0, len(opts.Focuses))
	for _, requested := range opts.Focuses {
		focusResult := mergedResults[requested.EvidenceFocus.ID]
		patterns := validator.validatePatterns(focusResult.Patterns)
		out = append(out, AnalyzeCurrentEvidenceResult{
			EvidenceFocus:             requested.EvidenceFocus,
			Patterns:                  patterns,
			ProfileRefreshRecommended: focusResult.ProfileRefreshRecommended,
		})
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.analyze_current_codebase_batch",
		"duration", time.Since(startedAt),
		"focuses_count", len(out),
	)
	return &AnalyzeCurrentCodebaseBatchResult{Focuses: out}, nil
}

func resolveBatchResultFocus(result agent.AnalyzeCurrentEvidenceResult, byID, byName map[string]domain.EvidenceFocus) (domain.EvidenceFocus, bool) {
	if focus, ok := byID[result.FocusID]; ok {
		return focus, true
	}
	if result.FocusName != "" {
		if focus, ok := byName[result.FocusName]; ok {
			return focus, true
		}
	}
	return domain.EvidenceFocus{}, false
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

// AnalyzeProjectOptions 控制项目画像刷新上下文。
type AnalyzeProjectOptions struct {
	ExistingProfile *domain.ProjectProfile
	FocusPaths      []string
	LearningSession agent.LearningSession
}

// buildProjectProfileResult 完整分析项目画像，支持基于已有画像和指定路径做增量刷新。
func (s *AnalyzerService) buildProjectProfileResult(ctx context.Context, projectRoot, projectName, requestedLanguage string, opts AnalyzeProjectOptions) (*AnalyzeProjectResult, error) {
	startedAt := time.Now()
	focusPaths := projectpath.Relative(projectRoot, opts.FocusPaths)
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
		LearningSession:     opts.LearningSession,
	}

	result, err := s.analyzeProjectProfile(ctx, req)
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

// RefreshProjectProfile 刷新并返回可直接持久化的项目画像。
func (s *AnalyzerService) RefreshProjectProfile(ctx context.Context, projectRoot, projectName, requestedLanguage string, opts AnalyzeProjectOptions) (*domain.ProjectProfile, error) {
	result, err := s.buildProjectProfileResult(ctx, projectRoot, projectName, requestedLanguage, opts)
	if err != nil {
		return nil, err
	}
	return NewProjectProfile(result, projectName, requestedLanguage), nil
}

func marshalProjectProfile(profile *domain.ProjectProfile) string {
	if profile == nil {
		return ""
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		logger.Warn(i18n.Get("AnalyzerMarshalProjectProfileFailed"), "error", err)
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
		EngineeringRules:   result.EngineeringRules,
		ValidationCommands: result.ValidationCommands,
		Summary:            result.Summary,
		GeneratedAt:        time.Now().Format("2006-01-02 15:04:05"),
	}
}

// AnalyzeCodebaseOptions 控制当前代码学习如何收集上下文。
type AnalyzeCodebaseOptions struct {
	FocusPaths         []string
	RuntimeLabel       string
	EvidenceFocus      domain.EvidenceFocus
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

// BuildCodebaseRunContext 预收集 learn current 中多个证据焦点可复用的上下文。
func (s *AnalyzerService) BuildCodebaseRunContext(ctx context.Context, projectRoot, language string, opts AnalyzeCodebaseOptions) (*CodebaseRunContext, error) {
	structure, _ := s.GetProjectStructure(projectRoot)
	mainFiles := s.FindMainFiles(projectRoot)
	sampleFiles := s.collectSampleFilesFromRoots(projectRoot, opts.FocusPaths, language)
	var diffFiles []agent.DiffFileRef
	var snapshotFlow *snapshotflow.Result
	var err error
	focusPaths := projectpath.Relative(projectRoot, opts.FocusPaths)
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

func batchFocusPaths(focuses []agent.AnalyzeCurrentEvidenceFocus) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, focus := range focuses {
		for _, path := range focus.FocusPaths {
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

func batchSeedPaths(focuses []agent.AnalyzeCurrentEvidenceFocus, mainFiles []string) []string {
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
	for _, focus := range focuses {
		for _, path := range focus.FocusPaths {
			add(path)
		}
		for _, file := range focus.SampleFiles {
			add(file.Path)
		}
		for _, file := range focus.DiffFiles {
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
