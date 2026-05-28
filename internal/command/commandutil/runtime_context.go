package commandutil

import (
	"os"
	"strings"
)

// ResolveRuntimeContext 合并 learn/generate 的一次性用户上下文参数。
// 行内上下文优先于 context-file，因为它是最明确的输入。
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
