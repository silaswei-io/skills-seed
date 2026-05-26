package analyzer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

var (
	errCodeGraphUnavailable    = errors.New("CodeGraph command unavailable")
	errCodeGraphNotInitialized = errors.New("CodeGraph index not initialized")
)

type codeGraphContextRequest struct {
	ProjectName string
	Language    string
	Purpose     string
	FocusPaths  []string
	MaxNodes    int
	MaxCode     int
}

type codeGraphCollector interface {
	Collect(ctx context.Context, projectRoot string, req codeGraphContextRequest) (string, error)
}

type cliCodeGraphCollector struct {
	command  string
	autoInit bool
	autoSync bool
}

func newCodeGraphCollector(cfg config.CodeGraphConfig) codeGraphCollector {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		command = "codegraph"
	}
	return &cliCodeGraphCollector{
		command:  command,
		autoInit: cfg.AutoInit,
		autoSync: cfg.AutoSync,
	}
}

func (c *cliCodeGraphCollector) Collect(ctx context.Context, projectRoot string, req codeGraphContextRequest) (string, error) {
	startedAt := time.Now()
	if _, err := exec.LookPath(c.command); err != nil {
		return "", fmt.Errorf("%w: %s", errCodeGraphUnavailable, c.command)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".codegraph")); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if !c.autoInit {
			return "", fmt.Errorf("%w: run `codegraph init -i` in %s", errCodeGraphNotInitialized, projectRoot)
		}
		if output, err := c.run(ctx, projectRoot, "init", "-i", projectRoot); err != nil {
			return "", fmt.Errorf("CodeGraph init failed: %w: %s", err, strings.TrimSpace(output))
		}
	} else if c.autoSync {
		if output, err := c.run(ctx, projectRoot, "sync", projectRoot); err != nil {
			return "", fmt.Errorf("CodeGraph sync failed: %w: %s", err, strings.TrimSpace(output))
		}
	}

	statusOutput, err := c.run(ctx, projectRoot, "status", projectRoot)
	if err != nil {
		return "", fmt.Errorf("CodeGraph status failed: %w: %s", err, strings.TrimSpace(statusOutput))
	}

	task := codeGraphTask(req)
	args := []string{"context", "-p", projectRoot, "-n", fmt.Sprintf("%d", req.MaxNodes)}
	if req.MaxCode <= 0 {
		args = append(args, "--no-code")
	} else {
		args = append(args, "-c", fmt.Sprintf("%d", req.MaxCode))
	}
	args = append(args, task)
	contextOutput, err := c.run(ctx, projectRoot, args...)
	if err != nil {
		return "", fmt.Errorf("CodeGraph context failed: %w: %s", err, strings.TrimSpace(contextOutput))
	}

	var b strings.Builder
	b.WriteString("## CodeGraph Structural Context\n\n")
	b.WriteString("### Status\n\n")
	b.WriteString(trimToMax(statusOutput, 2000))
	b.WriteString("\n\n### Context\n\n")
	b.WriteString(trimToMax(contextOutput, 12000))

	result := b.String()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "analyzer.codegraph_collect",
		"duration", time.Since(startedAt),
		"context_bytes", len(result),
		"project_root", projectRoot,
	)
	return result, nil
}

func (c *cliCodeGraphCollector) run(ctx context.Context, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.command, args...)
	cmd.Dir = workDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func codeGraphTask(req codeGraphContextRequest) string {
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
	if len(req.FocusPaths) > 0 {
		b.WriteString(". Focus paths: ")
		b.WriteString(strings.Join(req.FocusPaths, ", "))
	}
	return b.String()
}

func trimToMax(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(value[:max]) + "\n...[truncated]"
}
