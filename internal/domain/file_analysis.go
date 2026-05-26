package domain

import (
	"path/filepath"
	"strings"
)

const (
	// FileAnalysisHashMD5 表示当前文件增量学习使用 md5 指纹。
	FileAnalysisHashMD5 = "md5"
	// FileAnalysisSourceCurrentCode 表示记录来自 learn current。
	FileAnalysisSourceCurrentCode = "current_code"
)

// FileAnalysisScope 表示文件分析记录的隔离范围。
type FileAnalysisScope struct {
	ProjectID string `json:"project_id,omitempty"`
	ScopePath string `json:"scope_path,omitempty"`
}

// KeyForPath 返回当前 scope 下某个相对路径的稳定存储 key。
func (s FileAnalysisScope) KeyForPath(path string) string {
	return s.KeyPrefix() + normalizeAnalysisPath(path)
}

// KeyPrefix 返回当前 scope 的存储 key 前缀。
func (s FileAnalysisScope) KeyPrefix() string {
	return s.ProjectID + "\x00" + normalizeAnalysisPath(s.ScopePath) + "\x00"
}

// ContainsPath 判断 path 是否落在 focusPaths 指定的相对范围内。
func (s FileAnalysisScope) ContainsPath(path string, focusPaths []string) bool {
	if len(focusPaths) == 0 {
		return true
	}
	path = normalizeAnalysisPath(path)
	for _, focusPath := range focusPaths {
		focusPath = normalizeAnalysisPath(focusPath)
		if focusPath == "." || focusPath == "" || path == focusPath || strings.HasPrefix(path, focusPath+"/") {
			return true
		}
	}
	return false
}

// FileAnalysisRecord 保存单个文件最近一次成功分析时的指纹。
type FileAnalysisRecord struct {
	ProjectID      string `json:"project_id,omitempty"`
	ScopePath      string `json:"scope_path,omitempty"`
	Path           string `json:"path"`
	Hash           string `json:"hash"`
	HashAlgorithm  string `json:"hash_algorithm"`
	Size           int64  `json:"size"`
	ModTime        string `json:"mod_time"`
	Source         string `json:"source"`
	LastAnalyzedAt string `json:"last_analyzed_at"`
}

func normalizeAnalysisPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." {
		return ""
	}
	return strings.TrimPrefix(path, "./")
}
