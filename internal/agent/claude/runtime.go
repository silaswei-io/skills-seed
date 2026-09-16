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
	"strconv"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
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

func (c *ClaudeAgent) callClaudeInConversationWithOptions(ctx context.Context, operation, prompt, outputContract string, opts aicontract.StructuredOutputOptions, conversation agent.Conversation, task ...agent.RuntimeTask) (string, agent.Conversation, error) {
	result, err := c.callClaudeResult(ctx, operation, prompt, outputContract, opts, conversation, task...)
	return result.output, result.conversation, err
}

func (c *ClaudeAgent) callClaudeWithOptions(ctx context.Context, operation, prompt, outputContract string, opts aicontract.StructuredOutputOptions, task ...agent.RuntimeTask) (string, error) {
	output, _, err := c.callClaudeWithArchiveWithOptions(ctx, operation, prompt, outputContract, opts, task...)
	return output, err
}

type claudeCallResult struct {
	output       string
	conversation agent.Conversation
	archive      agent.AgentOutputArchive
}

func (c *ClaudeAgent) callClaudeWithArchive(ctx context.Context, operation, prompt, outputContract string, task ...agent.RuntimeTask) (string, agent.AgentOutputArchive, error) {
	return c.callClaudeWithArchiveWithOptions(ctx, operation, prompt, outputContract, aicontract.StructuredOutputOptions{}, task...)
}

func (c *ClaudeAgent) callClaudeWithArchiveWithOptions(ctx context.Context, operation, prompt, outputContract string, opts aicontract.StructuredOutputOptions, task ...agent.RuntimeTask) (string, agent.AgentOutputArchive, error) {
	result, err := c.callClaudeResult(ctx, operation, prompt, outputContract, opts, agent.Conversation{}, task...)
	return result.output, result.archive, err
}

func (c *ClaudeAgent) callClaudeResult(ctx context.Context, operation, prompt, outputContract string, opts aicontract.StructuredOutputOptions, conversation agent.Conversation, task ...agent.RuntimeTask) (claudeCallResult, error) {
	// 同一阶段的排队、执行、退避与结构修复共享预算，重试不重置截止时间。
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	outputSchema, err := aicontract.StructuredOutputSchemaWithOptions(outputContract, opts)
	if err != nil {
		return claudeCallResult{}, err
	}
	outputValidator, err := parser.CompileContractValidator(outputContract, outputSchema, opts)
	if err != nil {
		return claudeCallResult{}, err
	}
	workDir, err := agent.WorkDirForContext(ctx)
	if err != nil {
		return claudeCallResult{}, err
	}

	repair := agent.NewResultRepair(prompt)
	result, err := agent.RunRetryingCall(ctx, agent.RetryingCallOptions[claudeCallResult]{
		AgentName: c.Name(),
		Operation: operation,
		Policy:    c.retryCfg,
		Call: func(attempt int) (claudeCallResult, string, time.Duration, bool, error) {
			output, nextConversation, archive, duration, retryable, err := c.doCallClaude(ctx, operation, repair.Prompt(), outputSchema, outputValidator.Validate, conversation, attempt, workDir, agent.FirstRuntimeTask(task))
			retryable = repair.Prepare(err, retryable)
			retryOutput := output
			if err != nil && strings.TrimSpace(retryOutput) == "" {
				retryOutput = err.Error()
			}
			return claudeCallResult{output: output, conversation: nextConversation, archive: archive}, retryOutput, duration, retryable, err
		},
		RetryDetail: func(result claudeCallResult) string {
			return agentArchiveDiagnosticPath(result.archive)
		},
	})
	return result, err
}

func agentArchiveDiagnosticPath(archive agent.AgentOutputArchive) string {
	if archive.ManifestPath != "" {
		return archive.ManifestPath
	}
	if archive.RawPath != "" {
		return archive.RawPath
	}
	return archive.SchemaPath
}

// isRetryableError 检测是否为可重试错误（速率限制、过载等）
func isRetryableError(stdout, stderr string) bool {
	return agent.IsRetryableOutputError(stdout, stderr, "error_max_structured_output_retries")
}

