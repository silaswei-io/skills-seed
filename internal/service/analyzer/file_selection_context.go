package analyzer

import (
	"context"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

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

func newFileSelectionContextCollector(cfg config.StructuralConfig) structuralCollector {
	provider := config.NormalizeStructuralProvider(string(cfg.Provider))
	if provider == config.StructuralProviderTreeSitter {
		return nil
	}
	return &renderedStructuralCollector{
		provider:   newCodeGraphProvider(cfg),
		renderer:   structuralRenderer{},
		maxSymbols: cfg.MaxSymbols,
	}
}

// CollectFileSelectionContext 在 AI 文件筛选前收集结构化候选线索。
// 该阶段只使用 CodeGraph/auto provider，避免未显式选择的 tree-sitter 全仓扫描。
func (s *AnalyzerService) CollectFileSelectionContext(ctx context.Context, projectRoot string, req FileSelectionContextRequest) string {
	if s == nil || projectRoot == "" || s.fileSelectionContextCollector == nil {
		return ""
	}
	contextText, err := s.fileSelectionContextCollector.Collect(ctx, projectRoot, structuralContextRequest{
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
	return contextText
}
