// Package domain 提供核心领域模型的单元测试
package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== Pattern Tests ====================

func TestNewPattern(t *testing.T) {
	beforeCreate := time.Now()
	p := NewPattern("p1", "Use descriptive names", CategoryNaming)
	afterCreate := time.Now()

	assert.Equal(t, "p1", p.ID, "ID should be initialized")
	assert.Equal(t, "Use descriptive names", p.Name, "Name should be initialized")
	assert.Equal(t, CategoryNaming, p.Category, "Category should be initialized")
	assert.Equal(t, 0.0, p.Confidence, "Confidence should be 0")
	assert.Equal(t, 0, p.Frequency, "Frequency should be 0")
	assert.Equal(t, SourceLearned, p.Source, "Source should be SourceLearned")
	assert.False(t, p.Merged, "Merged should be false")
	assert.False(t, p.Generated, "Generated should be false")
	assert.Equal(t, []string{}, p.MergedFrom, "MergedFrom should be empty slice")
	assert.Nil(t, p.BusinessMethod, "BusinessMethod should be nil")
	assert.True(t, !p.CreatedAt.Before(beforeCreate), "CreatedAt should not be before creation")
	assert.True(t, !p.CreatedAt.After(afterCreate), "CreatedAt should not be after creation")
	assert.True(t, !p.UpdatedAt.Before(beforeCreate), "UpdatedAt should not be before creation")
	assert.True(t, !p.UpdatedAt.After(afterCreate), "UpdatedAt should not be after creation")
}

func TestPattern_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		pattern *Pattern
		want    bool
	}{
		{
			name: "valid pattern",
			pattern: &Pattern{
				ID:         "p1",
				Name:       "Use descriptive names",
				Category:   CategoryNaming,
				Confidence: 0.8,
			},
			want: true,
		},
		{
			name: "empty ID",
			pattern: &Pattern{
				ID:         "",
				Name:       "Use descriptive names",
				Category:   CategoryNaming,
				Confidence: 0.8,
			},
			want: false,
		},
		{
			name: "empty name",
			pattern: &Pattern{
				ID:         "p1",
				Name:       "",
				Category:   CategoryNaming,
				Confidence: 0.8,
			},
			want: false,
		},
		{
			name: "empty category",
			pattern: &Pattern{
				ID:         "p1",
				Name:       "Use descriptive names",
				Category:   "",
				Confidence: 0.8,
			},
			want: false,
		},
		{
			name: "negative confidence",
			pattern: &Pattern{
				ID:         "p1",
				Name:       "Use descriptive names",
				Category:   CategoryNaming,
				Confidence: -0.1,
			},
			want: false,
		},
		{
			name: "confidence greater than 1.0",
			pattern: &Pattern{
				ID:         "p1",
				Name:       "Use descriptive names",
				Category:   CategoryNaming,
				Confidence: 1.1,
			},
			want: false,
		},
		{
			name: "zero confidence is valid",
			pattern: &Pattern{
				ID:         "p1",
				Name:       "Use descriptive names",
				Category:   CategoryNaming,
				Confidence: 0.0,
			},
			want: true,
		},
		{
			name: "confidence exactly 1.0 is valid",
			pattern: &Pattern{
				ID:         "p1",
				Name:       "Use descriptive names",
				Category:   CategoryNaming,
				Confidence: 1.0,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.pattern.IsValid())
		})
	}
}

func TestPattern_UpdateConfidence(t *testing.T) {
	p := &Pattern{
		Confidence: 0.8,
		Frequency:  2,
	}

	p.UpdateConfidence(0.9)

	// Weighted average: (0.8*2 + 0.9) / 3 = 2.5/3 = 0.8333...
	expectedConfidence := (0.8*2 + 0.9) / 3
	assert.InDelta(t, expectedConfidence, p.Confidence, 0.0001, "Confidence should be weighted average")
	assert.Equal(t, 3, p.Frequency, "Frequency should be incremented")
}

func TestPattern_SetExamples(t *testing.T) {
	p := &Pattern{}

	p.SetExamples("good code", "bad code")

	assert.Equal(t, "good code", p.GoodExample, "GoodExample should be set")
	assert.Equal(t, "bad code", p.BadExample, "BadExample should be set")
}

func TestPattern_SetDescription(t *testing.T) {
	p := &Pattern{}

	p.SetDescription("A pattern about naming")

	assert.Equal(t, "A pattern about naming", p.Description, "Description should be set")
}

