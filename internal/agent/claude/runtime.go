package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	"github.com/silaswei-io/skills-seed/internal/utils/jsonx"
)

// 调用外部命令行程序，并处理可重试的瞬时错误和结构化输出失败。
func (c *ClaudeAgent) callClaude(ctx context.Context, operation, prompt, outputContract string, task ...agent.RuntimeTask) (string, error) {
	output, _, err := c.callClaudeWithArchive(ctx, operation, prompt, outputContract, task...)
	return output, err
}

type claudeCallResult struct {
	output  string
	archive agent.AgentOutputArchive
}

type claudeSessionStartResult struct {
	output    string
	archive   agent.AgentOutputArchive
	sessionID string
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

	result, err := agent.RunRetryingCall(ctx, agent.RetryingCallOptions[claudeCallResult]{
		AgentName: c.Name(),
		Operation: operation,
		Policy:    c.retryCfg,
		Call: func(attempt int) (claudeCallResult, string, time.Duration, bool, error) {
			output, archive, duration, retryable, err := c.doCallClaude(ctx, operation, prompt, outputSchema, attempt, workDir, agent.FirstRuntimeTask(task))
			return claudeCallResult{output: output, archive: archive}, output, duration, retryable, err
		},
	})
	return result.output, result.archive, err
}

func (c *ClaudeAgent) callClaudeNewSession(ctx context.Context, operation, prompt, outputContract string, task agent.RuntimeTask) (string, string, agent.AgentOutputArchive, error) {
	outputSchema, err := aicontract.StructuredOutputSchema(outputContract)
	if err != nil {
		return "", "", agent.AgentOutputArchive{}, err
	}
	workDir, err := agent.WorkDirForContext(ctx)
	if err != nil {
		return "", "", agent.AgentOutputArchive{}, err
	}

	result, err := agent.RunRetryingCall(ctx, agent.RetryingCallOptions[claudeSessionStartResult]{
		AgentName: c.Name(),
		Operation: operation,
		Policy:    c.retryCfg,
		Call: func(attempt int) (claudeSessionStartResult, string, time.Duration, bool, error) {
			sessionID, err := newClaudeSessionID()
			if err != nil {
				return claudeSessionStartResult{}, "", 0, false, err
			}
			output, archive, duration, retryable, err := c.doCallClaudeSession(ctx, operation, prompt, outputSchema, sessionID, false, attempt, workDir, task)
			return claudeSessionStartResult{output: output, archive: archive, sessionID: sessionID}, output, duration, retryable, err
		},
	})
	return result.output, result.sessionID, result.archive, err
}

func (c *ClaudeAgent) callClaudeSession(ctx context.Context, operation, prompt, outputContract, sessionID string, resume bool, task agent.RuntimeTask) (string, agent.AgentOutputArchive, error) {
	outputSchema, err := aicontract.StructuredOutputSchema(outputContract)
	if err != nil {
		return "", agent.AgentOutputArchive{}, err
	}
	workDir, err := agent.WorkDirForContext(ctx)
	if err != nil {
		return "", agent.AgentOutputArchive{}, err
	}

	result, err := agent.RunRetryingCall(ctx, agent.RetryingCallOptions[claudeCallResult]{
		AgentName: c.Name(),
		Operation: operation,
		Policy:    c.retryCfg,
		Call: func(attempt int) (claudeCallResult, string, time.Duration, bool, error) {
			output, archive, duration, retryable, err := c.doCallClaudeSession(ctx, operation, prompt, outputSchema, sessionID, resume, attempt, workDir, task)
			return claudeCallResult{output: output, archive: archive}, output, duration, retryable, err
		},
	})
	return result.output, result.archive, err
}

// isRetryableError 检测是否为可重试错误（速率限制、过载等）
func isRetryableError(stdout, stderr string) bool {
	return agent.IsRetryableOutputError(stdout, stderr, "error_max_structured_output_retries")
}

