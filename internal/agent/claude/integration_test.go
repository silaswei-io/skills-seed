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
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts"
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
	loader := promptloader.New("claude", "zh-CN", "")
	return New("claude", 180*time.Second, loader, false, config.DefaultRetryConfig(), config.AgentRuntimeOptions{})
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

// ========== E2E 测试：一个核心模板一个测试 ==========

// TestE2E_ProjectAnalysis 测试会话式项目画像刷新：渲染 → Claude → 解析
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

	session, err := ag.StartLearningSession(ctx, agent.LearningSessionRequest{
		ProjectName: "skills-seed",
		RootPath:    testProjectPath,
		Language:    "go",
	})
	require.NoError(t, err)
	defer session.Close(ctx)

	result, err := session.RefreshProjectProfile(ctx, req)
	require.NoError(t, err, "AnalyzeProject should succeed")
	require.NotNil(t, result, "Result should not be nil")

	t.Logf("Language: %s, Architecture: %s", result.Language, result.Architecture)
	t.Logf("Frameworks: %v", result.Frameworks)
	t.Logf("Summary: %s", result.Summary)
	assert.NotEmpty(t, result.Language, "Language should not be empty")
	assert.NotEmpty(t, result.Summary, "Summary should not be empty")
}
