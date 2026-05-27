package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	"github.com/silaswei-io/skills-seed/internal/prompts"
)

// CodexAgent 实现模型代理
type CodexAgent struct {
	commandPath      string
	timeout          time.Duration
	promptLoader     promptRenderer
	allowUserPlugins bool
}

// promptRenderer 是 Agent 依赖的最小提示词渲染能力，便于测试渲染错误链路
type promptRenderer interface {
	Render(name string, data interface{}) (string, error)
}

// New 创建代理
func New(commandPath string, timeout time.Duration, loader *prompts.Loader, allowUserPlugins ...bool) *CodexAgent {
	if commandPath == "" {
		commandPath = "codex"
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	loadUserPlugins := false
	if len(allowUserPlugins) > 0 {
		loadUserPlugins = allowUserPlugins[0]
	}

	return &CodexAgent{
		commandPath:      commandPath,
		timeout:          timeout,
		promptLoader:     loader,
		allowUserPlugins: loadUserPlugins,
	}
}

// Name 返回代理名称
func (c *CodexAgent) Name() string { return "codex" }

// IsAvailable 检查代理是否可用
func (c *CodexAgent) IsAvailable() bool {
	_, err := exec.LookPath(c.commandPath)
	return err == nil
}

// AnalyzeCode 分析代码
func (c *CodexAgent) AnalyzeCode(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
	prompt, err := c.promptLoader.Render("analyze", req)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderAnalyzePromptFailed"))
	}

	output, err := c.callCodex(ctx, "AnalyzeCode", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexAnalyzeFailed"), err)
	}

	result, err := parser.ParseAnalyzeResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeCode",
		"issues_count", len(result.Issues),
		"suggestions_count", len(result.Suggestions),
		"confidence", result.Confidence,
	)
	return result, nil
}

// LearnFromCommit 从提交中学习
func (c *CodexAgent) LearnFromCommit(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
	data := map[string]interface{}{
		"Commits":            []domain.CommitInfo{req.Commit},
		"CommitFiles":        []agent.CommitFileChange{{Commit: req.Commit, Files: req.ChangedFiles}},
		"KnownPatternsJSON":  req.KnownPatternsJSON,
		"KnownPatternsCount": req.KnownPatternsCount,
	}
	prompt, err := c.promptLoader.Render("batch-learn", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderBatchLearnPromptFailed"))
	}

	output, err := c.callCodex(ctx, "LearnFromCommit", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexLearnFailed"), err)
	}

	result, err := parser.ParseLearnResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "LearnFromCommit",
		"patterns_count", len(result.Patterns),
	)
	return result, nil
}

// BatchLearnFromCommits 批量从多个提交中学习
func (c *CodexAgent) BatchLearnFromCommits(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
	data := map[string]interface{}{
		"Commits":            req.Commits,
		"CommitFiles":        req.CommitFiles,
		"KnownPatternsJSON":  req.KnownPatternsJSON,
		"KnownPatternsCount": req.KnownPatternsCount,
	}
	prompt, err := c.promptLoader.Render("batch-learn", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderBatchLearnPromptFailed"))
	}

	output, err := c.callCodex(ctx, "BatchLearnFromCommits", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexBatchLearnFailed"), err)
	}

	result, err := parser.ParseBatchLearnResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "BatchLearnFromCommits",
		"patterns_count", len(result.Patterns),
	)
	result.LearnedAt = time.Now()
	return result, nil
}

// GenerateFixes 为给定的问题生成修复代码
func (c *CodexAgent) GenerateFixes(ctx context.Context, req *agent.GenerateFixesRequest) (*agent.GenerateFixesResult, error) {
	prompt, err := c.promptLoader.Render("generate_fixes", req)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderGenerateFixesPromptFailed"))
	}

	output, err := c.callCodex(ctx, "GenerateFixes", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexGenerateFixesFailed"), err)
	}

	result, err := parser.ParseGenerateFixesResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "GenerateFixes",
		"fixes_count", len(result.Fixes),
		"confidence", result.Confidence,
	)
	return result, nil
}