// 执行单次命令行调用
func (c *ClaudeAgent) doCallClaude(ctx context.Context, operation, prompt, outputSchema string, attempt int, workDir string, task agent.RuntimeTask) (string, agent.AgentOutputArchive, time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := claudePrintArgs(c.allowUserPlugins, outputSchema, task.PromptOnly, c.runtime)
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
		retryable := isRetryableError(stdoutStr, stderrStr)
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:     c.Name(),
			Operation: operation,
			RuntimeID: task.ID,
			Slug:      task.Slug,
			Attempt:   attempt,
			RawOutput: stdoutStr,
			Stderr:    stderrStr,
			ExitError: true,
		})

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
	output, outputErr := parseClaudeOutput(rawOutput)
	if outputErr != nil {
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:     c.Name(),
			Operation: operation,
			RuntimeID: task.ID,
			Slug:      task.Slug,
			Attempt:   attempt,
			RawOutput: rawOutput,
			Stderr:    stderr.String(),
		})
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
		if retryable || outputErr.invocation {
			return rawOutput + stderr.String(), archive, duration, retryable, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, outputErr, rawOutput, stderr.String(), archive))
		}
		return "", archive, duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), agent.NewResultContractError(c.Name(), operation, outputErr, rawOutput, archive))
	}
	archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
		Agent:     c.Name(),
		Operation: operation,
		RuntimeID: task.ID,
		Slug:      task.Slug,
		Attempt:   attempt,
		Content:   output,
		RawOutput: rawOutput,
		Stderr:    stderr.String(),
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
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallComplete"), callCompleteFields...)

	return output, archive, duration, false, nil
}

func (c *ClaudeAgent) doCallClaudeSession(ctx context.Context, operation, prompt, outputSchema, sessionID string, resume bool, attempt int, workDir string, task agent.RuntimeTask) (string, agent.AgentOutputArchive, time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := claudeSessionPrintArgs(c.allowUserPlugins, outputSchema, task.PromptOnly, sessionID, resume, c.runtime)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallStart"),
		"agent", c.Name(),
		"operation", operation,
		"command", c.commandPath,
		"timeout", c.timeout,
		"work_dir", workDir,
		"prompt_length", len(prompt),
		"session_id", sessionID,
		"args", claudeArgsForLog(args),
		"attempt", attempt,
	)

	cmd := exec.CommandContext(ctx, c.commandPath, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)

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
		retryable := isRetryableError(stdoutStr, stderrStr)
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:     c.Name(),
			Operation: operation,
			RuntimeID: task.ID,
			Slug:      task.Slug,
			Attempt:   attempt,
			RawOutput: stdoutStr,
			Stderr:    stderrStr,
			ExitError: true,
		})

		if retryable {
			return stdoutStr + stderrStr, archive, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeRetryable"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
		}
		return "", archive, duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
	}

	rawOutput := stdout.String()
	output, outputErr := parseClaudeOutput(rawOutput)
	if outputErr != nil {
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:     c.Name(),
			Operation: operation,
			RuntimeID: task.ID,
			Slug:      task.Slug,
			Attempt:   attempt,
			RawOutput: rawOutput,
			Stderr:    stderr.String(),
		})
		retryable := isRetryableError(rawOutput, stderr.String())
		if retryable || outputErr.invocation {
			return rawOutput + stderr.String(), archive, duration, retryable, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, outputErr, rawOutput, stderr.String(), archive))
		}
		return "", archive, duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), agent.NewResultContractError(c.Name(), operation, outputErr, rawOutput, archive))
	}
	archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
		Agent:     c.Name(),
		Operation: operation,
		RuntimeID: task.ID,
		Slug:      task.Slug,
		Attempt:   attempt,
		Content:   output,
		RawOutput: rawOutput,
		Stderr:    stderr.String(),
	})
	callCompleteFields := []any{
		"agent", c.Name(),
		"operation", operation,
		"attempt", attempt,
		"output_length", len(output),
		"raw_output_length", stdout.Len(),
		"stderr_length", stderr.Len(),
		"duration", duration,
		"session_id", sessionID,
		"output_path", archive.ContentPath,
		"raw_output_path", archive.RawPath,
		"stderr_path", archive.StderrPath,
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallComplete"), callCompleteFields...)

	return output, archive, duration, false, nil
}

type claudeOutputError struct {
	cause      error
	invocation bool
}

