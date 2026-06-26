package commandutil

import (
	"strings"

	"github.com/spf13/cobra"
)

// CommandStateScope 返回命令可恢复状态使用的稳定 scope。
func CommandStateScope(commandPath string) string {
	commandPath = strings.TrimSpace(commandPath)
	if commandPath == "" {
		return "command"
	}
	parts := strings.Fields(commandPath)
	if len(parts) == 0 {
		return "command"
	}
	return strings.Join(parts, "-")
}

// CommandStateScopeForCobra 从 Cobra 命令路径推导可恢复状态 scope。
func CommandStateScopeForCobra(cmd *cobra.Command) string {
	if cmd == nil {
		return CommandStateScope("")
	}
	parts := make([]string, 0)
	for current := cmd; current != nil; current = current.Parent() {
		if current.Parent() == nil && len(parts) > 0 {
			break
		}
		name := commandUseName(current)
		if name != "" {
			parts = append(parts, name)
		}
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return CommandStateScope(strings.Join(parts, " "))
}

func commandUseName(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	fields := strings.Fields(cmd.Use)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
