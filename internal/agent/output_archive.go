package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// AgentOutputArchive 表示单次 Agent 调用输出归档后的文件路径。
type AgentOutputArchive struct {
	ContentPath string
	RawPath     string
	StderrPath  string
}

type agentOutputManifest struct {
	Agent            string `json:"agent"`
	Operation        string `json:"operation"`
	Attempt          int    `json:"attempt"`
	ContentPath      string `json:"content_path,omitempty"`
	RawPath          string `json:"raw_path,omitempty"`
	StderrPath       string `json:"stderr_path,omitempty"`
	ContentLength    int    `json:"content_length,omitempty"`
	RawOutputLength  int    `json:"raw_output_length,omitempty"`
	StderrLength     int    `json:"stderr_length,omitempty"`
	ExitError        bool   `json:"exit_error,omitempty"`
	TokenUsageKnown  bool   `json:"token_usage_known,omitempty"`
	CreatedAtRFC3339 string `json:"created_at"`
}

// AgentOutputArchiveOptions 描述需要归档的 Agent 调用输出。
type AgentOutputArchiveOptions struct {
	Agent           string
	Operation       string
	Attempt         int
	Content         string
	RawOutput       string
	Stderr          string
	ExitError       bool
	TokenUsageKnown bool
}

// SaveAgentOutputForContext 把 Agent 输出保存到当前项目 .skills-seed/memory/runtime/agent-outputs。
func SaveAgentOutputForContext(ctx context.Context, opts AgentOutputArchiveOptions) AgentOutputArchive {
	seedPath := runtimecontext.SeedPath(ctx)
	if strings.TrimSpace(seedPath) == "" {
		return AgentOutputArchive{}
	}

	dir := filepath.Join(seedPath, "memory", "runtime", "agent-outputs")
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

	base := runtimefiles.Name("agent-output", opts.Agent, opts.Operation)
	archive := AgentOutputArchive{}
	if strings.TrimSpace(opts.Content) != "" {
		path := filepath.Join(dir, base+".md")
		if err := os.WriteFile(path, []byte(opts.Content+"\n"), 0600); err != nil {
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
	manifest := agentOutputManifest{
		Agent:            opts.Agent,
		Operation:        opts.Operation,
		Attempt:          opts.Attempt,
		ContentPath:      archive.ContentPath,
		RawPath:          archive.RawPath,
		StderrPath:       archive.StderrPath,
		ContentLength:    len(opts.Content),
		RawOutputLength:  len(opts.RawOutput),
		StderrLength:     len(opts.Stderr),
		ExitError:        opts.ExitError,
		TokenUsageKnown:  opts.TokenUsageKnown,
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
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "agent.output.write",
		"agent", opts.Agent,
		"agent_operation", opts.Operation,
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
