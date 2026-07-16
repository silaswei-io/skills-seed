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
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
)

func (c *CodexAgent) callCodex(ctx context.Context, operation, prompt, outputContract string, task ...agent.RuntimeTask) (string, error) {
	outputSchema, err := aicontract.StructuredOutputSchema(outputContract)
	if err != nil {
		return "", err
	}
	schemaFile, err := os.CreateTemp("", "skills-seed-output-schema-*.json")
	if err != nil {
		return "", err
	}
	schemaPath := schemaFile.Name()
	defer os.Remove(schemaPath)
	if _, err := schemaFile.WriteString(outputSchema); err != nil {
		_ = schemaFile.Close()
		return "", err
	}
	if err := schemaFile.Close(); err != nil {
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
		output, callDuration, isRetryable, err := c.doCallCodex(ctx, operation, prompt, schemaPath, attemptNumber, agent.FirstRuntimeTask(task))
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
func isCodexRetryableError(stdout, stderr string) bool {
	combined := stdout + stderr
	if agent.HTTPStatusRetryableRegex.MatchString(combined) {
		return true
	}
	return strings.Contains(combined, "overloaded_error") ||
		strings.Contains(combined, "rate limit") ||
		strings.Contains(combined, "速率限制") ||
		strings.Contains(combined, "请求频率") ||
		strings.Contains(combined, "访问量过大")
}

// doCallCodex 执行单次 Codex CLI 调用
func (c *CodexAgent) doCallCodex(ctx context.Context, operation, prompt, outputSchemaPath string, attempt int, task agent.RuntimeTask) (string, time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	workDir, err := agent.WorkDirForContext(ctx)
	if err != nil {
		return "", 0, false, err
	}
	args := codexExecArgs(c.allowUserPlugins, outputSchemaPath)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallStart"),
		"agent", c.Name(),
		"operation", operation,
		"command", c.commandPath,
		"args", args,
		"timeout", c.timeout,
		"work_dir", workDir,
		"prompt_length", len(prompt),
		"attempt", attempt,
	)

	cmd := exec.CommandContext(ctx, c.commandPath, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	configureCommandProcessGroup(cmd)
	stopProcessGroupKill := context.AfterFunc(ctx, func() {
		terminateCommandProcessGroup(cmd)
	})
	defer stopProcessGroupKill()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Since(startedAt)
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		retryable := isCodexRetryableError(stdoutStr, stderrStr)
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
			logger.Diagnostic(i18n.Get("LoggerAgentCodexCallRetryable"),
				"agent", c.Name(),
				"operation", operation,
				"attempt", attempt,
				"duration", duration,
				"error", err,
				"stdout_length", len(stdoutStr),
				"stderr_length", len(stderrStr),
				"raw_output_path", archive.RawPath,
				"stderr_path", archive.StderrPath,
				"retryable", true,
			)
			return stdoutStr + stderrStr, duration, true, fmt.Errorf("%s: %w", i18n.Get("AgentCodexRateLimited"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
		}

		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"agent", c.Name(),
			"operation", operation,
			"duration", duration,
			"stdout_length", stdout.Len(),
			"stderr_length", stderr.Len(),
			"raw_output_path", archive.RawPath,
			"stderr_path", archive.StderrPath,
		)
		return "", duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentCodexCLIFailed"), agent.NewInvocationDiagnosticError(c.Name(), operation, attempt, err, stdoutStr, stderrStr, archive))
	}
	duration := time.Since(startedAt)

	rawOutput := stdout.String()
	usage := tokenusage.Extract(rawOutput)
	callCompleteFields := []any{
		"agent", c.Name(),
		"operation", operation,
		"duration", duration,
		"output_length", stdout.Len(),
		"stderr_length", stderr.Len(),
		"attempt", attempt,
	}
	callCompleteFields = append(callCompleteFields, tokenusage.Fields(usage, "")...)
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentCallComplete"), callCompleteFields...)
	agent.LogTokenUsageForContext(ctx, c.Name(), operation, usage)

	content, err := extractFinalContent(rawOutput)
	if err != nil {
		archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
			Agent:           c.Name(),
			Operation:       operation,
			RuntimeID:       task.ID,
			Slug:            task.Slug,
			Attempt:         attempt,
			RawOutput:       rawOutput,
			Stderr:          stderr.String(),
			TokenUsageKnown: usage.Known(),
		})
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"agent", c.Name(),
			"operation", operation,
			"duration", duration,
			"error", err,
			"output_length", stdout.Len(),
			"raw_output_path", archive.RawPath,
			"stderr_path", archive.StderrPath,
		)
		return "", duration, false, fmt.Errorf("%s: %w", i18n.Get("AgentCodexExtractFinalContentWarn"), agent.NewResultContractError(c.Name(), operation, err, rawOutput, archive))
	}
	archive := agent.SaveAgentOutputForContext(ctx, agent.AgentOutputArchiveOptions{
		Agent:           c.Name(),
		Operation:       operation,
		RuntimeID:       task.ID,
		Slug:            task.Slug,
		Attempt:         attempt,
		Content:         content,
		RawOutput:       rawOutput,
		Stderr:          stderr.String(),
		TokenUsageKnown: usage.Known(),
	})
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", operation,
		"content_length", len(content),
		"output_path", archive.ContentPath,
		"raw_output_path", archive.RawPath,
		"stderr_path", archive.StderrPath,
	)
	return content, duration, false, nil
}

func codexExecArgs(allowUserPlugins bool, outputSchemaPath string) []string {
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
		"--output-schema", outputSchemaPath,
		"-",
	}
	if !allowUserPlugins {
		args = append(codexDisableUserPluginArgs(), args...)
	}
	return args
}

func promptRuntimeTask(task agent.RuntimeTask) promptloader.RuntimeTask {
	return promptloader.RuntimeTask{ID: task.ID, Slug: task.Slug}
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

func extractFinalContent(output string) (string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var lastMessageContent string
	var allParts []string

	// Forward pass: collect all message event contents.
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var evt map[string]interface{}
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}

		if content := extractCodexEventContent(evt); content != "" {
			lastMessageContent = content
			allParts = append(allParts, content)
		}
	}

	if lastMessageContent != "" {
		if looksLikeJSONContent(lastMessageContent) && !hasEarlierJSONContent(allParts) {
			return lastMessageContent, nil
		}
		// If there are multiple distinct message events, merge them.
		// Deduplicate: if the last message contains all previous content,
		// just return the last one.
		if len(allParts) <= 1 {
			return lastMessageContent, nil
		}
		merged := strings.Join(allParts, "\n")
		previous := strings.TrimSpace(strings.Join(allParts[:len(allParts)-1], "\n"))
		if previous != "" && strings.Contains(lastMessageContent, previous) {
			return lastMessageContent, nil
		}
		return merged, nil
	}

	return "", fmt.Errorf("%s", i18n.Get("AgentCodexNoFinalMessage"))
}

func looksLikeJSONContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "{")
}

func hasEarlierJSONContent(parts []string) bool {
	if len(parts) <= 1 {
		return false
	}
	for _, part := range parts[:len(parts)-1] {
		if looksLikeJSONContent(part) {
			return true
		}
	}
	return false
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
