package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// PromptInputSession 管理单次 Agent 调用的临时提示词输入文件。
type PromptInputSession struct {
	dir           string
	baseDir       string
	prefix        string
	keepOnCleanup bool
}

// NewPromptInputSession 准备临时提示词输入文件会话，首次写入时才创建目录。
func NewPromptInputSession(prefix string) (*PromptInputSession, error) {
	return &PromptInputSession{prefix: prefix}, nil
}

func newPromptInputSessionIn(baseDir, prefix string) (*PromptInputSession, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return NewPromptInputSession(prefix)
	}
	return &PromptInputSession{baseDir: baseDir, prefix: prefix, keepOnCleanup: true}, nil
}

// Cleanup 删除系统临时输入；项目 runtime 中的实际输入文件保留用于排障。
func (s *PromptInputSession) Cleanup() {
	if s == nil || s.dir == "" {
		return
	}
	if s.keepOnCleanup {
		return
	}
	_ = os.RemoveAll(s.dir)
}

// Write 将非空内容写入会话目录并返回文件路径。
func (s *PromptInputSession) Write(name, content string) (string, error) {
	if s == nil || strings.TrimSpace(content) == "" {
		return "", nil
	}
	createdDir := false
	createdBase := false
	if s.dir == "" {
		var err error
		if s.baseDir == "" {
			s.dir, err = os.MkdirTemp("", strings.TrimSpace(s.prefix)+"-*")
		} else {
			if _, statErr := os.Stat(s.baseDir); errors.Is(statErr, os.ErrNotExist) {
				createdBase = true
			}
			if err = os.MkdirAll(s.baseDir, 0755); err == nil {
				s.dir, err = os.MkdirTemp(s.baseDir, runtimefiles.TempPattern("prompt-input", s.prefix))
			}
		}
		if err != nil {
			if createdBase {
				_ = os.Remove(s.baseDir)
			}
			return "", err
		}
		createdDir = true
	}
	path := filepath.Join(s.dir, name)
	data := strings.TrimSpace(content) + "\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		if createdDir {
			_ = os.RemoveAll(s.dir)
			s.dir = ""
			if createdBase {
				_ = os.Remove(s.baseDir)
			}
		}
		return "", err
	}
	return path, nil
}

// UsePathOrWrite 优先返回已有路径；没有路径时写入内容并返回临时文件路径。
func (s *PromptInputSession) UsePathOrWrite(existingPath, name, content string) (string, error) {
	if strings.TrimSpace(existingPath) != "" {
		return existingPath, nil
	}
	return s.Write(name, content)
}