func TestPattern_SetRule(t *testing.T) {
	p := &Pattern{}

	p.SetRule("Always use camelCase")

	assert.Equal(t, "Always use camelCase", p.Rule, "Rule should be set")
}

func TestPattern_Merge(t *testing.T) {
	t.Run("merges examples from other when current is empty", func(t *testing.T) {
		p := &Pattern{
			Confidence: 0.7,
			Frequency:  3,
		}
		other := &Pattern{
			GoodExample: "good example",
			BadExample:  "bad example",
			Description: "some description",
			Confidence:  0.9,
			Frequency:   1,
		}

		p.Merge(other)

		assert.Equal(t, "good example", p.GoodExample, "Should take GoodExample from other")
		assert.Equal(t, "bad example", p.BadExample, "Should take BadExample from other")
		assert.Equal(t, "some description", p.Description, "Should take Description from other")
	})

	t.Run("weighted average of confidence and frequency addition", func(t *testing.T) {
		p := &Pattern{
			GoodExample: "existing good",
			BadExample:  "existing bad",
			Confidence:  0.8,
			Frequency:   4,
		}
		other := &Pattern{
			GoodExample: "other good",
			BadExample:  "other bad",
			Confidence:  0.6,
			Frequency:   2,
		}

		p.Merge(other)

		// Should not overwrite existing examples
		assert.Equal(t, "existing good", p.GoodExample, "Should keep current GoodExample")
		assert.Equal(t, "existing bad", p.BadExample, "Should keep current BadExample")
		// Weighted average: (0.8*4 + 0.6*2) / (4+2) = 4.4/6 = 0.7333...
		expectedConfidence := (0.8*4 + 0.6*2) / 6.0
		assert.InDelta(t, expectedConfidence, p.Confidence, 0.0001, "Confidence should be weighted average")
		assert.Equal(t, 6, p.Frequency, "Frequency should be sum of both")
	})
}

func TestPattern_IsSimilar(t *testing.T) {
	tests := []struct {
		name    string
		p1      *Pattern
		p2      *Pattern
		similar bool
	}{
		{
			name:    "same name and category",
			p1:      &Pattern{Name: "Use descriptive names", Category: CategoryNaming},
			p2:      &Pattern{Name: "Use descriptive names", Category: CategoryNaming},
			similar: true,
		},
		{
			name:    "different name",
			p1:      &Pattern{Name: "Use descriptive names", Category: CategoryNaming},
			p2:      &Pattern{Name: "Avoid abbreviations", Category: CategoryNaming},
			similar: false,
		},
		{
			name:    "different category",
			p1:      &Pattern{Name: "Use descriptive names", Category: CategoryNaming},
			p2:      &Pattern{Name: "Use descriptive names", Category: CategoryError},
			similar: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.similar, tt.p1.IsSimilar(tt.p2))
		})
	}
}

func TestPattern_SetBusinessMethod(t *testing.T) {
	p := &Pattern{}
	beforeUpdate := p.UpdatedAt

	method := &BusinessMethod{
		Name:        "GenerateUUID",
		Location:    "internal/utils/uuid.go:15",
		Description: "Generates a new UUID v4",
		Usage:       "Use when creating new entity IDs",
		Type:        "common",
	}

	// Small sleep to ensure UpdatedAt changes
	time.Sleep(time.Millisecond)

	p.SetBusinessMethod(method)

	assert.Equal(t, method, p.BusinessMethod, "BusinessMethod should be set")
	assert.True(t, p.UpdatedAt.After(beforeUpdate), "UpdatedAt should be updated")
}

// ==================== Issue Tests ====================

func TestNewIssue(t *testing.T) {
	issue := NewIssue("main.go", 42, SeverityError, "unused variable")

	assert.Equal(t, "main.go", issue.File, "File should be set")
	assert.Equal(t, 42, issue.Line, "Line should be set")
	assert.Equal(t, SeverityError, issue.Severity, "Severity should be set")
	assert.Equal(t, "unused variable", issue.Message, "Message should be set")
	assert.Equal(t, "", issue.Suggestion, "Suggestion should be empty by default")
	assert.Equal(t, "", issue.PatternID, "PatternID should be empty by default")
	assert.Equal(t, 0, issue.Column, "Column should be 0 by default")
	assert.Equal(t, 0.0, issue.Confidence, "Confidence should be 0 by default")
}

func TestIssue_IsError(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     bool
	}{
		{
			name:     "severity is error",
			severity: SeverityError,
			want:     true,
		},
		{
			name:     "severity is warning",
			severity: SeverityWarning,
			want:     false,
		},
		{
			name:     "severity is info",
			severity: SeverityInfo,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &Issue{Severity: tt.severity}
			assert.Equal(t, tt.want, issue.IsError())
		})
	}
}

