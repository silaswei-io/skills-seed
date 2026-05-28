package promptfiles

import (
	"os"
	"path/filepath"
	"strings"
)

// Session 管理单次 Agent 调用的临时提示词输入文件。
type Session struct {
	dir string
}

// New 创建用于保存提示词输入文件的临时目录。
func New(prefix string) (*Session, error) {
	dir, err := os.MkdirTemp("", strings.TrimSpace(prefix)+"-*")
	if err != nil {
		return nil, err
	}
	return &Session{dir: dir}, nil
}

// NewIn 在 baseDir 下创建用于保存提示词输入文件的临时目录。
func NewIn(baseDir, prefix string) (*Session, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return New(prefix)
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(baseDir, strings.TrimSpace(prefix)+"-*")
	if err != nil {
		return nil, err
	}
	return &Session{dir: dir}, nil
}

// Cleanup 删除所有临时提示词输入文件。
func (s *Session) Cleanup() {
	if s == nil || s.dir == "" {
		return
	}
	_ = os.RemoveAll(s.dir)
}

// Write 将非空内容写入会话目录并返回文件路径。
func (s *Session) Write(name, content string) (string, error) {
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
func (s *Session) UsePathOrWrite(existingPath, name, content string) (string, error) {
	if strings.TrimSpace(existingPath) != "" {
		return existingPath, nil
	}
	return s.Write(name, content)
}
