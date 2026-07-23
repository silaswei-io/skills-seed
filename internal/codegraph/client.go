// Package codegraph 提供 CodeGraph CLI 的索引生命周期与只读查询能力。
package codegraph

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
)

var (
	ErrUnavailable = errors.New("CodeGraph command unavailable")
	ErrNotReady    = errors.New("CodeGraph index not ready")
)

// Status 描述本次索引准备动作。
type Status struct {
	Output      string
	Initialized bool
	Repaired    bool
}

type runner interface {
	LookPath(command string) (string, error)
	Run(ctx context.Context, command, workDir string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) LookPath(command string) (string, error) {
	return exec.LookPath(command)
}

func (execRunner) Run(ctx context.Context, command, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// Client 管理 CodeGraph 索引生命周期与命令执行。
type Client struct {
	command string
	runner  runner
	locks   *projectLocks
}

// NewClient 创建 CodeGraph CLI 客户端。
func NewClient(command string) *Client {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "codegraph"
	}
	return &Client{command: command, runner: execRunner{}, locks: globalProjectLocks}
}

// EnsureReady 初始化或同步目标项目索引。
func (c *Client) EnsureReady(ctx context.Context, projectRoot string) (*Status, error) {
	if _, err := c.runner.LookPath(c.command); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, c.command)
	}

	unlock := c.locks.lock(projectRoot)
	defer unlock()

	status := &Status{}
	if _, err := os.Stat(filepath.Join(projectRoot, ".codegraph")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if output, err := c.Run(ctx, projectRoot, "init", "-i"); err != nil {
			return nil, fmt.Errorf("%w: init failed: %v: %s", ErrNotReady, err, strings.TrimSpace(output))
		}
		status.Initialized = true
	} else if output, err := c.Run(ctx, projectRoot, "sync"); err != nil {
		repaired, repairErr := c.repairLocked(ctx, projectRoot)
		if repairErr != nil {
			return nil, fmt.Errorf("%w: sync failed: %v: %s; repair failed: %v", ErrNotReady, err, strings.TrimSpace(output), repairErr)
		}
		return repaired, nil
	}

	output, err := c.Run(ctx, projectRoot, "status")
	if err == nil {
		status.Output = output
		return status, nil
	}
	repaired, repairErr := c.repairLocked(ctx, projectRoot)
	if repairErr != nil {
		return nil, fmt.Errorf("%w: status failed: %v: %s; repair failed: %v", ErrNotReady, err, strings.TrimSpace(output), repairErr)
	}
	return repaired, nil
}

// Repair 重建并同步目标项目索引。
func (c *Client) Repair(ctx context.Context, projectRoot string) (*Status, error) {
	if _, err := c.runner.LookPath(c.command); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, c.command)
	}
	unlock := c.locks.lock(projectRoot)
	defer unlock()
	return c.repairLocked(ctx, projectRoot)
}

func (c *Client) repairLocked(ctx context.Context, projectRoot string) (*Status, error) {
	if output, err := c.Run(ctx, projectRoot, "init", "-i"); err != nil {
		return nil, fmt.Errorf("init failed: %v: %s", err, strings.TrimSpace(output))
	}
	if output, err := c.Run(ctx, projectRoot, "sync"); err != nil {
		return nil, fmt.Errorf("sync failed: %v: %s", err, strings.TrimSpace(output))
	}
	output, err := c.Run(ctx, projectRoot, "status")
	if err != nil {
		return nil, fmt.Errorf("status failed: %v: %s", err, strings.TrimSpace(output))
	}
	return &Status{Output: output, Repaired: true}, nil
}

// Run 执行一条 CodeGraph CLI 命令。
func (c *Client) Run(ctx context.Context, projectRoot string, args ...string) (string, error) {
	return c.runner.Run(ctx, c.command, projectRoot, args...)
}

type projectLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

var globalProjectLocks = &projectLocks{locks: map[string]*sync.Mutex{}}

func (l *projectLocks) lock(projectRoot string) func() {
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