// GenerateSkillsSummary 汇总生成技能内容
func (c *CodexAgent) GenerateSkillsSummary(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
	data := map[string]interface{}{
		"PROJECT_NAME":         req.ProjectName,
		"LANGUAGE":             req.Language,
		"PATTERNS_JSON":        req.PatternsJSON,
		"PATTERNS_COUNT":       req.PatternsCount,
		"EXISTING_SKILLS_PATH": req.ExistingSkillsPath,
	}
	prompt, err := c.promptLoader.Render("generate_skills_summary", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderGenerateSkillsPromptFailed"))
	}

	output, err := c.callCodex(ctx, "GenerateSkillsSummary", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexGenerateSkillsFailed"), err)
	}

	result, err := parser.ParseGenerateSkillsResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "GenerateSkillsSummary",
		"category_summaries_count", len(result.CategorySummaries),
		"key_patterns_count", len(result.KeyPatterns),
		"business_rules_count", len(result.BusinessRules),
	)
	return result, nil
}

// MergePatterns 汇总合并相似模式
func (c *CodexAgent) MergePatterns(ctx context.Context, req *agent.MergePatternsRequest) (*agent.MergePatternsResult, error) {
	data := map[string]interface{}{
		"Category": req.Category,
		"Patterns": req.Patterns,
	}
	prompt, err := c.promptLoader.Render("merge-patterns", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderMergePatternsPromptFailed"))
	}

	output, err := c.callCodex(ctx, "MergePatterns", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexMergePatternsFailed"), err)
	}

	result, err := parser.ParseMergePatternsResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "MergePatterns",
		"merged_count", len(result.MergedPatterns),
		"unchanged_count", len(result.UnchangedPatterns),
		"total_input", result.Summary.TotalInput,
	)
	return result, nil
}

// AnalyzeProject 分析项目结构
func (c *CodexAgent) AnalyzeProject(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	data := map[string]interface{}{
		"ProjectName":         req.ProjectName,
		"RootPath":            req.RootPath,
		"Language":            req.Language,
		"Structure":           req.Structure,
		"StructuralContext":   req.StructuralContext,
		"ReadmePath":          req.ReadmePath,
		"MainFiles":           req.MainFiles,
		"ExistingProfileJSON": req.ExistingProfileJSON,
		"FocusPaths":          req.FocusPaths,
	}
	prompt, err := c.promptLoader.Render("project-analysis", data)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentProjectPromptRenderFailed"),
			"project", req.ProjectName,
			"error", err,
			"prompt_empty", prompt == "",
		)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.Get("AgentRenderProjectAnalysisPromptFailed"), err)
		}
		return nil, fmt.Errorf("%s: prompt is empty", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callCodex(ctx, "AnalyzeProject", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexProjectAnalysisFailed"), err)
	}

	result, err := parser.ParseAnalyzeProjectResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeProject",
		"frameworks_count", len(result.Frameworks),
		"dependencies_count", len(result.Dependencies),
		"key_modules_count", len(result.KeyModules),
	)
	return result, nil
}

// AnalyzeCurrentCodebase 分析当前代码库，提取初始模式
func (c *CodexAgent) AnalyzeCurrentCodebase(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
	prompt, err := c.promptLoader.Render("init-skills", req)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderInitSkillsPromptFailed"))
	}

	output, err := c.callCodex(ctx, "AnalyzeCurrentCodebase", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexCodebaseAnalysisFailed"), err)
	}

	result, err := parser.ParseAnalyzeCurrentCodebaseResult(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeCurrentCodebase",
		"patterns_count", len(result.Patterns),
		"category_summaries_count", len(result.CategorySummaries),
		"business_rules_count", len(result.BusinessRules),
		"best_practices_count", len(result.BestPractices),
		"common_patterns_count", len(result.CommonPatterns),
	)
	return result, nil
}

