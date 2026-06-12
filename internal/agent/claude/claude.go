package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	"github.com/silaswei-io/skills-seed/internal/prompts"
)

// ClaudeAgent 实现模型代理
type ClaudeAgent struct {
	commandPath      string
	timeout          time.Duration
	promptLoader     promptRenderer
	allowUserPlugins bool
	retryCfg         config.RetryConfig
}

// promptRenderer 是 Agent 依赖的最小提示词渲染能力，便于测试渲染错误链路
type promptRenderer interface {
	Render(name string, data interface{}) (string, error)
}

// New 创建代理
func New(commandPath string, timeout time.Duration, loader *prompts.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) *ClaudeAgent {
	if commandPath == "" {
		commandPath = "claude"
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &ClaudeAgent{
		commandPath:      commandPath,
		timeout:          timeout,
		promptLoader:     loader,
		allowUserPlugins: allowUserPlugins,
		retryCfg:         retryCfg,
	}
}

// Name 返回代理名称
func (c *ClaudeAgent) Name() string {
	return "claude"
}

// IsAvailable 检查代理是否可用
func (c *ClaudeAgent) IsAvailable() bool {
	_, err := exec.LookPath(c.commandPath)
	return err == nil
}

// AnalyzeCode 分析代码
func (c *ClaudeAgent) AnalyzeCode(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-check")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 构建提示词（从模板加载）
	data, err := agent.CheckPromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.Render("learn-analyze", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderAnalyzePromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, err := c.callClaude(ctx, "AnalyzeCode", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeAnalyzeFailed"), err)
	}

	// 3. 解析结构化结果
	result, err := parser.ParseAnalyzeResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeCode",
		"issues_count", len(result.Issues),
		"suggestions_count", len(result.Suggestions),
		"confidence", result.Confidence,
	)

	result.AnalyzedAt = time.Now()
	return result, nil
}

// LearnFromCommit 从提交中学习
func (c *ClaudeAgent) LearnFromCommit(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-learn")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 包装为批量格式，复用批量学习模板
	data, err := agent.BatchLearnPromptData(
		session,
		[]domain.CommitInfo{req.Commit},
		[]agent.CommitFileChange{{Commit: req.Commit, Files: req.ChangedFiles}},
		req.KnownPatternsJSON,
		req.KnownPatternsPath,
		req.KnownPatternsCount,
	)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.Render("learn-batch", data)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentPromptRenderFailed"),
			"method", "LearnFromCommit",
			"template", "learn-batch",
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderBatchLearnPromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, err := c.callClaude(ctx, "LearnFromCommit", prompt)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "LearnFromCommit",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeLearnFailed"), err)
	}

	// 3. 解析结构化结果
	result, err := parser.ParseLearnResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "LearnFromCommit",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "LearnFromCommit",
		"patterns_count", len(result.Patterns),
	)

	result.LearnedAt = time.Now()
	return result, nil
}

// BatchLearnFromCommits 批量从多个提交中学习
func (c *ClaudeAgent) BatchLearnFromCommits(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-learn-batch")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 准备模板数据
	data, err := agent.BatchLearnPromptData(session, req.Commits, req.CommitFiles, req.KnownPatternsJSON, req.KnownPatternsPath, req.KnownPatternsCount)
	if err != nil {
		return nil, err
	}

	// 2. 渲染提示词
	prompt, err := c.promptLoader.Render("learn-batch", data)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentPromptRenderFailed"),
			"method", "BatchLearnFromCommits",
			"template", "learn-batch",
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderBatchLearnPromptFailed"))
	}

	// 3. 调用外部命令行程序
	output, err := c.callClaude(ctx, "BatchLearnFromCommits", prompt)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "BatchLearnFromCommits",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeBatchLearnFailed"), err)
	}

	// 4. 解析结构化结果
	result, err := parser.ParseBatchLearnResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "BatchLearnFromCommits",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
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
func (c *ClaudeAgent) GenerateFixes(ctx context.Context, req *agent.GenerateFixesRequest) (*agent.GenerateFixesResult, error) {
	// 1. 构建提示词（从模板加载）
	prompt, err := c.promptLoader.Render("fix-generate", req)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentPromptRenderFailed"),
			"method", "GenerateFixes",
			"template", "fix-generate",
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderGenerateFixesPromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, err := c.callClaude(ctx, "GenerateFixes", prompt)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "GenerateFixes",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeGenerateFixesFailed"), err)
	}

	// 3. 解析结构化结果
	result, err := parser.ParseGenerateFixesResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "GenerateFixes",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "GenerateFixes",
		"fixes_count", len(result.Fixes),
		"confidence", result.Confidence,
	)

	result.GeneratedAt = time.Now()
	return result, nil
}

