package claude

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试项目路径 — 使用 skills-seed 自身作为测试项目
var testProjectPath = findTestProjectPath()

func findTestProjectPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// newE2EAgent 创建 E2E 测试用的 ClaudeAgent
func newE2EAgent() *ClaudeAgent {
	loader := prompts.NewLoader("claude", "zh-CN", "")
	return New("claude", 180*time.Second, loader)
}

// skipIfShort 跳过 E2E 测试（-short 模式或 Claude CLI 不可用）
func skipIfShort(t *testing.T) *ClaudeAgent {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	if os.Getenv("SKILLS_SEED_E2E") != "1" {
		t.Skip("Skipping E2E test; set SKILLS_SEED_E2E=1 to run Claude CLI integration tests")
	}
	ag := newE2EAgent()
	if !ag.IsAvailable() {
		t.Skip("Claude CLI not available")
	}
	return ag
}

// ========== 辅助函数：收集真实项目数据 ==========

// getProjectStructure 获取项目目录结构
func getProjectStructure(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("find", testProjectPath, "-maxdepth", "3",
		"-not", "-path", "*/vendor/*",
		"-not", "-path", "*/.git/*",
		"-not", "-path", "*/.skills-seed/*",
		"-type", "f",
		"-name", "*.go",
	)
	out, err := cmd.Output()
	require.NoError(t, err)
	// 转为相对路径
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var relPaths []string
	for _, l := range lines {
		rel, err := filepath.Rel(testProjectPath, l)
		if err == nil {
			relPaths = append(relPaths, rel)
		}
	}
	return strings.Join(relPaths, "\n")
}

// getRecentCommits 获取最近 N 条提交
func getRecentCommits(t *testing.T, n int) []domain.CommitInfo {
	t.Helper()
	cmd := exec.Command("git", "-C", testProjectPath, "log", "--pretty=format:%H|%an|%ai|%s", "-n", stringify(n))
	out, err := cmd.Output()
	require.NoError(t, err)

	var commits []domain.CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[2])
		commits = append(commits, domain.CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    date,
			Message: parts[3],
		})
	}
	return commits
}

// getMainFiles 获取项目入口文件
func getMainFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	entries, err := filepath.Glob(filepath.Join(testProjectPath, "cmd", "*", "main.go"))
	if err == nil && len(entries) > 0 {
		for _, e := range entries {
			rel, _ := filepath.Rel(testProjectPath, e)
			files = append(files, rel)
		}
	}
	// 也检查根目录 main.go
	if _, err := os.Stat(filepath.Join(testProjectPath, "main.go")); err == nil {
		files = append(files, "main.go")
	}
	require.NotEmpty(t, files, "No main.go found in test project")
	return files
}

// stringify 简单的 int → string
func stringify(n int) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(
		strings.Replace(
			strings.Replace(string(rune('0'+n%10)), "", "", -1),
			"", "", -1),
		" "), " "))
}

// ========== E2E 测试：一个模板一个测试 ==========

// TestE2E_Analyze 测试 analyze 模板：渲染 → Claude → 解析
// 模板绑定: AnalyzeRequest (Files, Context, Patterns, RecentCommits)
// 输出格式: {"issues":[...], "suggestions":[...], "confidence":0.85}
func TestE2E_Analyze(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	req := &agent.AnalyzeRequest{
		Files: []domain.FileInfo{
			{Path: "cmd/skills-seed/main.go", Language: "go"},
		},
		Context: agent.ProjectContext{
			Name:     "skills-seed",
			Language: "go",
		},
		Patterns: []domain.Pattern{
			{
				ID:          "error-wrapping",
				Name:        "Error Wrapping",
				Category:    domain.CategoryError,
				Rule:        "Always wrap errors with fmt.Errorf and %w",
				Confidence:  0.9,
				Description: "Wrap errors with context",
			},
		},
		RecentCommits: getRecentCommits(t, 5),
	}

	result, err := ag.AnalyzeCode(ctx, req)
	require.NoError(t, err, "AnalyzeCode should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Issues: %d, Confidence: %.2f", len(result.Issues), result.Confidence)
	for _, issue := range result.Issues {
		t.Logf("  - [%s] %s:%d %s", issue.Severity, issue.File, issue.Line, issue.Message)
	}
}

