package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// AgentOutputArchive 表示单次 Agent 调用输出归档后的文件路径。
type AgentOutputArchive struct {
	ContentPath  string
	RawPath      string
	StderrPath   string
	ManifestPath string
}

type agentOutputManifest struct {
	Agent            string  `json:"agent"`
	Operation        string  `json:"operation"`
	RuntimeID        string  `json:"runtime_id,omitempty"`
	Slug             string  `json:"slug,omitempty"`
	Label            string  `json:"label,omitempty"`
	Attempt          int     `json:"attempt"`
	ContentPath      string  `json:"content_path,omitempty"`
	RawPath          string  `json:"raw_path,omitempty"`
	StderrPath       string  `json:"stderr_path,omitempty"`
	ContentLength    int     `json:"content_length,omitempty"`
	RawOutputLength  int     `json:"raw_output_length,omitempty"`
	StderrLength     int     `json:"stderr_length,omitempty"`
	ExitError        bool    `json:"exit_error,omitempty"`
	TokenUsageKnown  bool    `json:"token_usage_known,omitempty"`
	InputTokens      int64   `json:"input_tokens,omitempty"`
	OutputTokens     int64   `json:"output_tokens,omitempty"`
	TotalTokens      int64   `json:"total_tokens,omitempty"`
	CacheReadTokens  int64   `json:"cache_read_input_tokens,omitempty"`
	CacheWriteTokens int64   `json:"cache_creation_input_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
	CreatedAtRFC3339 string  `json:"created_at"`
}

// AgentOutputArchiveOptions 描述需要归档的 Agent 调用输出。
type AgentOutputArchiveOptions struct {
	Agent      string
	Operation  string
	RuntimeID  string
	Slug       string
	Label      string
	Attempt    int
	Content    string
	RawOutput  string
	Stderr     string
	ExitError  bool
	TokenUsage tokenusage.Usage
}

// SaveAgentOutputForContext 把 Agent 输出保存到当前项目 .skills-seed/runtime/agent-outputs。
func SaveAgentOutputForContext(ctx context.Context, opts AgentOutputArchiveOptions) AgentOutputArchive {
	seedPath := runtimecontext.SeedPath(ctx)
	if strings.TrimSpace(seedPath) == "" {
		return AgentOutputArchive{}
	}
	if opts.Label == "" {
		opts.Label = OperationLabel(opts.Operation)
	}

	dir := layout.New(seedPath).Runtime("agent-outputs")
	if config.DefaultAutoDeleteAgentOutputs {
		if err := os.RemoveAll(dir); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "agent.output.cleanup",
				"agent", opts.Agent,
				"agent_operation", opts.Operation,
				"path", dir,
				"error", err,
			)
		}
	}
	if !config.DefaultSaveAgentOutputs {
		return AgentOutputArchive{}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "agent.output.mkdir",
			"agent", opts.Agent,
			"agent_operation", opts.Operation,
			"path", dir,
			"error", err,
		)
		return AgentOutputArchive{}
	}

	slug := strings.TrimSpace(opts.Slug)
	if slug == "" {
		slug = RuntimeSlug(OperationName(opts.Operation), opts.Label)
	}
	base := runtimefiles.NameWithID(opts.RuntimeID, opts.Agent, slug)
	if opts.Attempt > 1 {
		base += fmt.Sprintf("-attempt-%03d", opts.Attempt)
	}
	archive := AgentOutputArchive{}
	if strings.TrimSpace(opts.Content) != "" {
		path := filepath.Join(dir, base+".md")
		if err := os.WriteFile(path, []byte(renderAgentOutputContent(opts.Content)+"\n"), 0600); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "agent.output.write",
				"agent", opts.Agent,
				"agent_operation", opts.Operation,
				"path", path,
				"error", err,
			)
			return archive
		}
		archive.ContentPath = path
	}
	if strings.TrimSpace(opts.RawOutput) != "" && opts.RawOutput != opts.Content {
		path := filepath.Join(dir, base+".raw.txt")
		if err := os.WriteFile(path, []byte(opts.RawOutput+"\n"), 0600); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "agent.output.raw.write",
				"agent", opts.Agent,
				"agent_operation", opts.Operation,
				"path", path,
				"error", err,
			)
			return archive
		}
		archive.RawPath = path
	}
	if strings.TrimSpace(opts.Stderr) != "" {
		path := filepath.Join(dir, base+".stderr.txt")
		if err := os.WriteFile(path, []byte(opts.Stderr+"\n"), 0600); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "agent.output.stderr.write",
				"agent", opts.Agent,
				"agent_operation", opts.Operation,
				"path", path,
				"error", err,
			)
			return archive
		}
		archive.StderrPath = path
	}

	manifestPath := filepath.Join(dir, base+".manifest.json")
	usage := opts.TokenUsage.Normalize()
	manifest := agentOutputManifest{
		Agent:            opts.Agent,
		Operation:        opts.Operation,
		RuntimeID:        opts.RuntimeID,
		Slug:             slug,
		Label:            opts.Label,
		Attempt:          opts.Attempt,
		ContentPath:      archive.ContentPath,
		RawPath:          archive.RawPath,
		StderrPath:       archive.StderrPath,
		ContentLength:    len(opts.Content),
		RawOutputLength:  len(opts.RawOutput),
		StderrLength:     len(opts.Stderr),
		ExitError:        opts.ExitError,
		TokenUsageKnown:  usage.Known(),
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		CacheReadTokens:  usage.CacheReadInputTokens,
		CacheWriteTokens: usage.CacheCreationInputTokens,
		CostUSD:          usage.CostUSD,
		CreatedAtRFC3339: time.Now().Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "agent.output.manifest.marshal",
			"agent", opts.Agent,
			"agent_operation", opts.Operation,
			"path", manifestPath,
			"error", err,
		)
		return archive
	}
	if err := os.WriteFile(manifestPath, append(data, '\n'), 0600); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "agent.output.manifest.write",
			"agent", opts.Agent,
			"agent_operation", opts.Operation,
			"path", manifestPath,
			"error", err,
		)
		return archive
	}
	archive.ManifestPath = manifestPath
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "agent.output.write",
		"agent", opts.Agent,
		"agent_operation", opts.Operation,
		"label", opts.Label,
		"attempt", opts.Attempt,
		"path", archive.ContentPath,
		"raw_path", archive.RawPath,
		"stderr_path", archive.StderrPath,
		"manifest_path", manifestPath,
		"content_length", len(opts.Content),
		"raw_output_length", len(opts.RawOutput),
		"stderr_length", len(opts.Stderr),
	)
	return archive
}

// OperationLabel 从分层 operation 中提取 runtime 标签。
func OperationLabel(operation string) string {
	_, label, ok := strings.Cut(strings.TrimSpace(operation), "/")
	if !ok {
		return ""
	}
	return strings.TrimSpace(label)
}

// OperationName 从分层 operation 中提取稳定操作名。
func OperationName(operation string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(operation), "/")
	return strings.TrimSpace(name)
}

func renderAgentOutputContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed != "" && json.Valid([]byte(trimmed)) {
		var value any
		if err := json.Unmarshal([]byte(trimmed), &value); err == nil {
			if data, err := json.MarshalIndent(value, "", "  "); err == nil {
				return "```json\n" + string(data) + "\n```"
			}
		}
	}
	return strings.TrimRight(content, "\n")
}