// SelectFiles 基于候选文件树选择当前代码学习范围。
func (c *ClaudeAgent) SelectFiles(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-file-select")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	data, err := agent.SelectFilesPromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.Render("file-select", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderAnalyzePromptFailed"))
	}

	output, err := c.callClaude(ctx, "SelectFiles", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeAnalyzeFailed"), err)
	}
	return parser.ParseSelectFilesResult(output)
}

// 调用外部命令行程序（含速率限制自动重试）
func (c *ClaudeAgent) callClaude(ctx context.Context, operation, prompt string) (string, error) {
	workDir, err := agent.WorkDirForContext(ctx)
	if err != nil {
		return "", err
	}

	maxRetries := c.retryCfg.EffectiveMaxRetries()
	retried := false
	for attempt := 0; attempt <= maxRetries; attempt++ {
		attemptNumber := attempt + 1
		if attempt > 0 {
			agent.ReportRetryAttemptForContext(ctx, agent.RetryInfo{
				AgentName:  c.Name(),
				Operation:  operation,
				Attempt:    attemptNumber,
				MaxRetries: maxRetries,
			})
		}
		output, callDuration, isRetryable, err := c.doCallClaude(ctx, operation, prompt, attemptNumber, workDir)
		if err == nil {
			if retried {
				agent.ReportRetryRecoveredForContext(ctx, agent.RetryInfo{
					AgentName:    c.Name(),
					Operation:    operation,
					Attempt:      attemptNumber,
					MaxRetries:   maxRetries,
					CallDuration: callDuration,
				})
			}
			return output, nil
		}
		if !isRetryable || attempt == maxRetries {
			return "", err
		}

		retried = true
		waitDuration := c.retryCfg.WaitDuration(attempt)
		agent.ReportRetryForContext(ctx, agent.RetryInfo{
			AgentName:    c.Name(),
			Operation:    operation,
			Attempt:      attemptNumber,
			MaxRetries:   maxRetries,
			WaitDuration: waitDuration,
			CallDuration: callDuration,
			Reason:       agent.RetryReasonFromOutput(output, ""),
		})

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(waitDuration):
		}
	}

	logger.Error(i18n.Get("LoggerAgentRateLimitExhausted"), "max_retries", maxRetries)
	return "", fmt.Errorf("%s: %d", i18n.Get("AgentClaudeRateLimitExhausted"), maxRetries)
}

// isRetryableError 检测是否为可重试错误（速率限制、过载等）
func isRetryableError(stdout, stderr string) bool {
	combined := stdout + stderr
	return strings.Contains(combined, "429") ||
		strings.Contains(combined, "529") ||
		strings.Contains(combined, "overloaded_error") ||
		strings.Contains(combined, "rate limit") ||
		strings.Contains(combined, "速率限制") ||
		strings.Contains(combined, "请求频率") ||
		strings.Contains(combined, "访问量过大")
}

