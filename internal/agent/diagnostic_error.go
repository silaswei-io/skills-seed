package agent

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
)

const diagnosticPreviewLimit = 240

type DiagnosticKind string

const (
	DiagnosticInvocationFailed DiagnosticKind = "invocation_failed"
	DiagnosticResultInvalid    DiagnosticKind = "result_invalid"
)

type DiagnosticError struct {
	Kind          DiagnosticKind
	Agent         string
	Operation     string
	Attempt       int
	Cause         error
	OutputLength  int
	StderrLength  int
	OutputPreview string
	StderrPreview string
	Archive       AgentOutputArchive
}

func (e *DiagnosticError) Error() string {
	return i18n.GetWithParams("AgentDiagnosticError", map[string]interface{}{
		"Kind":          string(e.Kind),
		"Agent":         e.Agent,
		"Operation":     e.Operation,
		"Attempt":       e.Attempt,
		"Cause":         errorText(e.Cause),
		"OutputLength":  e.OutputLength,
		"StderrLength":  e.StderrLength,
		"OutputPreview": e.OutputPreview,
		"StderrPreview": e.StderrPreview,
		"ContentPath":   e.Archive.ContentPath,
		"RawPath":       e.Archive.RawPath,
		"StderrPath":    e.Archive.StderrPath,
		"ManifestPath":  e.Archive.ManifestPath,
	})
}

func (e *DiagnosticError) Unwrap() error {
	return e.Cause
}

func NewInvocationDiagnosticError(agentName, operation string, attempt int, cause error, stdout, stderr string, archive AgentOutputArchive) error {
	return &DiagnosticError{
		Kind:          DiagnosticInvocationFailed,
		Agent:         agentName,
		Operation:     operation,
		Attempt:       attempt,
		Cause:         cause,
		OutputLength:  len(stdout),
		StderrLength:  len(stderr),
		OutputPreview: DiagnosticPreview(stdout),
		StderrPreview: DiagnosticPreview(stderr),
		Archive:       archive,
	}
}

func NewResultContractError(agentName, operation string, cause error, output string, archive AgentOutputArchive) error {
	return &DiagnosticError{
		Kind:          DiagnosticResultInvalid,
		Agent:         agentName,
		Operation:     operation,
		Cause:         cause,
		OutputLength:  len(output),
		OutputPreview: DiagnosticPreview(output),
		Archive:       archive,
	}
}

func DiagnosticPreview(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len([]rune(value)) <= diagnosticPreviewLimit {
		return value
	}
	runes := []rune(value)
	return string(runes[:diagnosticPreviewLimit])
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