// 执行单次命令行调用
func (c *ClaudeAgent) doCallClaude(ctx context.Context, operation, prompt, outputSchema string, validateOutput func(string) error, conversation agent.Conversation, attempt int, workDir string, task agent.RuntimeTask) (string, agent.Conversation, agent.AgentOutputArchive, time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := claudePrintArgsForConversation(c.allowUserPlugins, outputSchema, task.PromptOnly, c.runtime, conversation)
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
		retryable := agent.IsRetryableInvocationError(err) || isRetryableError(stdoutStr, stderrStr)
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:     c.Name(),
			Operation: operation,
			RuntimeID: task.ID,
			Slug:      task.Slug,
			Attempt:   attempt,
			RawOutput: stdoutStr,
			Stderr:    stderrStr,
			Schema:    outputSchema,
			Error:     err.Error(),
			ExitError: true,
		})

		if retryable {
			reason := agent.RetryReasonFromOutput(stdoutStr, stderrStr)
			logger.DiagnosticWarn(i18n.Get("LoggerAgentClaudeCallRetryable"),
				"agent", c.Name(),
				"operation", operation,
				"attempt", attempt,
				"error", err,
				"reason", reason,
				"duration", duration,
				"stdout_length", len(stdoutStr),
				"stderr_length", len(stderrStr),
				"raw_output_path", archive.RawPath,
				"stderr_path", archive.StderrPath,
				"retryable", true,
			)
			return stdoutStr + stderrStr, agent.Conversation{}, archive, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeRetryable"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
		}

		logger.DiagnosticError(i18n.Get("LoggerAgentClaudeCallFailed"),
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
		return "", agent.Conversation{}, archive, duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
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
			Schema:    outputSchema,
			Error:     outputErr.Error(),
		})
		retryable := isRetryableError(rawOutput, stderr.String())
		logFields := []any{
			"agent", c.Name(),
			"operation", operation,
			"attempt", attempt,
			"error", outputErr,
			"duration", duration,
			"raw_output_path", archive.RawPath,
			"stderr_path", archive.StderrPath,
			"retryable", retryable,
		}
		if retryable {
			logger.DiagnosticWarn(i18n.Get("LoggerAgentParseResultFailedNonFallback"), logFields...)
		} else {
			logger.DiagnosticError(i18n.Get("LoggerAgentParseResultFailedNonFallback"), logFields...)
		}
		if strings.Contains(rawOutput, `"error_max_structured_output_retries"`) {
			return "", agent.Conversation{}, archive, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), agent.NewResultContractError(c.Name(), operation, attempt, outputErr, rawOutput, archive))
		}
		if retryable || outputErr.invocation {
			return rawOutput + stderr.String(), agent.Conversation{}, archive, duration, retryable, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, outputErr, rawOutput, stderr.String(), archive))
		}
		return "", agent.Conversation{}, archive, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), agent.NewResultContractError(c.Name(), operation, attempt, outputErr, rawOutput, archive))
	}
	if err := validateOutput(output); err != nil {
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:     c.Name(),
			Operation: operation,
			RuntimeID: task.ID,
			Slug:      task.Slug,
			Attempt:   attempt,
			Content:   output,
			RawOutput: rawOutput,
			Stderr:    stderr.String(),
			Schema:    outputSchema,
			Error:     err.Error(),
		})
		logger.DiagnosticWarn(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"agent", c.Name(),
			"operation", operation,
			"attempt", attempt,
			"error", err,
			"duration", duration,
			"output_path", archive.ContentPath,
			"raw_output_path", archive.RawPath,
			"schema_path", archive.SchemaPath,
			"retryable", true,
		)
		return "", agent.Conversation{}, archive, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), agent.NewResultContractError(c.Name(), operation, attempt, err, output, archive))
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
		Schema:    outputSchema,
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

	return output, claudeConversation(rawOutput, c.Name(), conversation), archive, duration, false, nil
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawOutput)), &result); err != nil {
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
			cause:      fmt.Errorf("%s", i18n.GetWithParams("AgentClaudeResultError", map[string]interface{}{"Detail": detail})),
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

func claudeConversation(rawOutput, provider string, requested agent.Conversation) agent.Conversation {
	if requested.Provider != provider {
		return agent.Conversation{}
	}
	if requested.Valid() {
		return requested
	}
	var envelope struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawOutput)), &envelope); err != nil {
		return agent.Conversation{}
	}
	return agent.Conversation{Provider: provider, ID: strings.TrimSpace(envelope.SessionID)}
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
	return claudePrintArgsForConversation(allowUserPlugins, outputSchema, promptOnly, runtime, agent.Conversation{})
}

func claudePrintArgsForConversation(allowUserPlugins bool, outputSchema string, promptOnly bool, runtime config.AgentRuntimeOptions, conversation agent.Conversation) []string {
	// 模型命令行常常在生成最终结构化结果之前尝试检查文件。
	// 默认使用非持久化的只读调用；焦点学习会显式传入会话以便后续审查续接。
	args := []string{"--print"}
	if conversation.Provider != "" {
		if conversation.Valid() {
			args = append(args, "--resume", conversation.ID)
		}
	} else {
		args = append(args, "--no-session-persistence")
	}
	args = append(args,
		"--disable-slash-commands",
		"--strict-mcp-config",
		"--output-format",
		"json",
		"--json-schema",
		outputSchema,
	)
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
	args := make([]string, 0, 4)
	if model := strings.TrimSpace(runtime.Model); model != "" {
		args = append(args, "--model", model)
	}
	if runtime.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(runtime.MaxTurns))
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
		logger.Debug(i18n.Get("AgentClaudeDisableUserPluginsFailed"), "error", err)
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
		logger.Debug(i18n.Get("AgentClaudeReadInstalledPluginsFailed"),
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
		logger.Debug(i18n.Get("AgentClaudeReadUserSettingsFailed"),
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