func TestIssue_IsWarning(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     bool
	}{
		{
			name:     "severity is warning",
			severity: SeverityWarning,
			want:     true,
		},
		{
			name:     "severity is error",
			severity: SeverityError,
			want:     false,
		},
		{
			name:     "severity is info",
			severity: SeverityInfo,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &Issue{Severity: tt.severity}
			assert.Equal(t, tt.want, issue.IsWarning())
		})
	}
}

func TestIssue_SetSuggestion(t *testing.T) {
	issue := &Issue{}

	issue.SetSuggestion("Remove the unused variable")

	assert.Equal(t, "Remove the unused variable", issue.Suggestion, "Suggestion should be set")
}

func TestIssue_SetPatternID(t *testing.T) {
	issue := &Issue{}

	issue.SetPatternID("pattern-123")

	assert.Equal(t, "pattern-123", issue.PatternID, "PatternID should be set")
}

// ==================== CommitInfo Tests ====================

func TestNewCommitInfo(t *testing.T) {
	date := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	info := NewCommitInfo("abc123def", "John Doe", "feat: add feature", date)

	assert.Equal(t, "abc123def", info.Hash, "Hash should be set")
	assert.Equal(t, "John Doe", info.Author, "Author should be set")
	assert.Equal(t, "feat: add feature", info.Message, "Message should be set")
	assert.Equal(t, date, info.Date, "Date should be set")
}

func TestCommitInfo_IsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		info  CommitInfo
		empty bool
	}{
		{
			name:  "empty hash",
			info:  CommitInfo{Hash: ""},
			empty: true,
		},
		{
			name:  "non-empty hash",
			info:  CommitInfo{Hash: "abc123"},
			empty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.empty, tt.info.IsEmpty())
		})
	}
}

func TestCommitInfo_ShortHash(t *testing.T) {
	tests := []struct {
		name      string
		hash      string
		shortHash string
	}{
		{
			name:      "hash longer than 7 chars",
			hash:      "abc123def456789",
			shortHash: "abc123d",
		},
		{
			name:      "hash exactly 7 chars",
			hash:      "abc123d",
			shortHash: "abc123d",
		},
		{
			name:      "hash shorter than 7 chars",
			hash:      "abc",
			shortHash: "abc",
		},
		{
			name:      "empty hash",
			hash:      "",
			shortHash: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := CommitInfo{Hash: tt.hash}
			assert.Equal(t, tt.shortHash, info.ShortHash())
		})
	}
}

func TestCommitInfo_Summary(t *testing.T) {
	tests := []struct {
		name    string
		message string
		summary string
	}{
		{
			name:    "single line message",
			message: "feat: add feature",
			summary: "feat: add feature",
		},
		{
			name:    "multi-line message",
			message: "feat: add feature\n\nThis is a detailed description.",
			summary: "feat: add feature",
		},
		{
			name:    "empty message",
			message: "",
			summary: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := CommitInfo{Message: tt.message}
			assert.Equal(t, tt.summary, info.Summary())
		})
	}
}

// ==================== FileInfo Tests ====================

func TestNewFileInfo(t *testing.T) {
	fi := NewFileInfo("main.go", "package main")

	assert.Equal(t, "main.go", fi.Path, "Path should be set")
	assert.Equal(t, "package main", fi.Content, "Content should be set")
	assert.Equal(t, "go", fi.Language, "Language should be detected as go")
	assert.Equal(t, StatusModified, fi.Status, "Status should be StatusModified by default")
}

func TestFileInfo_IsGoFile(t *testing.T) {
	tests := []struct {
		name     string
		fileInfo FileInfo
		goFile   bool
	}{
		{
			name:     "go file",
			fileInfo: FileInfo{Language: "go"},
			goFile:   true,
		},
		{
			name:     "python file",
			fileInfo: FileInfo{Language: "python"},
			goFile:   false,
		},
		{
			name:     "empty language",
			fileInfo: FileInfo{Language: ""},
			goFile:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.goFile, tt.fileInfo.IsGoFile())
		})
	}
}

func TestFileInfo_IsTestFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		testFile bool
	}{
		{
			name:     "test file with _test.go",
			path:     "service_test.go",
			testFile: true,
		},
		{
			name:     "regular go file",
			path:     "service.go",
			testFile: false,
		},
		{
			name:     "test file with path prefix",
			path:     "internal/domain/models_test.go",
			testFile: true,
		},
		{
			name:     "short filename",
			path:     "x.go",
			testFile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := FileInfo{Path: tt.path}
			assert.Equal(t, tt.testFile, fi.IsTestFile())
		})
	}
}

