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
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

var (
	errCodeGraphUnavailable = errors.New("CodeGraph command unavailable")
	errCodeGraphNotReady    = errors.New("CodeGraph index not ready")
)

type codeGraphProvider struct {
	maxSymbols int
	manager    *codeGraphManager
}

func newCodeGraphProvider(cfg config.StructuralConfig) *codeGraphProvider {
	maxSymbols := cfg.MaxSymbols
	if maxSymbols <= 0 {
		maxSymbols = 30
	}
	return &codeGraphProvider{
		maxSymbols: maxSymbols,
		manager:    newCodeGraphManager("codegraph"),
	}
}

func (p *codeGraphProvider) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (*structuralContextData, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, nil
	}
	startedAt := time.Now()
	status, err := p.manager.EnsureReady(ctx, projectRoot)
	if err != nil {
		return nil, err
	}

	args := []string{"context", "-p", projectRoot, "-n", fmt.Sprintf("%d", p.maxSymbols), "--no-code", codeGraphTask(req)}
	contextOutput, err := p.manager.run(ctx, projectRoot, args...)
	if err != nil {
		repairedStatus, repairErr := p.manager.Repair(ctx, projectRoot)
		if repairErr != nil {
			return nil, fmt.Errorf("%w: context failed: %v: %s; repair failed: %v",
				errCodeGraphNotReady,
				err,
				strings.TrimSpace(contextOutput),
				repairErr,
			)
		}
		status = repairedStatus
		contextOutput, err = p.manager.run(ctx, projectRoot, args...)
		if err != nil {
			return nil, fmt.Errorf("%w: context failed after repair: %v: %s", errCodeGraphNotReady, err, strings.TrimSpace(contextOutput))
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

type codeGraphStatus struct {
	Output      string
	Initialized bool
	Repaired    bool
}

type codeGraphCommandRunner interface {
	LookPath(command string) (string, error)
	Run(ctx context.Context, command, workDir string, args ...string) (string, error)
}

type execCodeGraphRunner struct{}

func (execCodeGraphRunner) LookPath(command string) (string, error) {
	return exec.LookPath(command)
}

func (execCodeGraphRunner) Run(ctx context.Context, command, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

type codeGraphManager struct {
	command string
	runner  codeGraphCommandRunner
	locks   *codeGraphProjectLocks
}

func newCodeGraphManager(command string) *codeGraphManager {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "codegraph"
	}
	return &codeGraphManager{
		command: command,
		runner:  execCodeGraphRunner{},
		locks:   globalCodeGraphProjectLocks,
	}
}

func (m *codeGraphManager) EnsureReady(ctx context.Context, projectRoot string) (*codeGraphStatus, error) {
	if _, err := m.runner.LookPath(m.command); err != nil {
		return nil, fmt.Errorf("%w: %s", errCodeGraphUnavailable, m.command)
	}

	unlock := m.locks.lock(projectRoot)
	defer unlock()

	status := &codeGraphStatus{}
	if _, err := os.Stat(filepath.Join(projectRoot, ".codegraph")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if output, err := m.run(ctx, projectRoot, "init", "-i"); err != nil {
			return nil, fmt.Errorf("%w: init failed: %v: %s", errCodeGraphNotReady, err, strings.TrimSpace(output))
		}
		status.Initialized = true
	} else {
		if output, err := m.run(ctx, projectRoot, "sync"); err != nil {
			repairedStatus, repairErr := m.repairLocked(ctx, projectRoot)
			if repairErr != nil {
				return nil, fmt.Errorf("%w: sync failed: %v: %s; repair failed: %v",
					errCodeGraphNotReady,
					err,
					strings.TrimSpace(output),
					repairErr,
				)
			}
			return repairedStatus, nil
		}
	}

	output, err := m.run(ctx, projectRoot, "status")
	if err == nil {
		status.Output = output
		return status, nil
	}

	repairedStatus, repairErr := m.repairLocked(ctx, projectRoot)
	if repairErr != nil {
		return nil, fmt.Errorf("%w: status failed: %v: %s; repair failed: %v",
			errCodeGraphNotReady,
			err,
			strings.TrimSpace(output),
			repairErr,
		)
	}
	return repairedStatus, nil
}

func (m *codeGraphManager) Repair(ctx context.Context, projectRoot string) (*codeGraphStatus, error) {
	if _, err := m.runner.LookPath(m.command); err != nil {
		return nil, fmt.Errorf("%w: %s", errCodeGraphUnavailable, m.command)
	}
	unlock := m.locks.lock(projectRoot)
	defer unlock()
	return m.repairLocked(ctx, projectRoot)
}

func (m *codeGraphManager) repairLocked(ctx context.Context, projectRoot string) (*codeGraphStatus, error) {
	status := &codeGraphStatus{Repaired: true}
	if repairOutput, repairErr := m.run(ctx, projectRoot, "init", "-i"); repairErr != nil {
		return nil, fmt.Errorf("init failed: %v: %s", repairErr, strings.TrimSpace(repairOutput))
	}
	if syncOutput, syncErr := m.run(ctx, projectRoot, "sync"); syncErr != nil {
		return nil, fmt.Errorf("sync failed: %v: %s", syncErr, strings.TrimSpace(syncOutput))
	}
	output, err := m.run(ctx, projectRoot, "status")
	if err != nil {
		return nil, fmt.Errorf("status failed: %v: %s", err, strings.TrimSpace(output))
	}
	status.Output = output
	return status, nil
}

func (m *codeGraphManager) run(ctx context.Context, workDir string, args ...string) (string, error) {
	return m.runner.Run(ctx, m.command, workDir, args...)
}

type codeGraphProjectLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

var globalCodeGraphProjectLocks = &codeGraphProjectLocks{locks: map[string]*sync.Mutex{}}

func (l *codeGraphProjectLocks) lock(projectRoot string) func() {
	projectRoot = filepath.Clean(projectRoot)
	l.mu.Lock()
	projectLock := l.locks[projectRoot]
	if projectLock == nil {
		projectLock = &sync.Mutex{}
		l.locks[projectRoot] = projectLock
	}
	l.mu.Unlock()

	projectLock.Lock()
	return projectLock.Unlock
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