func (e *claudeOutputError) Error() string { return e.cause.Error() }
func (e *claudeOutputError) Unwrap() error { return e.cause }

func parseClaudeOutput(rawOutput string) (string, *claudeOutputError) {
	var result struct {
		Type             string          `json:"type"`
		Subtype          string          `json:"subtype"`
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		Errors           []string        `json:"errors"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := jsonx.Unmarshal([]byte(strings.TrimSpace(rawOutput)), &result); err != nil {
		return "", &claudeOutputError{cause: fmt.Errorf("%s: %w", i18n.Get("AgentClaudeEnvelopeParseFailed"), err)}
	}
	if result.Type != "result" {
		return "", &claudeOutputError{cause: errors.New(i18n.GetWithParams("AgentClaudeUnexpectedEnvelopeType", map[string]interface{}{"Type": result.Type}))}
	}
	if result.IsError || strings.HasPrefix(result.Subtype, "error_") {
		detail := strings.TrimSpace(result.Result)
		if len(result.Errors) > 0 {
			detail = strings.Join(result.Errors, "; ")
		}
		if detail == "" {
			detail = result.Subtype
		}
		return "", &claudeOutputError{
			cause:      fmt.Errorf("claude CLI 返回失败结果: %s", detail),
			invocation: true,
		}
	}
	structuredOutput := bytes.TrimSpace(result.StructuredOutput)
	if len(structuredOutput) > 0 && !bytes.Equal(structuredOutput, []byte("null")) {
		return string(structuredOutput), nil
	}
	resultJSON, ok := claudeStructuredResult(result.Result)
	if ok {
		return resultJSON, nil
	}
	return "", &claudeOutputError{cause: errors.New(i18n.Get("AgentClaudeStructuredOutputMissing"))}
}

func claudeStructuredResult(value string) (string, bool) {
	raw := bytes.TrimSpace([]byte(stripJSONFence(value)))
	if len(raw) == 0 {
		return "", false
	}
	if repaired, ok := repairClaudeJSONCandidate(string(raw)); ok {
		return repaired, true
	}
	// Claude CLI 的 result 字段有时会在目标 JSON 前后追加解释文本。
	// 这里仅抽取语法有效的 JSON 候选，字段契约仍由后续业务 parser 校验。
	for _, candidate := range jsonx.Candidates(string(raw)) {
		return candidate, true
	}
	return "", false
}

func repairClaudeJSONCandidate(value string) (string, bool) {
	return jsonx.RepairCandidate(value)
}

func stripJSONFence(value string) string {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	firstLineEnd := strings.IndexByte(trimmed, '\n')
	if firstLineEnd < 0 {
		return trimmed
	}
	fenceHeader := strings.TrimSpace(trimmed[:firstLineEnd])
	if fenceHeader != "```" && !strings.EqualFold(fenceHeader, "```json") {
		return trimmed
	}
	body := strings.TrimSpace(trimmed[firstLineEnd+1:])
	if !strings.HasSuffix(body, "```") {
		return trimmed
	}
	return strings.TrimSpace(strings.TrimSuffix(body, "```"))
}

func claudePrintArgs(allowUserPlugins bool, outputSchema string, promptOnly bool, runtime config.AgentRuntimeOptions) []string {
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
	args = append(args, claudeRuntimeArgs(runtime)...)
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

func claudeSessionPrintArgs(allowUserPlugins bool, outputSchema string, promptOnly bool, sessionID string, resume bool, runtime config.AgentRuntimeOptions) []string {
	args := []string{
		"--print",
		"--disable-slash-commands",
		"--output-format",
		"json",
		"--json-schema",
		outputSchema,
	}
	if resume {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}
	args = append(args, claudeRuntimeArgs(runtime)...)
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

func claudeRuntimeArgs(runtime config.AgentRuntimeOptions) []string {
	args := make([]string, 0)
	if model := strings.TrimSpace(runtime.Model); model != "" {
		args = append(args, "--model", model)
	}
	return args
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
	if err := jsonx.Unmarshal(content, &cfg); err != nil {
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
	if err := jsonx.Unmarshal(content, &settings); err != nil {
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