// TestE2E_BatchLearn 测试 learn-batch 模板：渲染 → Claude → 解析
// 模板绑定: Commits, CommitFiles, KnownPatternsJSON, KnownPatternsCount
// 输出格式: {"patterns":[...]}
func TestE2E_BatchLearn(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	commits := getRecentCommits(t, 3)
	require.NotEmpty(t, commits, "Should have commits to analyze")

	req := &agent.BatchLearnRequest{
		Commits:            commits,
		KnownPatternsJSON:  "[]",
		KnownPatternsCount: 0,
	}

	result, err := ag.BatchLearnFromCommits(ctx, req)
	require.NoError(t, err, "BatchLearnFromCommits should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Patterns learned: %d", len(result.Patterns))
	for _, p := range result.Patterns {
		t.Logf("  - [%s] %s (confidence: %.2f)", p.Category, p.Name, p.Confidence)
	}
}

// TestE2E_LearnFromCommit 测试 learn-batch 模板（单提交模式）：渲染 → Claude → 解析
// 模板绑定: Commit (单条), ChangedFiles, KnownPatternsJSON, KnownPatternsCount
// 输出格式: {"patterns":[...]}
func TestE2E_LearnFromCommit(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	commits := getRecentCommits(t, 1)
	require.NotEmpty(t, commits, "Should have at least one commit")

	req := &agent.LearnRequest{
		Commit:             commits[0],
		KnownPatternsJSON:  "[]",
		KnownPatternsCount: 0,
	}

	result, err := ag.LearnFromCommit(ctx, req)
	require.NoError(t, err, "LearnFromCommit should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Patterns learned: %d", len(result.Patterns))
	for _, p := range result.Patterns {
		t.Logf("  - [%s] %s (confidence: %.2f)", p.Category, p.Name, p.Confidence)
	}
}

// TestE2E_GenerateFixes 测试 fix-generate 模板：渲染 → Claude → 解析
// 模板绑定: Issues, Files, Context
// 输出格式: {"fixes":{"file/path.go":"content"}, "confidence":0.88}
func TestE2E_GenerateFixes(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	req := &agent.GenerateFixesRequest{
		Issues: []domain.Issue{
			{
				File:       "main.go",
				Line:       1,
				Severity:   domain.SeverityWarning,
				Message:    "Missing package comment",
				Suggestion: "Add package documentation comment",
			},
		},
		Files: []domain.FileInfo{
			{Path: "cmd/skills-seed/main.go", Language: "go"},
		},
		Context: agent.ProjectContext{
			Name:     "skills-seed",
			Language: "go",
		},
	}

	result, err := ag.GenerateFixes(ctx, req)
	require.NoError(t, err, "GenerateFixes should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Confidence: %.2f, Fixes: %d files", result.Confidence, len(result.Fixes))
	for path := range result.Fixes {
		t.Logf("  - Fixed: %s", path)
	}
}

// TestE2E_GenerateSkillsSummary 测试 skill-project-summary 模板：渲染 → Claude → 解析
// 模板绑定: PROJECT_NAME, LANGUAGE, PATTERNS_JSON, PATTERNS_COUNT, EXISTING_SKILLS_PATH
// 输出格式: {"category_summaries":{...}, "key_patterns":[...], ...}
func TestE2E_GenerateSkillsSummary(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	patternsJSON := `[{
		"id": "err-wrap",
		"name": "Error Wrapping",
		"category": "error",
		"description": "Always wrap errors with context",
		"rule": "Use fmt.Errorf with %w to wrap errors",
		"confidence": 0.92,
		"frequency": 10
	}, {
		"id": "gorm-transaction",
		"name": "GORM Transaction",
		"category": "database",
		"description": "Use GORM Transaction method",
		"rule": "Use db.Transaction() for multi-table operations",
		"confidence": 0.88,
		"frequency": 6
	}]`

	req := &agent.GenerateSkillsRequest{
		PatternsJSON:  patternsJSON,
		PatternsCount: 2,
		ProjectName:   "skills-seed",
		Language:      "go",
	}

	result, err := ag.GenerateSkillsSummary(ctx, req)
	require.NoError(t, err, "GenerateSkillsSummary should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Categories: %d, BusinessRules: %d, BestPractices: %d",
		len(result.CategorySummaries), len(result.BusinessRules), len(result.BestPractices))
	t.Logf("KeyPatterns: %d, CommonPatterns: %d",
		len(result.KeyPatterns), len(result.CommonPatterns))
}

