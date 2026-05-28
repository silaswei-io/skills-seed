package commandutil

import (
	"os"
	"strings"
)

// ResolveRuntimeContext combines one-shot user context flags for learn/generate.
// Inline context wins over context-file because it is the most explicit input.
func ResolveRuntimeContext(inline, filePath string) (string, error) {
	inline = strings.TrimSpace(inline)
	if inline != "" {
		return inline, nil
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