// 执行单次命令行调用
func (c *ClaudeAgent) doCallClaude(ctx context.Context, operation, prompt string, attempt int, workDir string) (string, time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := claudePrintArgs(c.allowUserPlugins)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallStart"),
		"agent", c.Name(),
		"operation", operation,
		"command", c.commandPath,
		"timeout", c.timeout,
		"work_dir", workDir,
		"prompt_length", len(prompt),
		"args", args,
		"attempt", attempt,
	)

	cmd := exec.CommandContext(ctx, c.commandPath, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)

	// 分别捕获标准输出和标准错误
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		retryable := isRetryableError(stdoutStr, stderrStr)
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:     c.Name(),
			Operation: operation,
			Attempt:   attempt,
			RawOutput: stdoutStr,
			Stderr:    stderrStr,
			ExitError: true,
		})

		if retryable {
			logger.Warn(i18n.Get("LoggerAgentClaudeCallFailed"),
				"agent", c.Name(),
				"operation", operation,
				"attempt", attempt,
				"error", err,
				"duration", duration,
				"stdout_length", len(stdoutStr),
				"stderr_length", len(stderrStr),
				"raw_output_path", archive.RawPath,
				"stderr_path", archive.StderrPath,
				"retryable", true,
			)
			return stdoutStr + stderrStr, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeRateLimited"), err)
		}

		logger.Error(i18n.Get("LoggerAgentClaudeCallFailed"),
			"agent", c.Name(),
			"operation", operation,
			"attempt", attempt,
			"error", err,
			"duration", duration,
			"stdout_length", len(stdoutStr),
			"stderr_length", len(stderrStr),
			"raw_output_path", archive.RawPath,
			"stderr_path", archive.StderrPath,
			"prompt_length", len(prompt),
		)
		return "", duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), err)
	}

	rawOutput := stdout.String()
	output, usage := parseClaudeOutput(rawOutput)
	archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
		Agent:           c.Name(),
		Operation:       operation,
		Attempt:         attempt,
		Content:         output,
		RawOutput:       rawOutput,
		Stderr:          stderr.String(),
		TokenUsageKnown: usage.Known(),
	})
	callCompleteFields := []any{
		"agent", c.Name(),
		"operation", operation,
		"attempt", attempt,
		"output_length", len(output),
		"raw_output_length", stdout.Len(),
		"stderr_length", stderr.Len(),
		"duration", duration,
		"output_path", archive.ContentPath,
		"raw_output_path", archive.RawPath,
		"stderr_path", archive.StderrPath,
	}
	callCompleteFields = append(callCompleteFields, tokenusage.Fields(usage, "")...)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallComplete"), callCompleteFields...)
	agent.LogTokenUsageForContext(ctx, c.Name(), operation, usage)

	return output, duration, false, nil
}

func parseClaudeOutput(rawOutput string) (string, tokenusage.Usage) {
	usage := tokenusage.Extract(rawOutput)
	var result struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawOutput)), &result); err == nil && result.Result != "" {
		return result.Result, usage
	}
	return rawOutput, usage
}

func claudePrintArgs(allowUserPlugins bool) []string {
	// 模型命令行常常在生成最终结构化结果之前尝试检查文件
	// 将会话保持为非持久化且只读状态，这样批量分析就能顺利完成，而无需授予具备写入权限的工具
	args := []string{
		"--print",
		"--no-session-persistence",
		"--disable-slash-commands",
		"--output-format",
		"json",
	}
	if !allowUserPlugins {
		if settings := claudeDisableUserPluginSettings(); settings != "" {
			args = append(args, "--settings", settings)
		}
	}
	return append(args, "--tools", "Read,Glob,Grep,LS")
}

type claudeInstalledPluginsConfig struct {
	Plugins map[string][]claudeInstalledPlugin `json:"plugins"`
}

type claudeInstalledPlugin struct {
	Scope string `json:"scope"`
}

type claudeUserSettings struct {
	EnabledPlugins map[string]interface{} `json:"enabledPlugins"`
}

type claudePluginOverrideSettings struct {
	EnabledPlugins map[string]bool `json:"enabledPlugins"`
}

func claudeDisableUserPluginSettings() string {
	pluginNames := claudeUserPluginNames()
	if len(pluginNames) == 0 {
		return ""
	}

	settings := claudePluginOverrideSettings{
		EnabledPlugins: make(map[string]bool, len(pluginNames)),
	}
	for _, name := range pluginNames {
		settings.EnabledPlugins[name] = false
	}

	data, err := json.Marshal(settings)
	if err != nil {
		logger.Debug("生成 Claude 用户插件禁用配置失败", "error", err)
		return ""
	}
	return string(data)
}