func (c *CodexAgent) callCodex(ctx context.Context, operation, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	args := codexExecArgs(c.allowUserPlugins)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallStart"),
		"agent", c.Name(),
		"operation", operation,
		"command", c.commandPath,
		"args", args,
		"timeout", c.timeout,
		"work_dir", workDir,
		"prompt_length", len(prompt),
	)

	cmd := exec.CommandContext(ctx, c.commandPath, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now()
	if err := cmd.Run(); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"agent", c.Name(),
			"operation", operation,
			"duration", time.Since(startedAt),
			"stdout_length", stdout.Len(),
			"stderr_length", stderr.Len(),
			"stderr", stderr.String(),
		)
		return "", fmt.Errorf("%s: %w, stderr: %s", i18n.Get("AgentCodexCLIFailed"), err, stderr.String())
	}

	rawOutput := stdout.String()
	usage := tokenusage.Extract(rawOutput)
	callCompleteFields := []any{
		"agent", c.Name(),
		"operation", operation,
		"duration", time.Since(startedAt),
		"output_length", stdout.Len(),
		"stderr_length", stderr.Len(),
	}
	callCompleteFields = append(callCompleteFields, tokenusage.Fields(usage, "")...)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallComplete"), callCompleteFields...)
	agent.LogTokenUsageForContext(ctx, c.Name(), operation, usage)

	content, err := extractFinalContent(rawOutput)
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"agent", c.Name(),
			"operation", operation,
			"duration", time.Since(startedAt),
			"error", err,
			"output_length", stdout.Len(),
			"output_preview", parser.TruncString(rawOutput, 500),
		)
		return "", fmt.Errorf("%s: %w", i18n.Get("AgentCodexExtractFinalContentWarn"), err)
	}
	logger.Debug("Codex 模型回复预览",
		"agent", c.Name(),
		"operation", operation,
		"output_length", len(content),
		"output_preview", codexReplyPreview(content),
	)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", operation,
		"content_length", len(content),
	)
	return content, nil
}

func codexExecArgs(allowUserPlugins bool) []string {
	// 已经把需要分析的结构和样例代码放进提示词。这里让模型以一次性、
	// 只读、非交互模式在当前目录运行，避免写入文件或等待工具审批
	args := []string{
		"--ask-for-approval", "never",
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"--ignore-rules",
		"--sandbox", "read-only",
		"--color", "never",
		"--json",
		"-",
	}
	if !allowUserPlugins {
		args = append(codexDisableUserPluginArgs(), args...)
	}
	return args
}

func codexDisableUserPluginArgs() []string {
	pluginNames := codexUserPluginNames()
	args := make([]string, 0, len(pluginNames)*2)
	for _, name := range pluginNames {
		args = append(args, "-c", fmt.Sprintf("plugins.%s.enabled=false", strconv.Quote(name)))
	}
	return args
}

func codexUserPluginNames() []string {
	configPath := filepath.Join(codexHomeDir(), "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var cfg struct {
		Plugins map[string]interface{} `toml:"plugins"`
	}
	if err := toml.Unmarshal(content, &cfg); err != nil {
		logger.Debug("读取 Codex 用户插件配置失败",
			"config_path", configPath,
			"error", err,
		)
		return nil
	}

	names := make([]string, 0, len(cfg.Plugins))
	for name := range cfg.Plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func codexHomeDir() string {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(userHome, ".codex")
}

func codexReplyPreview(output string) string {
	return parser.TruncString(output, 1000)
}

func extractFinalContent(output string) (string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var evt map[string]interface{}
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}

		if content := extractCodexEventContent(evt); content != "" {
			return content, nil
		}
	}

	return "", fmt.Errorf("%s", i18n.Get("AgentCodexNoFinalMessage"))
}

func extractCodexEventContent(evt map[string]interface{}) string {
	if isCodexMessageType(stringField(evt, "msg_type")) || isCodexMessageType(stringField(evt, "type")) {
		if content := contentField(evt); content != "" {
			return content
		}
	}

	if item, ok := mapField(evt, "item"); ok && isCodexMessageType(stringField(item, "type")) {
		if content := contentField(item); content != "" {
			return content
		}
	}

	if message, ok := mapField(evt, "message"); ok {
		if content := contentField(message); content != "" {
			return content
		}
	}

	return ""
}

func isCodexMessageType(value string) bool {
	switch value {
	case "agent_message", "assistant_message", "message", "final_message":
		return true
	default:
		return false
	}
}

func contentField(data map[string]interface{}) string {
	for _, field := range []string{"content", "message", "text", "final_message"} {
		if content := stringifyContent(data[field]); content != "" {
			return content
		}
	}
	return ""
}

func stringifyContent(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []interface{}:
		var parts []string
		for _, item := range typed {
			switch content := item.(type) {
			case string:
				if content = strings.TrimSpace(content); content != "" {
					parts = append(parts, content)
				}
			case map[string]interface{}:
				if text := contentField(content); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func stringField(data map[string]interface{}, key string) string {
	value, _ := data[key].(string)
	return value
}

func mapField(data map[string]interface{}, key string) (map[string]interface{}, bool) {
	value, ok := data[key].(map[string]interface{})
	return value, ok
}
