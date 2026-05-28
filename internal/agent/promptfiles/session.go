package promptfiles

import (
	"os"
	"path/filepath"
	"strings"
)

// Session owns temporary prompt input files for one agent call.
type Session struct {
	dir string
}

// New creates a temporary directory for prompt input files.
func New(prefix string) (*Session, error) {
	dir, err := os.MkdirTemp("", strings.TrimSpace(prefix)+"-*")
	if err != nil {
		return nil, err
	}
	return &Session{dir: dir}, nil
}

// NewIn creates a temporary directory under baseDir for prompt input files.
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

// Cleanup removes all temporary prompt input files.
func (s *Session) Cleanup() {
	if s == nil || s.dir == "" {
		return
	}
	_ = os.RemoveAll(s.dir)
}

// Write writes non-empty content into the session directory and returns its path.
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

// UsePathOrWrite returns an existing path when present, otherwise writes content
// to the session directory and returns the temporary file path.
func (s *Session) UsePathOrWrite(existingPath, name, content string) (string, error) {
	if strings.TrimSpace(existingPath) != "" {
		return existingPath, nil
	}
	return s.Write(name, content)
}