func claudeUserPluginNames() []string {
	names := map[string]struct{}{}
	for _, name := range claudeInstalledUserPluginNames() {
		names[name] = struct{}{}
	}
	for _, name := range claudeEnabledUserPluginNames() {
		names[name] = struct{}{}
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func claudeInstalledUserPluginNames() []string {
	configPath := filepath.Join(claudeHomeDir(), "plugins", "installed_plugins.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var cfg claudeInstalledPluginsConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		logger.Debug("读取 Claude 已安装插件配置失败",
			"config_path", configPath,
			"error", err,
		)
		return nil
	}

	var names []string
	for name, installs := range cfg.Plugins {
		if !isClaudeMarketplacePluginName(name) {
			continue
		}
		for _, install := range installs {
			if install.Scope == "user" {
				names = append(names, name)
				break
			}
		}
	}
	return names
}

func claudeEnabledUserPluginNames() []string {
	settingsPath := filepath.Join(claudeHomeDir(), "settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}

	var settings claudeUserSettings
	if err := json.Unmarshal(content, &settings); err != nil {
		logger.Debug("读取 Claude 用户设置失败",
			"settings_path", settingsPath,
			"error", err,
		)
		return nil
	}

	names := make([]string, 0, len(settings.EnabledPlugins))
	for name := range settings.EnabledPlugins {
		if isClaudeMarketplacePluginName(name) {
			names = append(names, name)
		}
	}
	return names
}

func isClaudeMarketplacePluginName(name string) bool {
	return strings.Contains(name, "@") && !strings.HasSuffix(name, "@builtin")
}

func claudeHomeDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(userHome, ".claude")
}

// GenerateSkillsSummary 汇总生成技能内容
func (c *ClaudeAgent) GenerateSkillsSummary(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-generate")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 准备模板数据
	data, err := agent.GenerateSkillsPromptData(session, req)
	if err != nil {
		return nil, err
	}

	// 2. 渲染提示词
	prompt, err := c.promptLoader.Render("skill-project-summary", data)
	if err != nil || prompt == "" {
		logger.Warn(i18n.Get("LoggerAgentSkillsSummaryEmptyPrompt"))
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderGenerateSkillsPromptFailed"))
	}

	// 3. 调用外部命令行程序
	output, err := c.callClaude(ctx, "GenerateSkillsSummary", prompt)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentSkillsSummaryCallFailed"),
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeGenerateSkillsFailed"), err)
	}

	// 4. 解析结果
	result, err := parser.ParseGenerateSkillsResult(output)
	if err != nil {
		// 解析失败，返回空结果而不是错误
		logger.Warn(i18n.Get("LoggerAgentSkillsSummaryParseFallback"),
			"error", err,
		)
		return &agent.GenerateSkillsResult{
			CategorySummaries:      make(map[string]agent.CategorySummary),
			KeyPatterns:            []agent.PatternSummary{},
			BusinessRules:          []string{},
			BestPractices:          []string{},
			CommonPatterns:         []string{},
			KeyInsights:            []string{},
			ImprovementSuggestions: []string{},
		}, nil
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

// CuratePatterns 策展候选模式并输出规范模式。
func (c *ClaudeAgent) CuratePatterns(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
	data := map[string]interface{}{
		"Operation":           req.Operation,
		"CandidatePatterns":   req.CandidatePatterns,
		"ExistingPatterns":    req.ExistingPatterns,
		"AllExisting":         req.AllExisting,
		"ExistingByCandidate": req.ExistingByCandidate,
		"AllowedCategories":   domain.AllowedPatternCategoriesText(),
	}

	prompt, err := c.promptLoader.Render("pattern-curate", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderCuratePatternsPromptFailed"))
	}

	output, err := c.callClaude(ctx, "CuratePatterns", prompt)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentCuratePatternsCallFailed"),
			"error", err,
			"operation", req.Operation,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCuratePatternsFailed"), err)
	}

	result, err := parser.ParseCuratePatternsResult(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentCuratePatternsParseFallback"),
			"error", err,
			"operation", req.Operation,
		)
		return nil, err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "CuratePatterns",
		"written_count", len(result.Patterns),
		"dropped_count", len(result.Dropped),
		"total_candidates", result.Summary.TotalCandidates,
	)

	return result, nil
}

