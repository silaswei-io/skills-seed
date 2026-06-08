package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// PromptInputSession 管理单次 Agent 调用的临时提示词输入文件。
type PromptInputSession struct {
	dir           string
	keepOnCleanup bool
}

// NewPromptInputSession 创建临时提示词输入文件会话。
func NewPromptInputSession(prefix string) (*PromptInputSession, error) {
	dir, err := os.MkdirTemp("", strings.TrimSpace(prefix)+"-*")
	if err != nil {
		return nil, err
	}
	return &PromptInputSession{dir: dir}, nil
}

func newPromptInputSessionIn(baseDir, prefix string) (*PromptInputSession, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return NewPromptInputSession(prefix)
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(baseDir, strings.TrimSpace(prefix)+"-*")
	if err != nil {
		return nil, err
	}
	return &PromptInputSession{dir: dir, keepOnCleanup: true}, nil
}

// Cleanup 删除所有临时提示词输入文件。
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
	if s == nil || s.dir == "" || strings.TrimSpace(content) == "" {
		return "", nil
	}
	path := filepath.Join(s.dir, name)
	data := strings.TrimSpace(content) + "\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
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