// TestE2E_MergePatterns 测试 pattern-merge 模板：渲染 → Claude → 解析
// 模板绑定: Category, Patterns
// 输出格式: {"merged_patterns":[...], "unchanged_patterns":[...], "summary":{...}}
func TestE2E_MergePatterns(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	p1 := domain.NewPattern("err-01", "Error Wrapping", domain.CategoryError)
	p1.SetDescription("Use fmt.Errorf with %w to wrap errors")
	p1.SetRule("Always wrap errors with context")
	p1.Confidence = 0.85
	p1.Frequency = 5

	p2 := domain.NewPattern("err-02", "Error Checking", domain.CategoryError)
	p2.SetDescription("Always check error return values")
	p2.SetRule("Check all error returns")
	p2.Confidence = 0.80
	p2.Frequency = 3

	req := &agent.MergePatternsRequest{
		Category: "error",
		Patterns: []domain.Pattern{*p1, *p2},
	}

	result, err := ag.MergePatterns(ctx, req)
	require.NoError(t, err, "MergePatterns should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Merged: %d, Unchanged: %d, TotalInput: %d",
		len(result.MergedPatterns), len(result.UnchangedPatterns), result.Summary.TotalInput)
	for _, mp := range result.MergedPatterns {
		t.Logf("  Merged: %s (from %v)", mp.Name, mp.MergedFrom)
	}
}

// TestE2E_ProjectAnalysis 测试 project-analyze 模板：渲染 → Claude → 解析
// 模板绑定: ProjectName, RootPath, Structure, ReadmePath, MainFiles
// 输出格式: {"project_name":"...", "language":"go", "frameworks":[...], ...}
func TestE2E_ProjectAnalysis(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	structure := getProjectStructure(t)
	mainFiles := getMainFiles(t)

	req := &agent.AnalyzeProjectRequest{
		ProjectName: "skills-seed",
		RootPath:    testProjectPath,
		Structure:   structure,
		ReadmePath:  "README.md",
		MainFiles:   mainFiles,
	}

	result, err := ag.AnalyzeProject(ctx, req)
	require.NoError(t, err, "AnalyzeProject should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Language: %s, Architecture: %s", result.Language, result.Architecture)
	t.Logf("Frameworks: %v", result.Frameworks)
	t.Logf("Summary: %s", result.Summary)
	assert.NotEmpty(t, result.Language, "Language should not be empty")
	assert.NotEmpty(t, result.Summary, "Summary should not be empty")
}

// TestE2E_InitSkills 测试 skill-project-init 模板：渲染 → Claude → 解析
// 模板绑定: ProjectName, RootPath, Language, Structure, MainFiles, SampleFiles
// 输出格式: {"patterns":[...], "category_summaries":{...}, ...}
func TestE2E_InitSkills(t *testing.T) {
	ag := skipIfShort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	structure := getProjectStructure(t)
	mainFiles := getMainFiles(t)

	// 收集示例文件（选几个代表性的）
	sampleFiles := []agent.SampleFile{
		{Path: "cmd/skills-seed/main.go"},
	}

	// 尝试读取 domain 层的示例文件
	domainDir := filepath.Join(testProjectPath, "internal", "domain")
	if entries, err := os.ReadDir(domainDir); err == nil && len(entries) > 0 {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				sampleFiles = append(sampleFiles, agent.SampleFile{
					Path: filepath.Join("internal", "domain", e.Name()),
				})
				if len(sampleFiles) >= 3 {
					break
				}
			}
		}
	}

	req := &agent.AnalyzeCurrentCodebaseRequest{
		ProjectName: "skills-seed",
		RootPath:    testProjectPath,
		Language:    "go",
		Structure:   structure,
		MainFiles:   mainFiles,
		SampleFiles: sampleFiles,
	}

	result, err := ag.AnalyzeCurrentCodebase(ctx, req)
	require.NoError(t, err, "AnalyzeCurrentCodebase should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Patterns: %d, BusinessRules: %d, BestPractices: %d",
		len(result.Patterns), len(result.BusinessRules), len(result.BestPractices))
	for _, p := range result.Patterns {
		t.Logf("  - [%s] %s (confidence: %.2f)", p.Category, p.Name, p.Confidence)
	}
	assert.NotEmpty(t, result.Summary, "Summary should not be empty")
}