func TestFileInfo_IsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		content string
		isEmpty bool
	}{
		{
			name:    "empty content",
			content: "",
			isEmpty: true,
		},
		{
			name:    "non-empty content",
			content: "package main",
			isEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := FileInfo{Content: tt.content}
			assert.Equal(t, tt.isEmpty, fi.IsEmpty())
		})
	}
}

func TestFileInfo_LineCount(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		lineCount int
	}{
		{
			name:      "three lines",
			content:   "a\nb\nc",
			lineCount: 3,
		},
		{
			name:      "empty content",
			content:   "",
			lineCount: 1,
		},
		{
			name:      "single line without newline",
			content:   "hello",
			lineCount: 1,
		},
		{
			name:      "single line with trailing newline",
			content:   "hello\n",
			lineCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := FileInfo{Content: tt.content}
			assert.Equal(t, tt.lineCount, fi.LineCount())
		})
	}
}

// ==================== detectLanguage Tests (via NewFileInfo) ====================

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		language string
	}{
		{
			name:     "go file",
			path:     "main.go",
			language: "go",
		},
		{
			name:     "javascript file",
			path:     "app.js",
			language: "javascript",
		},
		{
			name:     "jsx file",
			path:     "component.jsx",
			language: "javascript",
		},
		{
			name:     "typescript file",
			path:     "index.ts",
			language: "typescript",
		},
		{
			name:     "tsx file",
			path:     "app.tsx",
			language: "typescript",
		},
		{
			name:     "python file",
			path:     "script.py",
			language: "python",
		},
		{
			name:     "java file",
			path:     "Main.java",
			language: "java",
		},
		{
			name:     "rust file",
			path:     "main.rs",
			language: "rust",
		},
		{
			name:     "Dockerfile by name",
			path:     "Dockerfile",
			language: "dockerfile",
		},
		{
			name:     "Makefile by name",
			path:     "Makefile",
			language: "makefile",
		},
		{
			name:     "unknown extension",
			path:     "data.xyz",
			language: "xyz",
		},
		{
			name:     "empty path",
			path:     "",
			language: "",
		},
		{
			name:     "go.mod file returns mod (extension matched first)",
			path:     "go.mod",
			language: "mod",
		},
		{
			name:     "go.sum file returns sum (extension matched first)",
			path:     "go.sum",
			language: "sum",
		},
		{
			name:     "yaml file",
			path:     "config.yaml",
			language: "yaml",
		},
		{
			name:     "yml file",
			path:     "config.yml",
			language: "yaml",
		},
		{
			name:     "json file",
			path:     "package.json",
			language: "json",
		},
		{
			name:     "shell file",
			path:     "run.sh",
			language: "shell",
		},
		{
			name:     "markdown file",
			path:     "README.md",
			language: "markdown",
		},
		{
			name:     "sql file",
			path:     "query.sql",
			language: "sql",
		},
		{
			name:     "ruby file",
			path:     "app.rb",
			language: "ruby",
		},
		{
			name:     "php file",
			path:     "index.php",
			language: "php",
		},
		{
			name:     "swift file",
			path:     "main.swift",
			language: "swift",
		},
		{
			name:     "kotlin file",
			path:     "Main.kt",
			language: "kotlin",
		},
		{
			name:     "scala file",
			path:     "App.scala",
			language: "scala",
		},
		{
			name:     "cpp file",
			path:     "main.cpp",
			language: "cpp",
		},
		{
			name:     "c file",
			path:     "main.c",
			language: "c",
		},
		{
			name:     "xml file",
			path:     "config.xml",
			language: "xml",
		},
		{
			name:     "path with directory",
			path:     "internal/domain/models.go",
			language: "go",
		},
		{
			name:     "no extension no special name",
			path:     "LICENSE",
			language: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := NewFileInfo(tt.path, "")
			assert.Equal(t, tt.language, fi.Language)
		})
	}
}

// ==================== Edge Case Tests ====================

