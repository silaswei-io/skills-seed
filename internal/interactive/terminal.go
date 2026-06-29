package interactive

import "os"

// IsTerminal 判断当前标准输入和标准输出是否都连接到交互终端。
func IsTerminal() bool {
	stdinInfo, stdinErr := os.Stdin.Stat()
	stdoutInfo, stdoutErr := os.Stdout.Stat()
	return stdinErr == nil && stdoutErr == nil &&
		stdinInfo.Mode()&os.ModeCharDevice != 0 &&
		stdoutInfo.Mode()&os.ModeCharDevice != 0
}
