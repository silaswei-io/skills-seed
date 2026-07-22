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
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
)

// 调用外部命令行程序，并处理可重试的瞬时错误和结构化输出失败。
func (c *ClaudeAgent) callClaude(ctx context.Context, operation, prompt, outputContract string, task ...agent.RuntimeTask) (string, error) {
	output, _, err := c.callClaudeWithArchive(ctx, operation, prompt, outputContract, task...)
	return output, err
}

func parseClaudeResult[T any](agentName, operation, output string, archive agent.AgentOutputArchive, parse func(string) (T, error)) (T, error) {
	result, err := parse(output)
	if err != nil {
		var zero T
		return zero, agent.NewResultContractError(agentName, operation, err, output, archive)
	}
	return result, nil
}

func (c *ClaudeAgent) callClaudeWithArchive(ctx context.Context, operation, prompt, outputContract string, task ...agent.RuntimeTask) (string, agent.AgentOutputArchive, error) {
	outputSchema, err := aicontract.StructuredOutputSchema(outputContract)
	if err != nil {
		return "", agent.AgentOutputArchive{}, err
	}
	workDir, err := agent.WorkDirForContext(ctx)
	if err != nil {
		return "", agent.AgentOutputArchive{}, err
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
		output, archive, callDuration, isRetryable, err := c.doCallClaude(ctx, operation, prompt, outputSchema, attemptNumber, workDir, agent.FirstRuntimeTask(task))
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
			return output, archive, nil
		}
		if !isRetryable || attempt == maxRetries {
			return "", archive, err
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
			return "", agent.AgentOutputArchive{}, ctx.Err()
		case <-time.After(waitDuration):
		}
	}

	logger.Error(i18n.Get("LoggerAgentRetryExhausted"), "max_retries", maxRetries)
	return "", agent.AgentOutputArchive{}, fmt.Errorf("%s: %d", i18n.Get("AgentRetryExhausted"), maxRetries)
}

// isRetryableError 检测是否为可重试错误（速率限制、过载等）
func isRetryableError(stdout, stderr string) bool {
	combined := stdout + stderr
	// 使用 HTTP status code 正则匹配，避免正常输出中包含 "429" 等数字被误判
	if agent.HTTPStatusRetryableRegex.MatchString(combined) {
		return true
	}
	// 非数字类的已知限流/过载信号
	return strings.Contains(combined, "overloaded_error") ||
		strings.Contains(combined, "error_max_structured_output_retries") ||
		strings.Contains(combined, "rate limit") ||
		strings.Contains(combined, "速率限制") ||
		strings.Contains(combined, "请求频率") ||
		strings.Contains(combined, "访问量过大")
}

// 执行单次命令行调用
func (c *ClaudeAgent) doCallClaude(ctx context.Context, operation, prompt, outputSchema string, attempt int, workDir string, task agent.RuntimeTask) (string, agent.AgentOutputArchive, time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := claudePrintArgs(c.allowUserPlugins, outputSchema, task.PromptOnly)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallStart"),
		"agent", c.Name(),
		"operation", operation,
		"command", c.commandPath,
		"timeout", c.timeout,
		"work_dir", workDir,
		"prompt_length", len(prompt),
		"args", claudeArgsForLog(args),
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
		err = agent.NormalizeInvocationError(err, ctx.Err(), c.timeout)
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		usage := tokenusage.Extract(stdoutStr)
		retryable := isRetryableError(stdoutStr, stderrStr)
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:      c.Name(),
			Operation:  operation,
			RuntimeID:  task.ID,
			Slug:       task.Slug,
			Attempt:    attempt,
			RawOutput:  stdoutStr,
			Stderr:     stderrStr,
			ExitError:  true,
			TokenUsage: usage,
		})
		agent.LogTokenUsageForContext(ctx, c.Name(), operation, usage)

		if retryable {
			logger.Diagnostic(i18n.Get("LoggerAgentClaudeCallRetryable"),
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
			return stdoutStr + stderrStr, archive, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeRetryable"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
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
		return "", archive, duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
	}

	rawOutput := stdout.String()
	output, usage, outputErr := parseClaudeOutput(rawOutput)
	archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
		Agent:      c.Name(),
		Operation:  operation,
		RuntimeID:  task.ID,
		Slug:       task.Slug,
		Attempt:    attempt,
		Content:    output,
		RawOutput:  rawOutput,
		Stderr:     stderr.String(),
		TokenUsage: usage,
	})
	if outputErr != nil {
		retryable := isRetryableError(rawOutput, stderr.String())
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"agent", c.Name(),
			"operation", operation,
			"attempt", attempt,
			"error", outputErr,
			"duration", duration,
			"raw_output_path", archive.RawPath,
			"stderr_path", archive.StderrPath,
			"retryable", retryable,
		)
		agent.LogTokenUsageForContext(ctx, c.Name(), operation, usage)
		if retryable || outputErr.invocation {
			return rawOutput + stderr.String(), archive, duration, retryable, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, outputErr, rawOutput, stderr.String(), archive))
		}
		return "", archive, duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), agent.NewResultContractError(c.Name(), operation, outputErr, rawOutput, archive))
	}
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

	return output, archive, duration, false, nil
}