// UserDefinePattern 根据用户自然语言描述生成模式
func (c *ClaudeAgent) UserDefinePattern(ctx context.Context, req *agent.UserDefinePatternRequest) (*agent.UserDefinePatternResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-user-pattern")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	data, err := agent.UserDefinePatternPromptData(session, req)
	if err != nil {
		return nil, err
	}

	prompt, err := c.promptLoader.Render("user-define-pattern", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderUserDefinePatternPromptFailed"))
	}

	output, err := c.callClaude(ctx, "UserDefinePattern", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentUserDefinePatternFailed"), err)
	}

	result, err := parser.ParseUserDefinePatternResult(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentParseResultFallback"),
			"method", "UserDefinePattern",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "UserDefinePattern",
		"pattern_id", result.Pattern.ID,
		"pattern_name", result.Pattern.Name,
		"category", result.Pattern.Category,
	)

	return result, nil
}

// AnalyzeProject 分析项目结构
func (c *ClaudeAgent) AnalyzeProject(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-project-analyze")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 准备模板数据
	data, err := agent.AnalyzeProjectPromptData(session, req)
	if err != nil {
		return nil, err
	}

	// 2. 渲染提示词
	prompt, err := c.promptLoader.Render("project-analyze", data)
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

	// 3. 调用外部命令行程序
	output, err := c.callClaude(ctx, "AnalyzeProject", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	// 4. 解析结果
	result, err := parser.ParseAnalyzeProjectResult(output)
	if err != nil {
		logger.Error(i18n.Get("AgentParseProjectAnalysisFailed"),
			"error", err,
			"project", req.ProjectName)
		logger.Error(i18n.Get("AgentRawOutputLength"), "output_length", len(output))
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
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
func (c *ClaudeAgent) AnalyzeCurrentCodebase(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-skill-project-init")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 构建提示词（从模板加载）
	data, err := agent.AnalyzeCurrentCodebasePromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.Render("skill-project-init", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderInitSkillsPromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, err := c.callClaude(ctx, "AnalyzeCurrentCodebase", prompt)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "AnalyzeCurrentCodebase",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	// 3. 解析结果
	result, err := parser.ParseAnalyzeCurrentCodebaseResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "AnalyzeCurrentCodebase",
			"error", err,
			"output_length", len(output),
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
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

// AnalyzeWorkspaceProfile 分析工作区结构和跨项目关系
func (c *ClaudeAgent) AnalyzeWorkspaceProfile(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
	prompt, err := c.promptLoader.Render("skill-workspace-profile", workspacePromptData(req.WorkspaceName, req.WorkspaceRoot, req.WorkspaceInputPath, "", req.UserContextPath))
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callClaude(ctx, "AnalyzeWorkspaceProfile", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	result, err := parser.ParseWorkspaceProfile(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeWorkspaceProfile",
		"projects_count", len(result.Projects),
		"impact_routes_count", len(result.ImpactRoutes),
	)
	return result, nil
}

// AnalyzeWorkspaceSpec 生成工作区级开发规范
func (c *ClaudeAgent) AnalyzeWorkspaceSpec(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
	data := workspacePromptData(req.WorkspaceName, req.WorkspaceRoot, req.WorkspaceInputPath, req.WorkspaceProfilePath, req.UserContextPath)
	prompt, err := c.promptLoader.Render("skill-workspace-spec", data)
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callClaude(ctx, "AnalyzeWorkspaceSpec", prompt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	result, err := parser.ParseWorkspaceSpec(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeWorkspaceSpec",
		"routing_count", len(result.Routing),
		"rules_count", len(result.Rules),
	)
	return result, nil
}

func workspacePromptData(workspaceName, workspaceRoot, workspaceInputPath, workspaceProfilePath, userContextPath string) map[string]interface{} {
	return map[string]interface{}{
		"WorkspaceName":        workspaceName,
		"WorkspaceRoot":        workspaceRoot,
		"WorkspaceInputPath":   workspaceInputPath,
		"WorkspaceProfilePath": workspaceProfilePath,
		"UserContextPath":      userContextPath,
	}
}