func TestPattern_Merge_EdgeCases(t *testing.T) {
	t.Run("merge with both having empty examples", func(t *testing.T) {
		p := &Pattern{Confidence: 0.5, Frequency: 1}
		other := &Pattern{Confidence: 0.7, Frequency: 1}

		p.Merge(other)

		assert.Equal(t, "", p.GoodExample, "GoodExample should remain empty")
		assert.Equal(t, "", p.BadExample, "BadExample should remain empty")
		assert.Equal(t, 2, p.Frequency, "Frequency should be sum")
	})

	t.Run("merge with zero frequencies", func(t *testing.T) {
		p := &Pattern{Confidence: 0.0, Frequency: 0}
		other := &Pattern{Confidence: 0.5, Frequency: 0}

		p.Merge(other)

		assert.Equal(t, 0.0, p.Confidence, "Confidence should be 0 with zero total frequency")
		assert.Equal(t, 0, p.Frequency, "Frequency should be 0+0")
	})
}

func TestCommitInfo_ShortHash_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		hash      string
		shortHash string
	}{
		{
			name:      "exactly 8 chars",
			hash:      "abcdef12",
			shortHash: "abcdef1",
		},
		{
			name:      "single char",
			hash:      "a",
			shortHash: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := CommitInfo{Hash: tt.hash}
			assert.Equal(t, tt.shortHash, info.ShortHash())
		})
	}
}

func TestFileInfo_LineCount_MultipleEmptyLines(t *testing.T) {
	fi := FileInfo{Content: "\n\n\n"}
	assert.Equal(t, 4, fi.LineCount(), "Three newlines should give 4 lines")
}

func TestFileInfo_IsTestFile_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "exactly 8 chars ending in _test.go",
			path:     "a_test.go",
			expected: true,
		},
		{
			name:     "7 chars cannot be test file",
			path:     "test.go",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := FileInfo{Path: tt.path}
			assert.Equal(t, tt.expected, fi.IsTestFile())
		})
	}
}

func TestPattern_UpdateConfidence_MultipleUpdates(t *testing.T) {
	p := &Pattern{Confidence: 0.0, Frequency: 0}

	p.UpdateConfidence(1.0) // (0*0 + 1.0) / 1 = 1.0
	assert.InDelta(t, 1.0, p.Confidence, 0.0001)
	assert.Equal(t, 1, p.Frequency)

	p.UpdateConfidence(0.5) // (1.0*1 + 0.5) / 2 = 0.75
	assert.InDelta(t, 0.75, p.Confidence, 0.0001)
	assert.Equal(t, 2, p.Frequency)

	p.UpdateConfidence(0.0) // (0.75*2 + 0.0) / 3 = 0.5
	assert.InDelta(t, 0.5, p.Confidence, 0.0001)
	assert.Equal(t, 3, p.Frequency)
}

func TestNewPattern_AllCategories(t *testing.T) {
	categories := []Category{
		CategoryNaming, CategoryError, CategoryStructure,
		CategoryConcurrency, CategoryTesting, CategoryBusiness,
		CategoryAPI, CategoryDatabase, CategoryUtils, CategoryMiddleware,
		CategoryConfig,
	}

	for _, cat := range categories {
		t.Run(string(cat), func(t *testing.T) {
			p := NewPattern("id", "name", cat)
			assert.Equal(t, cat, p.Category)
			assert.True(t, p.IsValid(), "Pattern with category %s should be valid", cat)
		})
	}
}

func TestCommitInfo_Summary_WithMultipleNewlines(t *testing.T) {
	message := "first line\nsecond line\nthird line"
	info := CommitInfo{Message: message}
	assert.Equal(t, "first line", info.Summary(), "Should return only the first line")
}

func TestCommitInfo_Summary_WithOnlyNewline(t *testing.T) {
	info := CommitInfo{Message: "\n"}
	assert.Equal(t, "", info.Summary(), "Should return empty string for message starting with newline")
}

func TestDetectLanguage_FileWithDirectoryPath(t *testing.T) {
	fi := NewFileInfo("cmd/server/main.go", "")
	assert.Equal(t, "go", fi.Language, "Should detect go from path with directories")
}

func TestDetectLanguage_FileWithDirectoryPath_NoExt(t *testing.T) {
	// "cmd/server/Makefile" does not match the exact string "Makefile" check,
	// so detectLanguage returns empty string. Only bare "Makefile" matches.
	fi := NewFileInfo("cmd/server/Makefile", "")
	assert.Equal(t, "", fi.Language, "Makefile with directory prefix does not match special filename check")
}

func TestFileInfo_LineCount_LargeContent(t *testing.T) {
	lineCount := 10000
	content := strings.Repeat("line\n", lineCount-1) + "last line"
	fi := FileInfo{Content: content}
	assert.Equal(t, lineCount, fi.LineCount(), "Should correctly count 10000 lines")
}