func promptRuntimeTask(task agent.RuntimeTask) promptloader.RuntimeTask {
	return promptloader.RuntimeTask{ID: task.ID, Slug: task.Slug}
}

type claudeOutputError struct {
	cause      error
	invocation bool
}

func (e *claudeOutputError) Error() string { return e.cause.Error() }
func (e *claudeOutputError) Unwrap() error { return e.cause }

func parseClaudeOutput(rawOutput string) (string, tokenusage.Usage, *claudeOutputError) {
	usage := tokenusage.Extract(rawOutput)
	var result struct {
		Type             string          `json:"type"`
		Subtype          string          `json:"subtype"`
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		Errors           []string        `json:"errors"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawOutput)), &result); err != nil {
		return "", usage, &claudeOutputError{cause: fmt.Errorf("解析 Claude JSON envelope: %w", err)}
	}
	if result.Type != "result" {
		return "", usage, &claudeOutputError{cause: fmt.Errorf("claude JSON envelope type 为 %q，期望 result", result.Type)}
	}
	if result.IsError || strings.HasPrefix(result.Subtype, "error_") {
		detail := strings.TrimSpace(result.Result)
		if len(result.Errors) > 0 {
			detail = strings.Join(result.Errors, "; ")
		}
		if detail == "" {
			detail = result.Subtype
		}
		return "", usage, &claudeOutputError{
			cause:      fmt.Errorf("claude CLI 返回失败结果: %s", detail),
			invocation: true,
		}
	}
	structuredOutput := bytes.TrimSpace(result.StructuredOutput)
	if len(structuredOutput) > 0 && !bytes.Equal(structuredOutput, []byte("null")) {
		return string(structuredOutput), usage, nil
	}
	return "", usage, &claudeOutputError{cause: fmt.Errorf("claude CLI 成功响应缺少 structured_output")}
}

func claudePrintArgs(allowUserPlugins bool, outputSchema string, promptOnly bool) []string {
	// 模型命令行常常在生成最终结构化结果之前尝试检查文件
	// 将会话保持为非持久化且只读状态，这样批量分析就能顺利完成，而无需授予具备写入权限的工具
	args := []string{
		"--print",
		"--no-session-persistence",
		"--disable-slash-commands",
		"--output-format",
		"json",
		"--json-schema",
		outputSchema,
	}
	if !allowUserPlugins {
		if settings := claudeDisableUserPluginSettings(); settings != "" {
			args = append(args, "--settings", settings)
		}
	}
	tools := "Read,Glob,Grep,LS"
	if promptOnly {
		tools = ""
	}
	return append(args, "--tools", tools)
}

func claudeArgsForLog(args []string) []string {
	logged := append([]string(nil), args...)
	for i := 0; i+1 < len(logged); i++ {
		if logged[i] == "--json-schema" {
			logged[i+1] = fmt.Sprintf("<schema:%d bytes>", len(logged[i+1]))
			break
		}
	}
	return logged
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
