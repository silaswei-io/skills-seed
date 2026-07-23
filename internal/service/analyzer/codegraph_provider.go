package analyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

type codeGraphProvider struct {
	maxSymbols int
	client     codeGraphClient
}

type codeGraphClient interface {
	EnsureReady(ctx context.Context, projectRoot string) (*codegraph.Status, error)
	Repair(ctx context.Context, projectRoot string) (*codegraph.Status, error)
	Run(ctx context.Context, projectRoot string, args ...string) (string, error)
}

func newCodeGraphProvider(cfg config.StructuralConfig) *codeGraphProvider {
	maxSymbols := cfg.MaxSymbols
	if maxSymbols <= 0 {
		maxSymbols = 30
	}
	return &codeGraphProvider{
		maxSymbols: maxSymbols,
		client:     codegraph.NewClient("codegraph"),
	}
}

func (p *codeGraphProvider) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, nil
	}
	startedAt := time.Now()
	status, err := p.client.EnsureReady(ctx, projectRoot)
	if err != nil {
		return nil, err
	}

	args := []string{"context", "-p", projectRoot, "-n", fmt.Sprintf("%d", p.maxSymbols), "--no-code", codeGraphTask(req)}
	contextOutput, err := p.client.Run(ctx, projectRoot, args...)
	if err != nil {
		repairedStatus, repairErr := p.client.Repair(ctx, projectRoot)
		if repairErr != nil {
			return nil, fmt.Errorf("%w: context failed: %v: %s; repair failed: %v",
				codegraph.ErrNotReady,
				err,
				strings.TrimSpace(contextOutput),
				repairErr,
			)
		}
		status = repairedStatus
		contextOutput, err = p.client.Run(ctx, projectRoot, args...)
		if err != nil {
			return nil, fmt.Errorf("%w: context failed after repair: %v: %s", codegraph.ErrNotReady, err, strings.TrimSpace(contextOutput))
		}
	}

	data := &structuralContextData{
		Source:      structuralProviderCodeGraph,
		FilesFound:  len(req.SeedPaths),
		FilesParsed: len(req.SeedPaths),
		LangCounts:  languageCounts(req.Language, req.SeedPaths),
		Sections: []structuralSection{
			{Title: "CodeGraph Status", Body: trimToMax(status.Output, 2000)},
			{Title: "CodeGraph Context", Body: trimToMax(contextOutput, 12000)},
		},
	}

	logger.Diagnostic("operation complete",
		"operation", "analyzer.codegraph_collect",
		"duration", time.Since(startedAt),
		"context_bytes", len(contextOutput),
		"project_root", projectRoot,
		"initialized", status.Initialized,
		"repaired", status.Repaired,
	)
	return data, nil
}

func codeGraphTask(req structuralContextRequest) string {
	var b strings.Builder
	b.WriteString("Analyze project structure, entry points, key modules, business methods, call relationships, dependency graph, and reusable coding patterns")
	if req.ProjectName != "" {
		b.WriteString(" for project ")
		b.WriteString(req.ProjectName)
	}
	if req.Language != "" {
		b.WriteString(" in ")
		b.WriteString(req.Language)
	}
	if req.Purpose != "" {
		b.WriteString(". Purpose: ")
		b.WriteString(req.Purpose)
	}
	focusPaths := append([]string{}, req.FocusPaths...)
	focusPaths = append(focusPaths, req.SeedPaths...)
	focusPaths = uniqueNonEmptyStrings(focusPaths)
	if len(focusPaths) > 0 {
		b.WriteString(". Focus paths: ")
		b.WriteString(strings.Join(focusPaths, ", "))
	}
	return b.String()
}

func languageCounts(language string, paths []string) map[string]int {
	language = strings.TrimSpace(language)
	if language == "" || len(paths) == 0 {
		return nil
	}
	return map[string]int{language: len(paths)}
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(filepath.ToSlash(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func trimToMax(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "\n...[truncated]"
}
